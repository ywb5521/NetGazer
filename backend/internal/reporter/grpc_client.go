package reporter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	netgazerv1 "github.com/netgazer/backend/gen/netgazer/v1"
	"github.com/netgazer/backend/internal/agenthealth"
	"github.com/netgazer/backend/internal/tracker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type IfacePipe struct {
	Name              string
	HostTracker       *tracker.HostTracker
	FlowTracker       *tracker.FlowTracker
	ProtoTracker      *tracker.ProtocolTracker
	DNSTracker        *tracker.DNSTracker
	PacketSizeTracker *tracker.PacketSizeTracker
	TCPMetrics        *tracker.TCPMetricsTracker
}

type GRPCClient struct {
	serverAddr       string
	conn             *grpc.ClientConn
	client           netgazerv1.AgentServiceClient
	stream           netgazerv1.AgentService_StreamSnapshotsClient
	nodeID           string
	ifacePipes       []*IfacePipe
	version          string
	tags             []string
	interval         time.Duration
	systemHealth     *agenthealth.Health
	tlsCert          string
	tlsKey           string
	tlsCA            string
	authToken        string
	onInterceptRules func(rules []*netgazerv1.InterceptRule)
}

func (c *GRPCClient) SetHealth(h *agenthealth.Health) {
	c.systemHealth = h
}

func (c *GRPCClient) SetInterceptCallback(fn func(rules []*netgazerv1.InterceptRule)) {
	c.onInterceptRules = fn
}

func NewGRPCClient(serverAddr, nodeID string, pipes []*IfacePipe, version string, tags []string, interval time.Duration,
	tlsCert, tlsKey, tlsCA, authToken string) *GRPCClient {
	return &GRPCClient{
		serverAddr: serverAddr,
		nodeID:     nodeID,
		ifacePipes: pipes,
		version:    version,
		tags:       tags,
		interval:   interval,
		tlsCert:    tlsCert,
		tlsKey:     tlsKey,
		tlsCA:      tlsCA,
		authToken:  authToken,
	}
}

func (c *GRPCClient) Connect(ctx context.Context) error {
	var opts []grpc.DialOption

	target := c.serverAddr

	// Override IPv6 connections with IPv4 when DNS returns AAAA records
	// (e.g. Cloudflare CDN) but the host has no IPv6 routing.
	// gRPC's default DNS resolver handles TLS SNI/:authority correctly;
	// we only intercept the actual TCP dial to swap v6 → v4.
	opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("parse addr %q: %w", addr, err)
		}
		ip := net.ParseIP(host)
		if ip != nil && ip.To4() == nil {
			// IPv6 address — re-resolve the original hostname to IPv4
			origHost, origPort, err := net.SplitHostPort(c.serverAddr)
			if err != nil {
				return nil, fmt.Errorf("parse server addr: %w", err)
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", origHost)
			if err != nil {
				return nil, fmt.Errorf("ipv4 lookup %s: %w", origHost, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no IPv4 address found for %s", origHost)
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(ips[0].String(), origPort))
		}
		return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}))

	if c.tlsCA != "" || (c.tlsCert != "" && c.tlsKey != "") {
		tlsCfg := &tls.Config{}

		if c.tlsCA != "" {
			caPEM, err := os.ReadFile(c.tlsCA)
			if err != nil {
				return fmt.Errorf("read CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return fmt.Errorf("failed to parse CA cert")
			}
			tlsCfg.RootCAs = pool
		}

		if c.tlsCert != "" && c.tlsKey != "" {
			cert, err := tls.LoadX509KeyPair(c.tlsCert, c.tlsKey)
			if err != nil {
				return fmt.Errorf("load client cert: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
			log.Printf("[agent] mTLS client certificate loaded")
		}

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
		log.Printf("[agent] TLS enabled for gRPC connection")
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.conn = conn
	c.client = netgazerv1.NewAgentServiceClient(conn)

	// Build interface names list
	var ifaceNames []string
	for _, p := range c.ifacePipes {
		ifaceNames = append(ifaceNames, p.Name)
	}

	// Register (send first interface for backward compat + full list)
	firstIface := ""
	if len(ifaceNames) > 0 {
		firstIface = ifaceNames[0]
	}
	resp, err := c.client.Register(ctx, &netgazerv1.RegisterRequest{
		NodeId:     c.nodeID,
		Interface:  firstIface,
		Version:    c.version,
		Tags:       c.tags,
		Interfaces: ifaceNames,
		AuthToken:  c.authToken,
	})
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if !resp.Accepted {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}
	log.Printf("[agent] registered with server: %s", resp.Message)
	if resp.SnapshotIntervalMs > 0 {
		c.interval = time.Duration(resp.SnapshotIntervalMs) * time.Millisecond
	}

	// Open bidirectional stream
	stream, err := c.client.StreamSnapshots(ctx)
	if err != nil {
		return fmt.Errorf("stream snapshots: %w", err)
	}
	c.stream = stream
	return nil
}

func (c *GRPCClient) Run(ctx context.Context) error {
	if c.stream == nil {
		return fmt.Errorf("stream not connected")
	}
	interval := c.interval
	if interval <= 0 {
		interval = 1000 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Goroutine to read server messages
	go c.readServerMessages(ctx)

	for {
		select {
		case <-ctx.Done():
			c.stream.CloseSend()
			return nil
		case <-ticker.C:
			for _, msg := range c.buildSnapshots() {
				if err := c.stream.Send(msg); err != nil {
					if err == io.EOF {
						return nil
					}
					return fmt.Errorf("send snapshot: %w", err)
				}
			}
		}
	}
}

func (c *GRPCClient) readServerMessages(ctx context.Context) {
	for {
		msg, err := c.stream.Recv()
		if err != nil {
			if err != io.EOF && ctx.Err() == nil {
				log.Printf("[agent] stream recv error: %v", err)
			}
			return
		}
		switch m := msg.Message.(type) {
		case *netgazerv1.ServerMessage_Ack:
			_ = m.Ack.ReceivedTimestampUnixMs
		case *netgazerv1.ServerMessage_ConfigUpdate:
			if m.ConfigUpdate.NewSnapshotIntervalMs > 0 {
				c.interval = time.Duration(m.ConfigUpdate.NewSnapshotIntervalMs) * time.Millisecond
				log.Printf("[agent] config update: new interval %v", c.interval)
			}
			if m.ConfigUpdate.BpfFilter != "" {
				log.Printf("[agent] config update: new BPF filter '%s' (requires agent restart to apply)", m.ConfigUpdate.BpfFilter)
			}
		case *netgazerv1.ServerMessage_InterceptUpdate:
			log.Printf("[agent] received intercept rules update (%d rules)", len(m.InterceptUpdate.Rules))
			if c.onInterceptRules != nil {
				c.onInterceptRules(m.InterceptUpdate.Rules)
			}
		}
	}
}

func (c *GRPCClient) buildSnapshots() []*netgazerv1.AgentMessage {
	now := time.Now()
	intervalSec := c.interval.Seconds()
	if intervalSec <= 0 {
		intervalSec = 1
	}

	var msgs []*netgazerv1.AgentMessage

	for _, pipe := range c.ifacePipes {
		msg := c.buildInterfaceSnapshot(pipe, now, intervalSec)
		msgs = append(msgs, msg)
	}

	return msgs
}

func (c *GRPCClient) buildInterfaceSnapshot(pipe *IfacePipe, now time.Time, intervalSec float64) *netgazerv1.AgentMessage {
	hosts := pipe.HostTracker.Snapshot()
	flows := pipe.FlowTracker.Snapshot()
	protocols := pipe.ProtoTracker.Snapshot()

	var totalBytesPerSec, totalPacketsPerSec float64

	pbHosts := make([]*netgazerv1.Host, len(hosts))
	for i, h := range hosts {
		pbHosts[i] = &netgazerv1.Host{
			Ip:              h.IP,
			Mac:             h.MAC,
			Hostname:        h.Hostname,
			BytesSent:       h.BytesSent,
			BytesReceived:   h.BytesReceived,
			PacketsSent:     h.PacketsSent,
			PacketsReceived: h.PacketsReceived,
			FirstSeenUnixMs: h.FirstSeen.UnixMilli(),
			LastSeenUnixMs:  h.LastSeen.UnixMilli(),
			Vendor:          h.Vendor,
			ActiveFlows:     int32(h.ActiveFlows),
		}
	}

	pbFlows := make([]*netgazerv1.Flow, len(flows))
	for i, f := range flows {
		pbFlows[i] = &netgazerv1.Flow{
			Id:              f.ID,
			SrcIp:           f.SrcIP,
			DstIp:           f.DstIP,
			SrcPort:         uint32(f.SrcPort),
			DstPort:         uint32(f.DstPort),
			Protocol:        f.Protocol,
			AppProtocol:     f.AppProtocol,
			Bytes:           f.Bytes,
			Packets:         f.Packets,
			FirstSeenUnixMs: f.FirstSeen.UnixMilli(),
			LastSeenUnixMs:  f.LastSeen.UnixMilli(),
		}
	}

	pbProtocols := make([]*netgazerv1.ProtocolStat, len(protocols))
	for i, p := range protocols {
		pbProtocols[i] = &netgazerv1.ProtocolStat{
			Protocol:   p.Protocol,
			Bytes:      p.Bytes,
			Packets:    p.Packets,
			Percentage: p.Percentage,
		}
		totalBytesPerSec += float64(p.Bytes)
		totalPacketsPerSec += float64(p.Packets)
	}

	totalBytesPerSec /= intervalSec
	totalPacketsPerSec /= intervalSec

	dnsQueries := pipe.DNSTracker.Top(50)
	pbDNS := make([]*netgazerv1.DNSQuery, len(dnsQueries))
	for i, q := range dnsQueries {
		pbDNS[i] = &netgazerv1.DNSQuery{
			QueryName: q.QueryName,
			Count:     q.Count,
			Bytes:     q.Bytes,
		}
	}

	psd := pipe.PacketSizeTracker.Snapshot()
	pbPSD := &netgazerv1.PacketSizeDist{
		Size_64:    psd.Size64,
		Size_128:   psd.Size128,
		Size_256:   psd.Size256,
		Size_512:   psd.Size512,
		Size_1024:  psd.Size1024,
		Size_1500:  psd.Size1500,
		SizeGt1500: psd.SizeGt1500,
	}

	tcpSummary := pipe.TCPMetrics.Snapshot()
	pbTCP := &netgazerv1.TCPMetrics{
		ActiveTcpFlows:   int32(tcpSummary.ActiveTCPFlows),
		TotalRetransmits: tcpSummary.TotalRetransmits,
		TotalRsts:        tcpSummary.TotalRSTs,
		TotalZeroWindows: tcpSummary.TotalZeroWindows,
		TotalOutOfOrder:  tcpSummary.TotalOutOfOrder,
		RttAvgMs:         tcpSummary.RTTAvgMS,
		RttMinMs:         tcpSummary.RTTMinMS,
		RttMaxMs:         tcpSummary.RTTMaxMS,
		RttSamples:       tcpSummary.RTTSamples,
	}
	msg := &netgazerv1.AgentMessage{
		NodeId:          c.nodeID,
		TimestampUnixMs: now.UnixMilli(),
		Interface:       pipe.Name,
		Snapshot: &netgazerv1.TrafficSnapshot{
			BytesPerSec:   totalBytesPerSec,
			PacketsPerSec: totalPacketsPerSec,
			FlowsCount:    int32(len(flows)),
		},
		Hosts:          pbHosts,
		Flows:          pbFlows,
		Protocols:      pbProtocols,
		DnsQueries:     pbDNS,
		PacketSizeDist: pbPSD,
		TcpMetrics:     pbTCP,
	}

	// System health is per-node, only attach on first interface to avoid duplication
	if c.systemHealth != nil {
		msg.SystemHealth = &netgazerv1.SystemHealth{
			CpuPercent:     c.systemHealth.CPUPercent,
			MemPercent:     c.systemHealth.MemPercent,
			MemUsedBytes:   c.systemHealth.MemUsedBytes,
			MemTotalBytes:  c.systemHealth.MemTotalBytes,
			DiskFreeBytes:  c.systemHealth.DiskFreeBytes,
			DiskTotalBytes: c.systemHealth.DiskTotalBytes,
			UptimeSeconds:  c.systemHealth.UptimeSeconds,
		}
	}

	return msg
}

func (c *GRPCClient) Close() {
	if c.stream != nil {
		c.stream.CloseSend()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
