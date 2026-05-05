package models

import "time"

type AlertType string

const (
	AlertHighBandwidth        AlertType = "high_bandwidth"
	AlertNewDevice            AlertType = "new_device"
	AlertSuspiciousPort       AlertType = "suspicious_port"
	AlertFlowFlood            AlertType = "flow_flood"
	AlertPortScan             AlertType = "port_scan"
	AlertDNSSuspiciousPort    AlertType = "dns_suspicious_port"
	AlertDNSExfiltration      AlertType = "dns_exfiltration"
	AlertICMPFlood            AlertType = "icmp_flood"
	AlertSYNFlood             AlertType = "syn_flood"
	AlertHorizontalScan       AlertType = "horizontal_scan"
	AlertDataExfiltration     AlertType = "data_exfiltration"
	AlertUnexpectedProtocol   AlertType = "unexpected_protocol"
	AlertARPSpoof             AlertType = "arp_spoof"
	AlertLongFlow             AlertType = "long_flow"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Alert struct {
	ID           string    `json:"id"`
	Type         AlertType `json:"type"`
	Severity     Severity  `json:"severity"`
	Message      string    `json:"message"`
	SourceIP     string    `json:"source_ip,omitempty"`
	NodeID       string    `json:"node_id"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

type AlertJSON struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	SourceIP     string `json:"source_ip,omitempty"`
	NodeID       string `json:"node_id"`
	Timestamp    int64  `json:"timestamp"`
	Acknowledged bool   `json:"acknowledged"`
}

func (a *Alert) ToJSON() AlertJSON {
	return AlertJSON{
		ID:           a.ID,
		Type:         string(a.Type),
		Severity:     string(a.Severity),
		Message:      a.Message,
		SourceIP:     a.SourceIP,
		NodeID:       a.NodeID,
		Timestamp:    a.Timestamp.UnixMilli(),
		Acknowledged: a.Acknowledged,
	}
}
