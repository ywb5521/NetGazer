package collector

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/netgazer/backend/internal/models"
)

// Config holds the collector configuration.
type Config struct {
	NetFlowPort int // UDP port for NetFlow (v5/v9), default 2055
	SFlowPort   int // UDP port for sFlow, default 6343
}

// FlowRecord is a normalized flow record from any collector source.
type FlowRecord struct {
	SrcIP    net.IP
	DstIP    net.IP
	SrcPort  uint16
	DstPort  uint16
	Protocol string
	Bytes    uint64
	Packets  uint64
	NodeID   string // pseudo node_id for the flow source
}

// Callback is called for each batch of flow records received.
type Callback func(records []FlowRecord)

// Collector listens for NetFlow and sFlow UDP packets.
type Collector struct {
	cfg      Config
	callback Callback
	mu       sync.Mutex
}

// New creates a new Collector.
func New(cfg Config, cb Callback) *Collector {
	if cfg.NetFlowPort == 0 {
		cfg.NetFlowPort = 2055
	}
	if cfg.SFlowPort == 0 {
		cfg.SFlowPort = 6343
	}
	return &Collector{cfg: cfg, callback: cb}
}

// Start begins listening for NetFlow/sFlow on configured UDP ports.
func (c *Collector) Start(ctx context.Context) error {
	if c.cfg.NetFlowPort > 0 {
		go c.listenUDP(ctx, c.cfg.NetFlowPort, "netflow")
	}
	if c.cfg.SFlowPort > 0 {
		go c.listenUDP(ctx, c.cfg.SFlowPort, "sflow")
	}
	return nil
}

func (c *Collector) listenUDP(ctx context.Context, port int, proto string) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("[collector] failed to listen on :%d for %s: %v", port, proto, err)
		return
	}
	defer conn.Close()

	log.Printf("[collector] listening for %s on :%d", proto, port)

	buf := make([]byte, 65536)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		go c.processPacket(proto, data, remoteAddr.IP)
	}
}

func (c *Collector) processPacket(proto string, data []byte, srcIP net.IP) {
	var records []FlowRecord
	nodeID := "flow:" + srcIP.String()

	switch proto {
	case "netflow":
		records = c.processNetFlow(data, srcIP, nodeID)
	case "sflow":
		records = c.processSFlow(data, srcIP, nodeID)
	default:
		return
	}

	if len(records) > 0 && c.callback != nil {
		c.callback(records)
	}
}

func (c *Collector) processNetFlow(data []byte, srcIP net.IP, nodeID string) []FlowRecord {
	// Try v5 first
	if recs, err := parseNetFlow5(data, srcIP); err == nil {
		records := make([]FlowRecord, 0, len(recs))
		for _, r := range recs {
			records = append(records, FlowRecord{
				SrcIP:    r.SrcAddr,
				DstIP:    r.DstAddr,
				SrcPort:  r.SrcPort,
				DstPort:  r.DstPort,
				Protocol: protoName(r.Protocol),
				Bytes:    uint64(r.Bytes),
				Packets:  uint64(r.Packets),
				NodeID:   nodeID,
			})
		}
		return records
	}

	// Try v9
	if recs, _, err := parseNetFlow9(data, srcIP); err == nil {
		records := make([]FlowRecord, 0, len(recs))
		for _, r := range recs {
			records = append(records, FlowRecord{
				SrcIP:    r.SrcIP,
				DstIP:    r.DstIP,
				SrcPort:  r.SrcPort,
				DstPort:  r.DstPort,
				Protocol: protoName(r.Protocol),
				Bytes:    r.Bytes,
				Packets:  r.Packets,
				NodeID:   nodeID,
			})
		}
		return records
	}

	return nil
}

func (c *Collector) processSFlow(data []byte, srcIP net.IP, nodeID string) []FlowRecord {
	recs, err := parseSFlow(data, srcIP)
	if err != nil {
		return nil
	}

	records := make([]FlowRecord, 0, len(recs))
	for _, r := range recs {
		records = append(records, FlowRecord{
			SrcIP:    r.SrcIP,
			DstIP:    r.DstIP,
			SrcPort:  r.SrcPort,
			DstPort:  r.DstPort,
			Protocol: protoName(r.Protocol),
			Bytes:    r.Bytes,
			Packets:  r.Packets,
			NodeID:   nodeID,
		})
	}
	return records
}

// ConvertToHosts converts flow records to netgazer Host models.
func ConvertToHosts(records []FlowRecord) []models.Host {
	hostMap := make(map[string]*models.Host)
	now := time.Now()

	for _, r := range records {
		if r.SrcIP != nil && !r.SrcIP.IsUnspecified() {
			key := r.SrcIP.String()
			if h, ok := hostMap[key]; ok {
				h.BytesSent += r.Bytes
				h.PacketsSent += uint64(r.Packets)
				h.LastSeen = now
				h.ActiveFlows++
			} else {
				hostMap[key] = &models.Host{
					IP:          r.SrcIP.String(),
					BytesSent:   r.Bytes,
					PacketsSent: uint64(r.Packets),
					FirstSeen:   now,
					LastSeen:    now,
					NodeID:      r.NodeID,
				}
			}
		}
		if r.DstIP != nil && !r.DstIP.IsUnspecified() {
			key := r.DstIP.String()
			if h, ok := hostMap[key]; ok {
				h.BytesReceived += r.Bytes
				h.PacketsReceived += uint64(r.Packets)
				h.LastSeen = now
			} else {
				hostMap[key] = &models.Host{
					IP:              r.DstIP.String(),
					BytesReceived:   r.Bytes,
					PacketsReceived: uint64(r.Packets),
					FirstSeen:       now,
					LastSeen:        now,
					NodeID:          r.NodeID,
				}
			}
		}
	}

	hosts := make([]models.Host, 0, len(hostMap))
	for _, h := range hostMap {
		hosts = append(hosts, *h)
	}
	return hosts
}
