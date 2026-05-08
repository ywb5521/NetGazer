package tracker

import (
	"encoding/binary"
	"sync"
	"time"
)

// LatencySample holds a single latency measurement.
type LatencySample struct {
	Protocol string
	ValueMS  float64
	TS       time.Time
}

// LatencyTracker tracks per-protocol application latency via packet timing.
type LatencyTracker struct {
	mu sync.Mutex

	// DNS transaction tracking: query ID -> send timestamp
	dnsPending map[uint16]time.Time

	// TLS handshake tracking: flow key -> ClientHello timestamp
	tlsHandshakes map[string]time.Time

	// TCP handshake RTT: flow key -> SYN timestamp
	tcpHandshakes map[string]time.Time

	// Circular buffers for recent samples per protocol
	dnsSamples  []LatencySample
	tlsSamples  []LatencySample
	httpSamples []LatencySample
	maxSamples  int
}

func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{
		dnsPending:    make(map[uint16]time.Time),
		tlsHandshakes: make(map[string]time.Time),
		tcpHandshakes: make(map[string]time.Time),
		maxSamples:    256,
	}
}

// LatencyPacketInfo holds the fields needed for latency tracking.
type LatencyPacketInfo struct {
	SrcIP    string
	DstIP    string
	SrcPort  uint16
	DstPort  uint16
	Protocol string // "TCP" or "UDP"
	AppProto string // "DNS", "TLS", etc.
	TS       time.Time
	TCPFlags TCPFlags
	Payload  []byte
}

// Process processes a packet for latency tracking.
func (t *LatencyTracker) Process(info LatencyPacketInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch info.AppProto {
	case "DNS":
		t.processDNS(info)
	}
	if info.Protocol == "TCP" {
		t.processTCP(info)
	}
	if info.AppProto == "TLS" || (len(info.Payload) > 0 && info.Payload[0] == 0x16) {
		t.processTLS(info)
	}
}

func (t *LatencyTracker) processDNS(info LatencyPacketInfo) {
	if len(info.Payload) < 12 {
		return
	}
	txID := binary.BigEndian.Uint16(info.Payload[0:2])
	isQuery := info.Payload[2]&0x80 == 0

	if isQuery && (info.DstPort == 53 || info.SrcPort != 53) {
		t.dnsPending[txID] = info.TS
		return
	}

	if !isQuery {
		if start, ok := t.dnsPending[txID]; ok {
			latency := info.TS.Sub(start).Seconds() * 1000
			if latency > 0 && latency < 10000 {
				t.dnsSamples = append(t.dnsSamples, LatencySample{
					Protocol: "DNS",
					ValueMS:  latency,
					TS:       info.TS,
				})
				if len(t.dnsSamples) > t.maxSamples {
					t.dnsSamples = t.dnsSamples[1:]
				}
			}
			delete(t.dnsPending, txID)
		}
	}

	// Clean old pending
	if len(t.dnsPending) > 1024 {
		cutoff := time.Now().Add(-10 * time.Second)
		for id, ts := range t.dnsPending {
			if ts.Before(cutoff) {
				delete(t.dnsPending, id)
			}
		}
	}
}

func (t *LatencyTracker) processTCP(info LatencyPacketInfo) {
	flowKey := info.SrcIP + ":" + itoa16(info.SrcPort) + "-" + info.DstIP + ":" + itoa16(info.DstPort)

	if info.TCPFlags.SYN && !info.TCPFlags.ACK {
		t.tcpHandshakes[flowKey] = info.TS
		return
	}

	if info.TCPFlags.SYN && info.TCPFlags.ACK {
		reverseKey := info.DstIP + ":" + itoa16(info.DstPort) + "-" + info.SrcIP + ":" + itoa16(info.SrcPort)
		if start, ok := t.tcpHandshakes[reverseKey]; ok {
			latency := info.TS.Sub(start).Seconds() * 1000
			if latency > 0 && latency < 5000 {
				t.httpSamples = append(t.httpSamples, LatencySample{
					Protocol: "TCP",
					ValueMS:  latency,
					TS:       info.TS,
				})
				if len(t.httpSamples) > t.maxSamples {
					t.httpSamples = t.httpSamples[1:]
				}
			}
			delete(t.tcpHandshakes, reverseKey)
		}
	}

	// Clean old handshakes
	if len(t.tcpHandshakes) > 1024 {
		cutoff := time.Now().Add(-30 * time.Second)
		for k, ts := range t.tcpHandshakes {
			if ts.Before(cutoff) {
				delete(t.tcpHandshakes, k)
			}
		}
	}
}

func (t *LatencyTracker) processTLS(info LatencyPacketInfo) {
	if len(info.Payload) < 6 {
		return
	}
	flowKey := info.SrcIP + ":" + itoa16(info.SrcPort) + "-" + info.DstIP + ":" + itoa16(info.DstPort)

	// TLS record: type(1), version(2), length(2)
	recordType := info.Payload[0]
	if recordType != 0x16 { // handshake
		return
	}
	if len(info.Payload) < 6 {
		return
	}
	// handshake type is at offset 5 in TLS record
	handshakeType := info.Payload[5]

	if handshakeType == 0x01 { // ClientHello
		t.tlsHandshakes[flowKey] = info.TS
		return
	}
	if handshakeType == 0x02 { // ServerHello
		reverseKey := info.DstIP + ":" + itoa16(info.DstPort) + "-" + info.SrcIP + ":" + itoa16(info.SrcPort)
		if start, ok := t.tlsHandshakes[reverseKey]; ok {
			latency := info.TS.Sub(start).Seconds() * 1000
			if latency > 0 && latency < 10000 {
				t.tlsSamples = append(t.tlsSamples, LatencySample{
					Protocol: "TLS",
					ValueMS:  latency,
					TS:       info.TS,
				})
				if len(t.tlsSamples) > t.maxSamples {
					t.tlsSamples = t.tlsSamples[1:]
				}
			}
			delete(t.tlsHandshakes, reverseKey)
		}
	}

	if len(t.tlsHandshakes) > 1024 {
		cutoff := time.Now().Add(-30 * time.Second)
		for k, ts := range t.tlsHandshakes {
			if ts.Before(cutoff) {
				delete(t.tlsHandshakes, k)
			}
		}
	}
}

// LatencySnapshot returns a summary of application latency metrics.
func (t *LatencyTracker) Snapshot() LatencySnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()

	return LatencySnapshot{
		DNSLatency: calcStats(t.dnsSamples),
		TLSLatency: calcStats(t.tlsSamples),
		TCPLatency: calcStats(t.httpSamples),
	}
}

func calcStats(samples []LatencySample) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}
	var sum, minV, maxV float64
	minV = samples[0].ValueMS
	for _, s := range samples {
		sum += s.ValueMS
		if s.ValueMS < minV {
			minV = s.ValueMS
		}
		if s.ValueMS > maxV {
			maxV = s.ValueMS
		}
	}
	return LatencyStats{
		Samples: len(samples),
		AvgMS:   sum / float64(len(samples)),
		MinMS:   minV,
		MaxMS:   maxV,
	}
}

// LatencyStats holds summary statistics for a protocol's latency.
type LatencyStats struct {
	Samples int
	AvgMS   float64
	MinMS   float64
	MaxMS   float64
}

// LatencySnapshot holds latency metrics for all tracked protocols.
type LatencySnapshot struct {
	DNSLatency LatencyStats `json:"dns_latency"`
	TLSLatency LatencyStats `json:"tls_latency"`
	TCPLatency LatencyStats `json:"tcp_latency"`
}
