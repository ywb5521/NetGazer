package receiver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	netgazerv1 "github.com/netgazer/backend/gen/netgazer/v1"
	"github.com/netgazer/backend/internal/aggregator"
	"github.com/netgazer/backend/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

type GRPCServer struct {
	netgazerv1.UnimplementedAgentServiceServer
	agg                *aggregator.Aggregator
	mu                 sync.RWMutex
	clients            map[string]netgazerv1.AgentService_StreamSnapshotsServer
	nodeAuthEnabled    bool
	tokenValidator     func(token string) bool
	authenticatedNodes map[string]bool
}

func NewGRPCServer(agg *aggregator.Aggregator, nodeAuthEnabled bool, tokenValidator func(string) bool) *GRPCServer {
	return &GRPCServer{
		agg:                agg,
		clients:            make(map[string]netgazerv1.AgentService_StreamSnapshotsServer),
		nodeAuthEnabled:    nodeAuthEnabled,
		tokenValidator:     tokenValidator,
		authenticatedNodes: make(map[string]bool),
	}
}

func (s *GRPCServer) Register(ctx context.Context, req *netgazerv1.RegisterRequest) (*netgazerv1.RegisterResponse, error) {
	if s.nodeAuthEnabled {
		if req.AuthToken == "" || !s.tokenValidator(req.AuthToken) {
			log.Printf("[server] agent registration rejected: node_id=%s (invalid auth token)", req.NodeId)
			return &netgazerv1.RegisterResponse{
				Accepted: false,
				Message:  "invalid auth token",
			}, nil
		}
		s.mu.Lock()
		s.authenticatedNodes[req.NodeId] = true
		s.mu.Unlock()
	}

	ifaces := req.Interfaces
	if len(ifaces) == 0 && req.Interface != "" {
		ifaces = []string{req.Interface}
	}
	log.Printf("[server] agent registered: node_id=%s interfaces=%v tags=%v", req.NodeId, ifaces, req.Tags)
	s.agg.RegisterNode(req.NodeId, ifaces, req.Version, req.Tags)
	return &netgazerv1.RegisterResponse{
		Accepted:           true,
		Message:            "welcome",
		SnapshotIntervalMs: 0,
	}, nil
}

func (s *GRPCServer) StreamSnapshots(stream netgazerv1.AgentService_StreamSnapshotsServer) error {
	var nodeID string

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Printf("[server] agent %s stream ended", nodeID)
			if nodeID != "" {
				s.agg.SetNodeOffline(nodeID)
				s.mu.Lock()
				delete(s.clients, nodeID)
				s.mu.Unlock()
			}
			return nil
		}
		if err != nil {
			log.Printf("[server] stream recv error from %s: %v", nodeID, err)
			if nodeID != "" {
				s.agg.SetNodeOffline(nodeID)
				s.mu.Lock()
				delete(s.clients, nodeID)
				s.mu.Unlock()
			}
			return err
		}

		if nodeID == "" {
			nodeID = msg.NodeId
			if s.nodeAuthEnabled {
				s.mu.RLock()
				authed := s.authenticatedNodes[nodeID]
				s.mu.RUnlock()
				if !authed {
					log.Printf("[server] stream rejected: node_id=%s not authenticated", nodeID)
					return fmt.Errorf("node %s not authenticated", nodeID)
				}
			}
			s.mu.Lock()
			s.clients[nodeID] = stream
			s.mu.Unlock()
			log.Printf("[server] stream established: %s", nodeID)
		}

		s.agg.Ingest(msg)

		if err := stream.Send(&netgazerv1.ServerMessage{
			Message: &netgazerv1.ServerMessage_Ack{
				Ack: &netgazerv1.Ack{ReceivedTimestampUnixMs: time.Now().UnixMilli()},
			},
		}); err != nil {
			log.Printf("[server] stream ack error to %s: %v", nodeID, err)
			if nodeID != "" {
				s.agg.SetNodeOffline(nodeID)
				s.mu.Lock()
				delete(s.clients, nodeID)
				s.mu.Unlock()
			}
			return err
		}

		// Optional: send ack
		// stream.Send(&netgazerv1.ServerMessage{...})
	}
}

func (s *GRPCServer) BroadcastConfigUpdate(bpfFilter string, intervalMs int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for nodeID, stream := range s.clients {
		msg := &netgazerv1.ServerMessage{
			Message: &netgazerv1.ServerMessage_ConfigUpdate{
				ConfigUpdate: &netgazerv1.ConfigUpdate{
					BpfFilter:             bpfFilter,
					NewSnapshotIntervalMs: intervalMs,
				},
			},
		}
		if err := stream.Send(msg); err != nil {
			log.Printf("[server] failed to send config update to %s: %v", nodeID, err)
		}
	}
	if bpfFilter != "" {
		log.Printf("[server] broadcast BPF filter to %d agents: %s", len(s.clients), bpfFilter)
	}
}

// SendInterceptRules sends intercept rules to specified nodes (or all if targetNodes is empty).
func (s *GRPCServer) SendInterceptRules(targetNodes []string, rules []*netgazerv1.InterceptRule) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targets := make(map[string]bool)
	allNodes := len(targetNodes) == 0
	for _, n := range targetNodes {
		targets[n] = true
	}

	var sentCount int
	for nodeID, stream := range s.clients {
		if !allNodes && !targets[nodeID] {
			continue
		}
		msg := &netgazerv1.ServerMessage{
			Message: &netgazerv1.ServerMessage_InterceptUpdate{
				InterceptUpdate: &netgazerv1.InterceptConfigUpdate{
					TargetNodes: targetNodes,
					Rules:       rules,
				},
			},
		}
		if err := stream.Send(msg); err != nil {
			log.Printf("[server] failed to send intercept rules to %s: %v", nodeID, err)
		} else {
			sentCount++
		}
	}
	log.Printf("[server] sent intercept rules to %d agents (targets: %v)", sentCount, targetNodes)
}

// OnlineNodes returns the list of currently connected node IDs.
func (s *GRPCServer) OnlineNodes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes := make([]string, 0, len(s.clients))
	for n := range s.clients {
		nodes = append(nodes, n)
	}
	return nodes
}

func StartGRPCServer(cfg *config.ServerConfig, agg *aggregator.Aggregator, nodeAuthEnabled bool, tokenValidator func(string) bool) (*GRPCServer, *grpc.Server, error) {
	var opts []grpc.ServerOption

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, nil, err
		}
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}

		if cfg.TLSCA != "" {
			caPEM, err := os.ReadFile(cfg.TLSCA)
			if err != nil {
				return nil, nil, err
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, nil, err
			}
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
			tlsCfg.ClientCAs = pool
			log.Printf("[server] gRPC mTLS enabled (client cert verification active)")
		} else {
			log.Printf("[server] gRPC TLS enabled (server-only)")
		}

		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	srv := grpc.NewServer(opts...)
	svc := NewGRPCServer(agg, nodeAuthEnabled, tokenValidator)
	netgazerv1.RegisterAgentServiceServer(srv, svc)
	reflection.Register(srv)
	return svc, srv, nil
}
