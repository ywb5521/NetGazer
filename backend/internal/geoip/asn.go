package geoip

import (
	"net"
	"sync"
)

// ASNInfo holds Autonomous System information for an IP address.
type ASNInfo struct {
	ASNumber uint32 `json:"as_number"`
	ASOrg    string `json:"as_org"`
}

// Built-in ASN mapping for common private/well-known prefixes.
// Production deployments should use a MaxMind GeoLite2 ASN database or similar.
var builtinASN = []struct {
	prefix string // CIDR notation
	asn    uint32
	org    string
}{
	// RFC 1918 private ranges — mapped to local AS
	{"10.0.0.0/8", 0, "Private (RFC 1918)"},
	{"172.16.0.0/12", 0, "Private (RFC 1918)"},
	{"192.168.0.0/16", 0, "Private (RFC 1918)"},
	// Carrier-grade NAT
	{"100.64.0.0/10", 0, "CGNAT (RFC 6598)"},
	// Loopback
	{"127.0.0.0/8", 0, "Loopback"},
	// Link-local
	{"169.254.0.0/16", 0, "Link-Local (RFC 3927)"},
	// Multicast
	{"224.0.0.0/4", 0, "Multicast"},
	// Documentation
	{"192.0.2.0/24", 0, "Documentation (RFC 5737)"},
	{"198.51.100.0/24", 0, "Documentation (RFC 5737)"},
	{"203.0.113.0/24", 0, "Documentation (RFC 5737)"},
	// Benchmarking
	{"198.18.0.0/15", 0, "Benchmarking (RFC 2544)"},
	// Major cloud/ISP /8 blocks (illustrative — real deployment needs full database)
	{"8.0.0.0/8", 3356, "Level 3 Communications"},
	{"17.0.0.0/8", 714, "Apple Inc."},
	{"52.0.0.0/8", 14618, "Amazon Web Services"},
	{"54.0.0.0/8", 14618, "Amazon Web Services"},
	{"35.0.0.0/8", 15169, "Google LLC"},
	{"74.0.0.0/8", 15169, "Google LLC"},
	{"172.217.0.0/16", 15169, "Google LLC"},
	{"142.250.0.0/16", 15169, "Google LLC"},
	{"157.240.0.0/16", 32934, "Facebook/Meta"},
	{"31.13.0.0/16", 32934, "Facebook/Meta"},
	{"13.0.0.0/8", 16509, "Amazon Web Services"},
	{"18.0.0.0/8", 3, "MIT / Amazon"},
	{"3.0.0.0/8", 14618, "Amazon Web Services"},
	{"34.0.0.0/8", 15169, "Google LLC"},
	{"104.16.0.0/12", 13335, "Cloudflare Inc."},
	{"104.0.0.0/8", 13335, "Cloudflare Inc."},
	{"23.0.0.0/8", 20940, "Akamai Technologies"},
	{"96.0.0.0/8", 20940, "Akamai Technologies"},
	{"151.101.0.0/16", 54113, "Fastly Inc."},
}

// CIDR entries from builtinASN, pre-parsed for fast lookup.
var (
	parsedASN    []*asnEntry
	parseASNOnce sync.Once
)

type asnEntry struct {
	network *net.IPNet
	asn     uint32
	org     string
}

func initASN() {
	parsedASN = make([]*asnEntry, 0, len(builtinASN))
	for _, e := range builtinASN {
		_, nw, err := net.ParseCIDR(e.prefix)
		if err != nil {
			continue
		}
		parsedASN = append(parsedASN, &asnEntry{network: nw, asn: e.asn, org: e.org})
	}
}

// LookupASN returns AS information for an IP address using the built-in database.
func LookupASN(ipStr string) *ASNInfo {
	parseASNOnce.Do(initASN)

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil
	}

	for _, e := range parsedASN {
		if e.network.Contains(ip) {
			return &ASNInfo{ASNumber: e.asn, ASOrg: e.org}
		}
	}
	return nil
}

// LookupASNString returns a descriptive AS string like "AS15169 (Google LLC)".
func LookupASNString(ipStr string) string {
	info := LookupASN(ipStr)
	if info == nil {
		return ""
	}
	if info.ASNumber == 0 {
		return info.ASOrg
	}
	return "AS" + itoa64(info.ASNumber) + " (" + info.ASOrg + ")"
}

func itoa64(n uint32) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte(n%10) + '0'}, buf...)
		n /= 10
	}
	return string(buf)
}
