package tracker

import (
	"sync"

	"github.com/gtopng/backend/internal/capture"
)

type PacketSizeDist struct {
	Size64    uint64
	Size128   uint64
	Size256   uint64
	Size512   uint64
	Size1024  uint64
	Size1500  uint64
	SizeGt1500 uint64
}

type PacketSizeTracker struct {
	mu   sync.Mutex
	dist PacketSizeDist
}

func NewPacketSizeTracker() *PacketSizeTracker {
	return &PacketSizeTracker{}
}

func (t *PacketSizeTracker) Process(p capture.ParsedPacket, nodeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch {
	case p.Length <= 64:
		t.dist.Size64++
	case p.Length <= 128:
		t.dist.Size128++
	case p.Length <= 256:
		t.dist.Size256++
	case p.Length <= 512:
		t.dist.Size512++
	case p.Length <= 1024:
		t.dist.Size1024++
	case p.Length <= 1500:
		t.dist.Size1500++
	default:
		t.dist.SizeGt1500++
	}
}

func (t *PacketSizeTracker) Snapshot() PacketSizeDist {
	t.mu.Lock()
	defer t.mu.Unlock()
	d := t.dist
	t.dist = PacketSizeDist{}
	return d
}
