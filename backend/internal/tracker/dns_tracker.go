package tracker

import (
	"sync"

	"github.com/netgazer/backend/internal/capture"
)

type dnsEntry struct {
	QueryName string
	Count     uint64
	Bytes     uint64
}

type DNSTracker struct {
	mu      sync.RWMutex
	queries map[string]*dnsEntry
}

func NewDNSTracker() *DNSTracker {
	return &DNSTracker{
		queries: make(map[string]*dnsEntry),
	}
}

func (t *DNSTracker) Process(p capture.ParsedPacket, nodeID string) {
	if p.DNSQuery == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	key := p.DNSQuery
	if entry, ok := t.queries[key]; ok {
		entry.Count++
		entry.Bytes += uint64(p.Length)
	} else {
		t.queries[key] = &dnsEntry{
			QueryName: p.DNSQuery,
			Count:     1,
			Bytes:     uint64(p.Length),
		}
	}
}

// Top returns the top N DNS queries sorted by count descending. Resets stats after each snapshot.
func (t *DNSTracker) Top(n int) []dnsEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]dnsEntry, 0, len(t.queries))
	for _, e := range t.queries {
		result = append(result, *e)
	}
	for i := 0; i < len(result) && i < n; i++ {
		maxIdx := i
		for j := i + 1; j < len(result); j++ {
			if result[j].Count > result[maxIdx].Count {
				maxIdx = j
			}
		}
		result[i], result[maxIdx] = result[maxIdx], result[i]
	}
	if len(result) > n {
		result = result[:n]
	}
	t.queries = make(map[string]*dnsEntry)
	return result
}
