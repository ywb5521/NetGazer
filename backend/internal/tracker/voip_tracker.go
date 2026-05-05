package tracker

import (
	"sync"
	"time"
)

// RTPPacketInfo holds RTP packet fields for VoIP tracking.
type RTPPacketInfo struct {
	SrcIP    string
	DstIP    string
	SrcPort  uint16
	DstPort  uint16
	SSRC     uint32
	SeqNum   uint16
	RTPTS    uint32
	PT       uint8
	Size     int
	Arrival  time.Time
	Codec    string
}

type rtpSession struct {
	SSRC        uint32
	SrcIP       string
	DstIP       string
	SrcPort     uint16
	DstPort     uint16
	FirstSeq    uint16
	LastSeq     uint16
	Packets     int64
	Bytes       int64
	LostPkts    int64
	TotalJitter float64
	MaxJitter   float64
	AvgJitter   float64
	LastRTPTS   uint32
	LastArrival time.Time
	InterJitter float64 // current interarrival jitter (RFC 3550)
	Codec       string
	FirstSeen   time.Time
	LastSeen    time.Time
	Active      bool
}

// VOIPTracker tracks RTP flows and computes VoIP quality metrics.
type VOIPTracker struct {
	mu       sync.Mutex
	sessions map[uint32]*rtpSession // keyed by SSRC
	maxSessions int
}

func NewVOIPTracker() *VOIPTracker {
	return &VOIPTracker{
		sessions: make(map[uint32]*rtpSession),
		maxSessions: 4096,
	}
}

// Process processes an RTP packet for VoIP tracking.
func (t *VOIPTracker) Process(info RTPPacketInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()

	sess, ok := t.sessions[info.SSRC]
	if !ok {
		if len(t.sessions) >= t.maxSessions {
			t.evictOldest()
		}
		sess = &rtpSession{
			SSRC:      info.SSRC,
			SrcIP:     info.SrcIP,
			DstIP:     info.DstIP,
			SrcPort:   info.SrcPort,
			DstPort:   info.DstPort,
			FirstSeq:  info.SeqNum,
			LastSeq:   info.SeqNum,
			Codec:     info.Codec,
			FirstSeen: info.Arrival,
			LastSeen:  info.Arrival,
			Active:    true,
		}
		t.sessions[info.SSRC] = sess
		sess.Packets = 1
		sess.Bytes = int64(info.Size)
		return
	}

	sess.Packets++
	sess.Bytes += int64(info.Size)
	sess.LastSeen = info.Arrival
	sess.Active = true

	// RFC 3550 interarrival jitter calculation
	// J(i) = J(i-1) + (|D(i-1,i)| - J(i-1)) / 16
	if !sess.LastArrival.IsZero() && sess.LastRTPTS != 0 {
		arrivalDelta := info.Arrival.Sub(sess.LastArrival).Seconds() * 1000 // ms
		rtpDelta := float64(int32(info.RTPTS-sess.LastRTPTS)) / 8.0        // ms (8kHz typical)

		if rtpDelta < 0 {
			rtpDelta += 65536 / 8.0 // handle 16-bit rollover
		}
		d := arrivalDelta - rtpDelta
		if d < 0 {
			d = -d
		}
		sess.InterJitter += (d - sess.InterJitter) / 16.0
		if d > sess.MaxJitter {
			sess.MaxJitter = d
		}
		sess.TotalJitter += d
	}

	sess.LastArrival = info.Arrival
	sess.LastRTPTS = info.RTPTS

	// Detect packet loss from sequence gap
	expected := sess.LastSeq + 1
	if sess.LastSeq != 0 {
		gap := int32(info.SeqNum) - int32(expected)
		if gap > 0 {
			sess.LostPkts += int64(gap)
		} else if gap < -32768 {
			// Sequence number wraparound (16-bit)
			gap = 65536 - int32(expected) + int32(info.SeqNum)
			sess.LostPkts += int64(gap)
		}
	}
	sess.LastSeq = info.SeqNum
}

func (t *VOIPTracker) evictOldest() {
	var oldest uint32
	var oldestTime time.Time
	first := true
	for ssrc, s := range t.sessions {
		if first || s.LastSeen.Before(oldestTime) {
			oldest = ssrc
			oldestTime = s.LastSeen
			first = false
		}
	}
	if !first {
		delete(t.sessions, oldest)
	}
}

// MarkInactive marks sessions with LastSeen older than the given cutoff as inactive.
func (t *VOIPTracker) MarkInactive(cutoff time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	before := time.Now().Add(-cutoff)
	for _, s := range t.sessions {
		if s.LastSeen.Before(before) {
			s.Active = false
		}
	}
}

// VOIPSessionJSON holds a VoIP session for API export.
type VOIPSessionJSON struct {
	SSRC        uint32  `json:"ssrc"`
	SrcIP       string  `json:"src_ip"`
	DstIP       string  `json:"dst_ip"`
	SrcPort     uint16  `json:"src_port"`
	DstPort     uint16  `json:"dst_port"`
	Packets     int64   `json:"packets"`
	Bytes       int64   `json:"bytes"`
	LostPkts    int64   `json:"lost_packets"`
	LossPct     float64 `json:"loss_pct"`
	JitterMS    float64 `json:"jitter_ms"`
	MaxJitterMS float64 `json:"max_jitter_ms"`
	MOS         float64 `json:"mos"`
	Codec       string  `json:"codec"`
	FirstSeen   int64   `json:"first_seen"`
	LastSeen    int64   `json:"last_seen"`
	Active      bool    `json:"active"`
}

// VOIPStats holds aggregate VoIP metrics.
type VOIPStats struct {
	ActiveSessions int     `json:"active_sessions"`
	TotalSessions  int     `json:"total_sessions"`
	TotalPackets   int64   `json:"total_packets"`
	TotalBytes     int64   `json:"total_bytes"`
	TotalLost      int64   `json:"total_lost"`
	AvgJitterMS    float64 `json:"avg_jitter_ms"`
	MinMOS         float64 `json:"min_mos"`
	AvgMOS         float64 `json:"avg_mos"`
}

// Snapshot returns all active sessions and aggregate stats.
func (t *VOIPTracker) Snapshot() ([]VOIPSessionJSON, VOIPStats) {
	t.mu.Lock()
	defer t.mu.Unlock()

	sessions := make([]VOIPSessionJSON, 0, len(t.sessions))
	stats := VOIPStats{}

	for _, s := range t.sessions {
		var lossPct, avgJitter float64
		if s.Packets > 0 {
			lossPct = float64(s.LostPkts) / float64(s.Packets) * 100
			if lossPct > 100 {
				lossPct = 100
			}
			avgJitter = s.TotalJitter / float64(s.Packets)
		}

		mos := estimateMOS(lossPct, s.InterJitter, s.Codec, nil)

		sessions = append(sessions, VOIPSessionJSON{
			SSRC:        s.SSRC,
			SrcIP:       s.SrcIP,
			DstIP:       s.DstIP,
			SrcPort:     s.SrcPort,
			DstPort:     s.DstPort,
			Packets:     s.Packets,
			Bytes:       s.Bytes,
			LostPkts:    s.LostPkts,
			LossPct:     lossPct,
			JitterMS:    avgJitter,
			MaxJitterMS: s.MaxJitter,
			MOS:         mos,
			Codec:       s.Codec,
			FirstSeen:   s.FirstSeen.UnixMilli(),
			LastSeen:    s.LastSeen.UnixMilli(),
			Active:      s.Active,
		})

		stats.TotalSessions++
		if s.Active {
			stats.ActiveSessions++
		}
		stats.TotalPackets += s.Packets
		stats.TotalBytes += s.Bytes
		stats.TotalLost += s.LostPkts
		stats.AvgJitterMS += avgJitter
	}

	if stats.TotalSessions > 0 {
		stats.AvgJitterMS /= float64(stats.TotalSessions)
	}

	// Compute MOS stats
	if len(sessions) > 0 {
		stats.MinMOS = 4.5
		for _, s := range sessions {
			if s.MOS < stats.MinMOS {
				stats.MinMOS = s.MOS
			}
			stats.AvgMOS += s.MOS
		}
		stats.AvgMOS /= float64(len(sessions))
	}

	return sessions, stats
}

// estimateMOS computes MOS (1-5 scale) using a simplified E-model.
// Takes packet loss %, jitter in ms, codec name, and optional delay in ms.
func estimateMOS(lossPct, jitterMS float64, codec string, delayMS *float64) float64 {
	// Base R-factor per codec (G.711 reference = 93.2)
	baseR := 93.2
	switch codec {
	case "G711", "PCMA", "PCMU", "g711":
		baseR = 93.2
	case "G729", "g729":
		baseR = 82.0
	case "G722", "g722":
		baseR = 93.2
	case "G723", "g723":
		baseR = 78.0
	case "G726", "g726":
		baseR = 88.0
	case "GSM", "gsm":
		baseR = 73.0
	case "AMR", "amr":
		baseR = 80.0
	case "Opus", "opus":
		baseR = 94.0
	case "iLBC", "ilbc":
		baseR = 77.0
	}

	// Packet loss impairment: Ie = a + b * ln(1 + lossPct/100 * c)
	// Simplified: R-loss = 10 * lossPct (approx 1 R-point per 1% loss up to moderate rates)
	lossPenalty := lossPct * 1.5

	// Jitter impairment (simplified delay factor)
	jitterPenalty := 0.0
	if jitterMS > 30 {
		jitterPenalty = (jitterMS - 30) * 0.1
	}

	// Delay penalty
	delayPenalty := 0.0
	if delayMS != nil && *delayMS > 150 {
		delayPenalty = (*delayMS - 150) * 0.05
	}

	effectiveR := baseR - lossPenalty - jitterPenalty - delayPenalty
	if effectiveR < 0 {
		effectiveR = 0
	}
	if effectiveR > 100 {
		effectiveR = 100
	}

	// R-factor to MOS mapping (ITU-T G.107)
	if effectiveR <= 0 {
		return 1.0
	}
	if effectiveR >= 100 {
		return 4.5
	}
	mos := 1.0 + 0.035*effectiveR + effectiveR*(effectiveR-60)*(100-effectiveR)*7e-6
	if mos < 1.0 {
		return 1.0
	}
	if mos > 4.5 {
		return 4.5
	}
	return float64(int(mos*100)) / 100 // round to 2 decimal places
}

// ParseRTPFromPayload attempts to parse an RTP header from a UDP payload.
// Returns nil if it does not look like RTP.
func ParseRTPFromPayload(payload []byte) *RTPPacketInfo {
	if len(payload) < 12 {
		return nil
	}
	// RTP header: V(2) | P(1) | X(1) | CC(4) | M(1) | PT(7) | seq(16) | ts(32) | ssrc(32)
	version := (payload[0] >> 6) & 0x03
	if version != 2 {
		return nil
	}
	pt := payload[1] & 0x7F
	// Typical voice codec PTs: 0 (PCMU), 3 (GSM), 8 (PCMA), 9 (G722), 18 (G729)
	// Dynamic RTP types (96-127) also common for VoIP
	if pt < 96 && pt != 0 && pt != 3 && pt != 8 && pt != 9 && pt != 18 {
		// Check for common dynamic voice range, otherwise not likely voice
		// Still allow 96-127 and common static types
	}

	seq := uint16(payload[2])<<8 | uint16(payload[3])
	ts := uint32(payload[4])<<24 | uint32(payload[5])<<16 | uint32(payload[6])<<8 | uint32(payload[7])
	ssrc := uint32(payload[8])<<24 | uint32(payload[9])<<16 | uint32(payload[10])<<8 | uint32(payload[11])

	codec := codecForPT(pt)

	return &RTPPacketInfo{
		SSRC:  ssrc,
		SeqNum: seq,
		RTPTS:  ts,
		PT:     pt,
		Codec:  codec,
		Size:   len(payload),
	}
}

func codecForPT(pt uint8) string {
	switch pt {
	case 0:
		return "PCMU"
	case 3:
		return "GSM"
	case 8:
		return "PCMA"
	case 9:
		return "G722"
	case 18:
		return "G729"
	default:
		if pt >= 96 && pt <= 127 {
			return "Opus" // most common dynamic PT
		}
		return ""
	}
}
