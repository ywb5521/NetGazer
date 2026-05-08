package alerting

type AlertThresholds struct {
	BannedPorts          []uint16 `json:"banned_ports"`
	PortScanThreshold    int      `json:"port_scan_threshold"`
	PortScanWindowSec    int      `json:"port_scan_window_sec"`
	FlowFloodThreshold   int      `json:"flow_flood_threshold"`
	AlertCooldownMin     int      `json:"alert_cooldown_min"`
	DNSSuspiciousPorts   []uint16 `json:"dns_suspicious_ports"`
	SuppressedAlertTypes []string `json:"suppressed_alert_types"`
	// Extended behavioral alerts
	DNSExfilQueryMinLen     int      `json:"dns_exfil_query_min_len"`
	DNSExfilMinBytes        uint64   `json:"dns_exfil_min_bytes"`
	ICMPFloodThreshold      int      `json:"icmp_flood_threshold"`
	SYNFloodRatio           float64  `json:"syn_flood_ratio"`
	HorizontalScanThreshold int      `json:"horizontal_scan_threshold"`
	DataExfilRatio          float64  `json:"data_exfil_ratio"`
	UnexpectedProtocols     []string `json:"unexpected_protocols"`
	ARPSpoofThreshold       int      `json:"arp_spoof_threshold"`
	LongFlowSeconds         int      `json:"long_flow_seconds"`
}

func DefaultThresholds() AlertThresholds {
	return AlertThresholds{
		BannedPorts:             []uint16{23, 3389, 445, 135, 139},
		PortScanThreshold:       20,
		PortScanWindowSec:       60,
		FlowFloodThreshold:      100,
		AlertCooldownMin:        5,
		DNSSuspiciousPorts:      nil,
		DNSExfilQueryMinLen:     52,
		DNSExfilMinBytes:        100000,
		ICMPFloodThreshold:      50,
		SYNFloodRatio:           0.8,
		HorizontalScanThreshold: 20,
		DataExfilRatio:          10.0,
		UnexpectedProtocols:     nil,
		ARPSpoofThreshold:       2,
		LongFlowSeconds:         3600,
	}
}
