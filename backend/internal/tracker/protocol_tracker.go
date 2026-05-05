package tracker

import (
	"sync"

	"github.com/gtopng/backend/internal/capture"
	"github.com/gtopng/backend/internal/models"
)

type ProtocolTracker struct {
	mu     sync.RWMutex
	stats  map[string]*models.ProtocolStat
	total  uint64
}

func NewProtocolTracker() *ProtocolTracker {
	return &ProtocolTracker{
		stats: make(map[string]*models.ProtocolStat),
	}
}

func (t *ProtocolTracker) Process(p capture.ParsedPacket, nodeID string) {
	key := p.Protocol + ":" + p.AppProto
	if p.Protocol == p.AppProto {
		key = p.Protocol
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.stats[key]
	if !ok {
		s = &models.ProtocolStat{
			Protocol: key,
			NodeID:   nodeID,
		}
		t.stats[key] = s
	}
	s.Bytes += uint64(p.Length)
	s.Packets++
	t.total += uint64(p.Length)
}

func (t *ProtocolTracker) Snapshot() []models.ProtocolStat {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]models.ProtocolStat, 0, len(t.stats))
	total := float64(t.total)
	for _, s := range t.stats {
		copy := *s
		if total > 0 {
			copy.Percentage = float64(s.Bytes) / total * 100
		}
		result = append(result, copy)
	}
	t.stats = make(map[string]*models.ProtocolStat)
	t.total = 0
	return result
}

func (t *ProtocolTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats = make(map[string]*models.ProtocolStat)
	t.total = 0
}
