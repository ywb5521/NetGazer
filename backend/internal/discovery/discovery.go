package discovery

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gtopng/backend/internal/aggregator"
	"github.com/gtopng/backend/internal/models"
)

// DiscoveredHost represents a host found during network discovery.
type DiscoveredHost struct {
	IP       net.IP
	MAC      net.HardwareAddr
	Hostname string
	Method   string // "arp", "ping", "passive"
}

// Config holds discovery scanner configuration.
type Config struct {
	Subnets       []string      // CIDR subnets to scan (e.g. "192.168.1.0/24")
	Interval      time.Duration // scan interval (0 = run once)
	PingTimeout   time.Duration
	ARPTimeout    time.Duration
	PingCount     int
	PingParallel  int
	Interface     string // specific interface to use, empty = auto-detect
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Interval:     5 * time.Minute,
		PingTimeout:  1 * time.Second,
		ARPTimeout:   2 * time.Second,
		PingCount:    2,
		PingParallel: 50,
	}
}

// Scanner performs network discovery and reports findings to the aggregator.
type Scanner struct {
	cfg  Config
	agg  *aggregator.Aggregator
	mu   sync.Mutex
	running bool
}

// NewScanner creates a new discovery scanner.
func NewScanner(cfg Config, agg *aggregator.Aggregator) *Scanner {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = 1 * time.Second
	}
	if cfg.ARPTimeout == 0 {
		cfg.ARPTimeout = 2 * time.Second
	}
	if cfg.PingCount == 0 {
		cfg.PingCount = 2
	}
	if cfg.PingParallel == 0 {
		cfg.PingParallel = 50
	}
	return &Scanner{cfg: cfg, agg: agg}
}

// Start begins periodic network discovery.
func (s *Scanner) Start(ctx context.Context) {
	if len(s.cfg.Subnets) == 0 {
		log.Printf("[discovery] no subnets configured, discovery disabled")
		return
	}

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	log.Printf("[discovery] starting scanner for %d subnets, interval=%s", len(s.cfg.Subnets), s.cfg.Interval)

	// Run immediately on start
	s.runScan()

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.runScan()
		}
	}
}

// ScanNow triggers an immediate scan.
func (s *Scanner) ScanNow() []DiscoveredHost {
	return s.runScan()
}

func (s *Scanner) runScan() []DiscoveredHost {
	var allHosts []DiscoveredHost

	for _, subnet := range s.cfg.Subnets {
		_, ipNet, err := net.ParseCIDR(subnet)
		if err != nil {
			log.Printf("[discovery] invalid subnet %s: %v", subnet, err)
			continue
		}

		// Determine the interface to use
		iface := s.selectInterface(ipNet)

		// ARP scan (fast, finds all hosts on local subnet)
		if iface != nil {
			arpHosts := scanARP(iface, ipNet, s.cfg.ARPTimeout)
			log.Printf("[discovery] ARP scan on %s: found %d hosts", subnet, len(arpHosts))
			allHosts = append(allHosts, arpHosts...)
		}

		// Ping sweep (slower, works across subnets)
		pingHosts := pingSweep(ipNet, s.cfg.PingCount, s.cfg.PingTimeout, s.cfg.PingParallel)
		log.Printf("[discovery] ping sweep on %s: found %d hosts", subnet, len(pingHosts))
		allHosts = append(allHosts, pingHosts...)
	}

	// Deduplicate by IP (keep first found, preferring ARP over ping)
	seen := make(map[string]bool)
	var unique []DiscoveredHost
	for _, h := range allHosts {
		key := h.IP.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, h)
	}

	// Passive discovery: extract hosts from aggregator data
	passiveHosts := s.passiveDiscover()
	log.Printf("[discovery] passive discovery: found %d hosts from existing data", len(passiveHosts))

	// Deduplicate passive hosts against active scan results
	for _, h := range passiveHosts {
		key := h.IP.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, h)
	}

	// Report to aggregator
	if len(unique) > 0 {
		s.reportToAggregator(unique)
	}

	return unique
}

// passiveDiscover extracts host data from all aggregator nodes and creates
// discovery entries for hosts not yet in the discovery node.
func (s *Scanner) passiveDiscover() []DiscoveredHost {
	snapshot := s.agg.GlobalSnapshot()
	var hosts []DiscoveredHost

	for _, h := range snapshot.Hosts {
		ip := net.ParseIP(h.IP)
		if ip == nil || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
			continue
		}
		mac, _ := net.ParseMAC(h.MAC)
		hosts = append(hosts, DiscoveredHost{
			IP:       ip,
			MAC:      mac,
			Hostname: h.Hostname,
			Method:   "passive",
		})
	}
	return hosts
}

func (s *Scanner) selectInterface(ipNet *net.IPNet) *net.Interface {
	if s.cfg.Interface != "" {
		if iface, err := net.InterfaceByName(s.cfg.Interface); err == nil {
			return iface
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.Contains(ipNet.IP) || ipNet.Contains(ipnet.IP) {
					return &iface
				}
			}
		}
	}

	// Fallback: return first non-loopback up interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			return &iface
		}
	}
	return nil
}

func (s *Scanner) reportToAggregator(hosts []DiscoveredHost) {
	now := time.Now()
	for _, h := range hosts {
		macStr := ""
		if h.MAC != nil {
			macStr = h.MAC.String()
		}
		host := &models.Host{
			IP:          h.IP.String(),
			MAC:         macStr,
			Hostname:    h.Hostname,
			FirstSeen:   now,
			LastSeen:    now,
			NodeID:      "discovery",
			ActiveFlows: 0,
		}
		s.agg.UpsertHost(host)
	}
}
