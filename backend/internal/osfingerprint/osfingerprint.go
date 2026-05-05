package osfingerprint

import (
	"strconv"
	"strings"
)

// OSInfo holds detected operating system information.
type OSInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Method  string `json:"method"` // "tcp", "http", "dhcp", "manual"
}

// TCPFingerprint matches TCP SYN packet characteristics against known signatures.
type TCPFingerprint struct {
	TTL        uint8
	WindowSize uint16
	MSS        uint16
	Options    string // compact option string: "M"=MSS, "S"=SACK, "T"=Timestamp, "N"=NOP, "W"=WindowScale
}

// Known TCP/IP stack signatures (TTL, Window, MSS, Options pattern).
var tcpSignatures = []struct {
	OS     string
	TTL    uint8
	Win    uint16
	MSS    uint16
	Opts   string
	Match  string // "exact" or "ttl_win" or "ttl"
}{
	// Linux
	{OS: "Linux", TTL: 64, Win: 29200, MSS: 1460, Opts: "MSTNW", Match: "ttl_win"},
	{OS: "Linux", TTL: 64, Win: 65535, MSS: 1460, Opts: "MSTNW", Match: "ttl_win"},
	{OS: "Linux", TTL: 64, Win: 64240, MSS: 1460, Opts: "MSTNW", Match: "ttl"},
	// Windows
	{OS: "Windows (10/11/Server 2016+)", TTL: 128, Win: 65535, MSS: 1460, Opts: "MSTNW", Match: "ttl_win"},
	{OS: "Windows (XP/2003)", TTL: 128, Win: 65535, MSS: 1460, Opts: "MNSTW", Match: "ttl_win"},
	// macOS / iOS
	{OS: "macOS / iOS", TTL: 64, Win: 65535, MSS: 1460, Opts: "MSTNW", Match: "ttl_win"},
	{OS: "macOS (older)", TTL: 64, Win: 65535, MSS: 1460, Opts: "MNSTW", Match: "ttl"},
	// FreeBSD
	{OS: "FreeBSD", TTL: 64, Win: 65535, MSS: 1460, Opts: "MSTNW", Match: "ttl"},
	// Android
	{OS: "Android", TTL: 64, Win: 65535, MSS: 1460, Opts: "MSTNW", Match: "ttl"},
	// Network devices
	{OS: "Cisco IOS", TTL: 255, Win: 4128, MSS: 536, Opts: "MS", Match: "ttl_win"},
	{OS: "Juniper JunOS", TTL: 64, Win: 65535, MSS: 1460, Opts: "MSTNW", Match: "ttl"},
	{OS: "VMware ESXi", TTL: 64, Win: 29200, MSS: 1460, Opts: "MSTNW", Match: "ttl_win"},
}

// IdentifyByTCP attempts to identify the OS from TCP SYN packet characteristics.
func IdentifyByTCP(ttl uint8, windowSize, mss uint16, opts string) *OSInfo {
	for _, sig := range tcpSignatures {
		switch sig.Match {
		case "exact":
			if sig.TTL == ttl && sig.Win == windowSize && sig.MSS == mss && sig.Opts == opts {
				return &OSInfo{Name: sig.OS, Method: "tcp"}
			}
		case "ttl_win":
			if sig.TTL == ttl && sig.Win == windowSize {
				return &OSInfo{Name: sig.OS, Method: "tcp"}
			}
		case "ttl":
			if sig.TTL == ttl {
				return &OSInfo{Name: sig.OS, Method: "tcp"}
			}
		}
	}
	return nil
}

// IdentifyByUserAgent extracts OS from HTTP User-Agent string.
func IdentifyByUserAgent(ua string) *OSInfo {
	uaLower := strings.ToLower(ua)
	switch {
	case strings.Contains(uaLower, "windows nt 10.0") || strings.Contains(uaLower, "windows nt 10"):
		return &OSInfo{Name: "Windows 10/11", Method: "http"}
	case strings.Contains(uaLower, "windows nt 6.3"):
		return &OSInfo{Name: "Windows 8.1", Method: "http"}
	case strings.Contains(uaLower, "windows nt 6.1"):
		return &OSInfo{Name: "Windows 7", Method: "http"}
	case strings.Contains(uaLower, "mac os x") || strings.Contains(uaLower, "macintosh"):
		return &OSInfo{Name: "macOS", Method: "http"}
	case strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad"):
		return &OSInfo{Name: "iOS", Method: "http"}
	case strings.Contains(uaLower, "android"):
		return &OSInfo{Name: "Android", Method: "http"}
	case strings.Contains(uaLower, "linux") && !strings.Contains(uaLower, "android"):
		return &OSInfo{Name: "Linux", Method: "http"}
	case strings.Contains(uaLower, "crkey") || strings.Contains(uaLower, "chrome"):
		return &OSInfo{Name: "ChromeOS", Method: "http"}
	}
	return nil
}

// IdentifyByDHCP extracts OS from DHCP hostname or vendor class.
func IdentifyByDHCP(hostname, vendorClass string) *OSInfo {
	vcLower := strings.ToLower(vendorClass)
	switch {
	case strings.Contains(vcLower, "msft 5.0"):
		return &OSInfo{Name: "Windows", Method: "dhcp"}
	case strings.Contains(vcLower, "msft"):
		return &OSInfo{Name: "Windows", Method: "dhcp"}
	case strings.Contains(vcLower, "android-dhcp"):
		ver := extractVersion(vendorClass, "android-dhcp-")
		if ver != "" {
			return &OSInfo{Name: "Android", Version: ver, Method: "dhcp"}
		}
		return &OSInfo{Name: "Android", Method: "dhcp"}
	case strings.Contains(vcLower, "dhcpcd"):
		return &OSInfo{Name: "Linux/Android", Method: "dhcp"}
	case strings.Contains(hostname, "iPhone") || strings.Contains(hostname, "iPad"):
		return &OSInfo{Name: "iOS", Method: "dhcp"}
	}
	return nil
}

func extractVersion(s, prefix string) string {
	idx := strings.Index(strings.ToLower(s), prefix)
	if idx < 0 {
		return ""
	}
	s = s[idx+len(prefix):]
	for i, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			return s[:i]
		}
	}
	return s
}

// TTLToOS guesses OS from IP TTL alone (used as fallback).
func TTLToOS(ttl uint8) *OSInfo {
	switch {
	case ttl <= 64:
		return &OSInfo{Name: "Linux/Unix/macOS", Method: "tcp"}
	case ttl <= 128:
		return &OSInfo{Name: "Windows", Method: "tcp"}
	case ttl >= 250:
		return &OSInfo{Name: "Network Device", Method: "tcp"}
	default:
		return nil
	}
}

// CompressOptions creates a compact option string from TCP option kinds.
func CompressOptions(optionKinds []uint8) string {
	var b strings.Builder
	for _, k := range optionKinds {
		switch k {
		case 0:
			// EOL, skip
			return b.String()
		case 1:
			b.WriteByte('N')
		case 2:
			b.WriteByte('M')
		case 3:
			b.WriteByte('W')
		case 4:
			b.WriteByte('S')
		case 5:
			b.WriteByte('S')
		case 8:
			b.WriteByte('T')
		default:
			b.WriteByte('?')
			b.WriteString(strconv.Itoa(int(k)))
		}
	}
	return b.String()
}
