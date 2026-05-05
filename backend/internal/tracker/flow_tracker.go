package tracker

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"github.com/gtopng/backend/internal/capture"
	"github.com/gtopng/backend/internal/models"
)

type FlowTracker struct {
	mu    sync.RWMutex
	flows map[string]*models.Flow
}

func NewFlowTracker() *FlowTracker {
	return &FlowTracker{
		flows: make(map[string]*models.Flow),
	}
}

func (t *FlowTracker) Process(p capture.ParsedPacket, nodeID string) {
	if p.Protocol == "ARP" || p.Protocol == "" {
		return
	}

	key := flowKey(p.SrcIP.String(), p.DstIP.String(), p.SrcPort, p.DstPort, p.Protocol)
	if key == "" {
		return
	}
	id := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	f, ok := t.flows[key]
	if !ok {
		f = &models.Flow{
			ID:          id,
			SrcIP:       p.SrcIP.String(),
			DstIP:       p.DstIP.String(),
			SrcPort:     p.SrcPort,
			DstPort:     p.DstPort,
			Protocol:    p.Protocol,
			AppProtocol: p.AppProto,
			FirstSeen:   now,
			NodeID:      nodeID,
			VlanID:      p.VlanID,
		}
		t.flows[key] = f
	}
	f.Bytes += uint64(p.Length)
	f.Packets++
	f.LastSeen = now
	// Prefer more detailed app protocol (enriched with SNI/Host)
	if p.AppProto != "" && p.AppProto != p.Protocol {
		if f.AppProtocol == p.Protocol || len(p.AppProto) > len(f.AppProtocol) {
			f.AppProtocol = p.AppProto
		}
	}
}

func flowKey(srcIP, dstIP string, srcPort, dstPort uint16, protocol string) string {
	if srcIP == "" || dstIP == "" || srcIP == "<nil>" || dstIP == "<nil>" {
		return ""
	}
	// Canonicalize: always put lower IP first for the key to avoid duplicates
	if srcIP < dstIP || (srcIP == dstIP && srcPort <= dstPort) {
		return fmt.Sprintf("%s:%d-%s:%d-%s", srcIP, srcPort, dstIP, dstPort, protocol)
	}
	return fmt.Sprintf("%s:%d-%s:%d-%s", dstIP, dstPort, srcIP, srcPort, protocol)
}

func (t *FlowTracker) Snapshot() []models.Flow {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]models.Flow, 0, len(t.flows))
	for _, f := range t.flows {
		result = append(result, *f)
	}
	t.flows = make(map[string]*models.Flow)
	return result
}

func (t *FlowTracker) ExpireStale(maxAge time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	count := 0
	for k, f := range t.flows {
		if f.LastSeen.Before(cutoff) {
			delete(t.flows, k)
			count++
		}
	}
	return count
}

func (t *FlowTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.flows)
}
