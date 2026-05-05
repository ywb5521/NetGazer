package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gtopng/backend/internal/agenthealth"
	"github.com/gtopng/backend/internal/capture"
	"github.com/gtopng/backend/internal/config"
	"github.com/gtopng/backend/internal/ndpi"
	"github.com/gtopng/backend/internal/reporter"
	"github.com/gtopng/backend/internal/tracker"
)

var version = "0.1.0"

func main() {
	cfg := config.ParseAgentFlags()

	log.Printf("[agent] gtopng-agent %s starting on interfaces %v", version, cfg.Interfaces)
	log.Printf("[agent] server: %s | node-id: %s", cfg.ServerAddr, cfg.NodeID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tags := parseTags(cfg.Tags)

	// Initialize nDPI deep packet inspection engine
	var ndpiEngine *ndpi.Engine
	if cfg.ProtoEngine == "ndpi" || cfg.ProtoEngine == "both" {
		var err error
		ndpiEngine, err = ndpi.NewEngine()
		if err != nil {
			log.Printf("[agent] WARNING: nDPI engine init failed: %v (falling back)", err)
		} else {
			log.Printf("[agent] nDPI engine initialized (%d protocols)", ndpi.GetNumSupported())
			defer ndpiEngine.Close()
		}
	}

	// Initialize OpenGFW protocol analyzer detector
	var ogfwDetector *capture.OGFWDetector
	if cfg.ProtoEngine == "opengfw" || cfg.ProtoEngine == "both" {
		ogfwDetector = capture.NewOGFWDetector()
		log.Printf("[agent] OpenGFW analyzers initialized (TCP: HTTP,TLS,SSH,SOCKS,Trojan,FET | UDP: DNS,QUIC,WireGuard,OpenVPN)")
	}

	// Create per-interface capture + trackers pipeline
	var pipes []*reporter.IfacePipe
	var engines []*capture.Engine

	for _, iface := range cfg.Interfaces {
		eng, err := capture.NewEngine(iface, cfg.BPFFilter, true)
		if err != nil {
			log.Fatalf("[agent] failed to create capture engine for %s: %v", iface, err)
		}
		engines = append(engines, eng)
		defer eng.Stop()

		log.Printf("[agent] capture engine started on %s (link type: %s)", iface, eng.LinkType())

		pipe := &reporter.IfacePipe{
			Name:              iface,
			HostTracker:       tracker.NewHostTracker(),
			FlowTracker:       tracker.NewFlowTracker(),
			ProtoTracker:      tracker.NewProtocolTracker(),
			DNSTracker:        tracker.NewDNSTracker(),
			PacketSizeTracker: tracker.NewPacketSizeTracker(),
			TCPMetrics:        tracker.NewTCPMetricsTracker(),
		}
		pipes = append(pipes, pipe)
	}

	r := reporter.NewGRPCClient(cfg.ServerAddr, cfg.NodeID, pipes, version, tags, 0,
		cfg.TLSCert, cfg.TLSKey, cfg.TLSCA, cfg.AuthToken)

	if err := r.Connect(ctx); err != nil {
		log.Printf("[agent] WARNING: failed to connect to server: %v (will retry)", err)
	}

	// nDPI flow idle cleanup goroutine (shared across all interfaces)
	if ndpiEngine != nil {
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					removed := ndpiEngine.IdleCleanup(120 * time.Second)
					if removed > 0 {
						log.Printf("[agent] nDPI: cleaned up %d idle flows (%d active)", removed, ndpiEngine.FlowCount())
					}
				}
			}
		}()
	}

	// OGFW detector flow cleanup goroutine
	if ogfwDetector != nil {
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					ogfwDetector.CleanupFlows()
				}
			}
		}()
	}

	// Start capture + analysis + tracking per interface
	for i, iface := range cfg.Interfaces {
		eng := engines[i]
		pipe := pipes[i]
		ifaceName := iface

		log.Printf("[agent] starting pipeline for %s", ifaceName)

		var analyzer *capture.Analyzer
		if ogfwDetector != nil && ndpiEngine != nil {
			analyzer = capture.NewAnalyzerWithOpenGFW(ndpiEngine, ogfwDetector, cfg.ProtoEngine)
		} else if ndpiEngine != nil {
			analyzer = capture.NewAnalyzerWithNDPI(ndpiEngine)
		} else {
			analyzer = capture.NewAnalyzer()
		}
		packetCh := eng.Start(ctx)
		parsedCh := analyzer.Start(ctx, packetCh)

		// Packet processing goroutine per interface
		go func() {
			for p := range parsedCh {
				p.Interface = ifaceName
				pipe.HostTracker.Process(p, cfg.NodeID)
				pipe.FlowTracker.Process(p, cfg.NodeID)
				pipe.ProtoTracker.Process(p, cfg.NodeID)
				pipe.DNSTracker.Process(p, cfg.NodeID)
				pipe.PacketSizeTracker.Process(p, cfg.NodeID)
				// TCP metrics tracking
				if p.Protocol == "TCP" {
					pipe.TCPMetrics.Process(tracker.TCPPacketInfo{
						SrcIP:   p.SrcIP.String(),
						DstIP:   p.DstIP.String(),
						SrcPort: p.SrcPort,
						DstPort: p.DstPort,
						Seq:     p.TCPSeq,
						Flags: tracker.TCPFlags{
							SYN: p.TCPSYN,
							RST: p.TCPRST,
							FIN: p.TCPFIN,
							ACK: p.TCPACK,
						},
						Window: p.TCPWindow,
						Len:    p.Length,
						TS:     time.UnixMilli(p.Timestamp),
					})
				}
			}
		}()

		// Flow expiration goroutine per interface
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					pipe.FlowTracker.ExpireStale(120 * time.Second)
				}
			}
		}()
	}

	// Reporter goroutine (single stream, sends per-interface messages)
	go func() {
		for {
			if err := r.Run(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[agent] reporter error: %v, reconnecting...", err)
				if err := r.Connect(ctx); err != nil {
					log.Printf("[agent] reconnect failed: %v", err)
				}
				time.Sleep(3 * time.Second)
			}
		}
	}()

	// System health collection goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.SetHealth(agenthealth.Collect())
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\n[agent] shutting down...")
	cancel()
	r.Close()
}

func parseTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}
