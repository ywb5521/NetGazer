package tracker

import (
	"sync"
	"time"
)

// TCPFlowMetrics holds per-flow TCP statistics.
type TCPFlowMetrics struct {
	RetransmitCount int64
	RSTCount        int64
	ZeroWindowCount int64
	OutOfOrderCount int64
	SYNCount        int64
	FINCount        int64
	RTTAvgMS        float64
	RTTMinMS        float64
	RTTMaxMS        float64
	RTTSamples      int64
}

// TCPMetricsTracker tracks TCP-level health metrics across all flows.
type TCPMetricsTracker struct {
	mu    sync.RWMutex
	flows map[string]*tcpFlowState

	// Aggregated global counters
	totalRetransmits  int64
	totalRSTs         int64
	totalZeroWindows  int64
	totalOutOfOrder   int64
	totalExpectedPkts int64
	totalLostPkts     int64
}

type tcpFlowState struct {
	key          string
	srcSeqMax    uint32
	dstSeqMax    uint32
	expectedPkts int64
	lostPkts     int64
	synSentTS    time.Time
	synAckTS     time.Time
	rttSamples   []float64
	lastSeen     time.Time
	packetCount  int64
}

func NewTCPMetricsTracker() *TCPMetricsTracker {
	return &TCPMetricsTracker{
		flows: make(map[string]*tcpFlowState),
	}
}

// flowKey builds a canonical key from connection 4-tuple.
func tcpFlowKey(srcIP, dstIP string, srcPort, dstPort uint16) string {
	// Canonical: lexicographically smaller IP:port first
	if srcIP < dstIP || (srcIP == dstIP && srcPort < dstPort) {
		return srcIP + ":" + itoa16(srcPort) + "-" + dstIP + ":" + itoa16(dstPort)
	}
	return dstIP + ":" + itoa16(dstPort) + "-" + srcIP + ":" + itoa16(srcPort)
}

func itoa16(v uint16) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 5)
	for v > 0 {
		buf = append([]byte{byte(v%10) + '0'}, buf...)
		v /= 10
	}
	return string(buf)
}

// TCPPacketInfo holds the TCP-level fields extracted from a parsed packet.
type TCPPacketInfo struct {
	SrcIP   string
	DstIP   string
	SrcPort uint16
	DstPort uint16
	Seq     uint32
	Flags   TCPFlags
	Window  uint16
	Len     int // payload length (0 for pure ACK/SYN)
	TS      time.Time
}

type TCPFlags struct {
	SYN, RST, FIN, ACK bool
}

// Process processes a single TCP packet and updates flow state.
func (t *TCPMetricsTracker) Process(info TCPPacketInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := tcpFlowKey(info.SrcIP, info.DstIP, info.SrcPort, info.DstPort)
	state, ok := t.flows[key]
	if !ok {
		state = &tcpFlowState{key: key}
		t.flows[key] = state
	}
	state.lastSeen = info.TS
	state.packetCount++

	// Determine direction: is this src→dst or dst→src?
	srcIsFirst := info.SrcIP+":"+itoa16(info.SrcPort) < info.DstIP+":"+itoa16(info.DstPort)

	// Track sequence numbers per direction for retransmission detection
	if srcIsFirst {
		t.updateSeqDirection(info, &state.srcSeqMax, state)
	} else {
		t.updateSeqDirection(info, &state.dstSeqMax, state)
	}

	// Count flags
	if info.Flags.RST {
		t.totalRSTs++
	}
	if info.Flags.SYN && !info.Flags.ACK {
		state.synSentTS = info.TS
	}
	if info.Flags.SYN && info.Flags.ACK {
		state.synAckTS = info.TS
		// RTT estimate from handshake
		if !state.synSentTS.IsZero() {
			rtt := info.TS.Sub(state.synSentTS).Seconds() * 1000
			if rtt > 0 && rtt < 10000 { // sanity: RTT < 10s
				state.rttSamples = append(state.rttSamples, rtt)
				if len(state.rttSamples) > 16 {
					state.rttSamples = state.rttSamples[1:]
				}
			}
		}
	}

	// Zero window detection
	if info.Flags.ACK && info.Window == 0 {
		t.totalZeroWindows++
	}
}

func (t *TCPMetricsTracker) updateSeqDirection(info TCPPacketInfo, seqMax *uint32, state *tcpFlowState) {
	if info.Len == 0 && !info.Flags.SYN && !info.Flags.FIN {
		return // pure ACK, no sequence consumption
	}

	// SYN and FIN consume 1 sequence number
	consumed := uint32(info.Len)
	if info.Flags.SYN {
		consumed++
	}
	if info.Flags.FIN {
		consumed++
	}

	if *seqMax == 0 {
		*seqMax = info.Seq + consumed
		state.expectedPkts++
		t.totalExpectedPkts++
		return
	}

	// Retransmission: sequence number is behind our maximum seen
	if info.Seq < *seqMax {
		t.totalRetransmits++
		state.lostPkts++
		t.totalLostPkts++
	} else if info.Seq > *seqMax {
		// Gap in sequence space indicates lost packets
		t.totalOutOfOrder++
		state.lostPkts++
		t.totalLostPkts++
	}

	state.expectedPkts++
	t.totalExpectedPkts++

	if info.Seq+consumed > *seqMax {
		*seqMax = info.Seq + consumed
	}
}

// ExpireStale removes flows that have been idle for the given duration.
func (t *TCPMetricsTracker) ExpireStale(idleTimeout time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-idleTimeout)
	removed := 0
	for key, state := range t.flows {
		if state.lastSeen.Before(cutoff) {
			delete(t.flows, key)
			removed++
		}
	}
	return removed
}

// Snapshot returns a summary of TCP metrics for all active flows.
func (t *TCPMetricsTracker) Snapshot() TCPMetricsSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var rttSum float64
	var rttSamples int64
	var rttMin, rttMax float64

	for _, state := range t.flows {
		var sum float64
		for _, s := range state.rttSamples {
			sum += s
			if rttMin == 0 || s < rttMin {
				rttMin = s
			}
			if s > rttMax {
				rttMax = s
			}
		}
		rttSum += sum
		rttSamples += int64(len(state.rttSamples))
	}

	var rttAvg float64
	if rttSamples > 0 {
		rttAvg = rttSum / float64(rttSamples)
	}

	var lossPct float64
	if t.totalExpectedPkts > 0 {
		lossPct = float64(t.totalLostPkts) / float64(t.totalExpectedPkts) * 100
	}

	return TCPMetricsSummary{
		ActiveTCPFlows:    len(t.flows),
		TotalRetransmits:  t.totalRetransmits,
		TotalRSTs:         t.totalRSTs,
		TotalZeroWindows:  t.totalZeroWindows,
		TotalOutOfOrder:   t.totalOutOfOrder,
		RTTAvgMS:          rttAvg,
		RTTMinMS:          rttMin,
		RTTMaxMS:          rttMax,
		RTTSamples:        rttSamples,
		TotalExpectedPkts: t.totalExpectedPkts,
		TotalLostPkts:     t.totalLostPkts,
		PacketLossPct:     lossPct,
	}
}

// TCPMetricsSummary is a point-in-time snapshot of all TCP metrics.
type TCPMetricsSummary struct {
	ActiveTCPFlows    int
	TotalRetransmits  int64
	TotalRSTs         int64
	TotalZeroWindows  int64
	TotalOutOfOrder   int64
	RTTAvgMS          float64
	RTTMinMS          float64
	RTTMaxMS          float64
	RTTSamples        int64
	TotalExpectedPkts int64
	TotalLostPkts     int64
	PacketLossPct     float64
}
