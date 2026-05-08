package models

import "time"

type TrafficSnapshot struct {
	Timestamp     time.Time `json:"timestamp"`
	BytesPerSec   float64   `json:"bytes_per_sec"`
	PacketsPerSec float64   `json:"packets_per_sec"`
	FlowsCount    int       `json:"flows_count"`
	NodeID        string    `json:"node_id"`
}

type TCPMetricsJSON struct {
	ActiveTCPFlows    int     `json:"active_tcp_flows"`
	TotalRetransmits  int64   `json:"total_retransmits"`
	TotalRSTs         int64   `json:"total_rsts"`
	TotalZeroWindows  int64   `json:"total_zero_windows"`
	TotalOutOfOrder   int64   `json:"total_out_of_order"`
	RTTAvgMS          float64 `json:"rtt_avg_ms"`
	RTTMinMS          float64 `json:"rtt_min_ms"`
	RTTMaxMS          float64 `json:"rtt_max_ms"`
	RTTSamples        int64   `json:"rtt_samples"`
	TotalExpectedPkts int64   `json:"total_expected_pkts"`
	TotalLostPkts     int64   `json:"total_lost_pkts"`
	PacketLossPct     float64 `json:"packet_loss_pct"`
}

type SystemHealthJSON struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemPercent     float64 `json:"mem_percent"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	DiskFreeBytes  uint64  `json:"disk_free_bytes"`
	DiskTotalBytes uint64  `json:"disk_total_bytes"`
	UptimeSeconds  uint64  `json:"uptime_seconds"`
}

type InterfaceInfo struct {
	Name          string  `json:"name"`
	BytesPerSec   float64 `json:"bytes_per_sec"`
	PacketsPerSec float64 `json:"packets_per_sec"`
	HostsCount    int     `json:"hosts_count"`
	FlowsCount    int     `json:"flows_count"`
}

type NodeInfo struct {
	NodeID        string            `json:"node_id"`
	Interface     string            `json:"interface"`
	Interfaces    []string          `json:"interfaces"`
	InterfaceInfo []InterfaceInfo   `json:"interface_info"`
	Tags          []string          `json:"tags"`
	Online        bool              `json:"online"`
	BytesPerSec   float64           `json:"bytes_per_sec"`
	PacketsPerSec float64           `json:"packets_per_sec"`
	HostsCount    int               `json:"hosts_count"`
	FlowsCount    int               `json:"flows_count"`
	LastSeen      int64             `json:"last_seen"`
	Version       string            `json:"version"`
	SystemHealth  *SystemHealthJSON `json:"system_health,omitempty"`
	TCPMetrics    *TCPMetricsJSON   `json:"tcp_metrics,omitempty"`
	DNSLatency    *LatencyStatsJSON `json:"dns_latency,omitempty"`
	TLSLatency    *LatencyStatsJSON `json:"tls_latency,omitempty"`
	TCPLatency    *LatencyStatsJSON `json:"tcp_latency,omitempty"`
	VOIPStats     *VOIPStatsJSON    `json:"voip_stats,omitempty"`
}

type LatencyStatsJSON struct {
	Samples int     `json:"samples"`
	AvgMS   float64 `json:"avg_ms"`
	MinMS   float64 `json:"min_ms"`
	MaxMS   float64 `json:"max_ms"`
}

type Summary struct {
	HostsCount   int    `json:"hosts_count"`
	ActiveFlows  int    `json:"active_flows"`
	TotalBytes   uint64 `json:"total_bytes"`
	TotalPackets uint64 `json:"total_packets"`
	Uptime       string `json:"uptime"`
	NodesOnline  int    `json:"nodes_online"`
	NodesTotal   int    `json:"nodes_total"`
}

type DNSQueryJSON struct {
	QueryName string `json:"query_name"`
	Count     uint64 `json:"count"`
	Bytes     uint64 `json:"bytes"`
}

type PacketSizeDistJSON struct {
	Size64     uint64 `json:"size_64"`
	Size128    uint64 `json:"size_128"`
	Size256    uint64 `json:"size_256"`
	Size512    uint64 `json:"size_512"`
	Size1024   uint64 `json:"size_1024"`
	Size1500   uint64 `json:"size_1500"`
	SizeGt1500 uint64 `json:"size_gt1500"`
}

type HostSnapshot struct {
	Timestamp       time.Time `json:"timestamp"`
	NodeID          string    `json:"node_id"`
	HostIP          string    `json:"host_ip"`
	BytesSent       float64   `json:"bytes_sent"`
	BytesReceived   float64   `json:"bytes_received"`
	PacketsSent     int       `json:"packets_sent"`
	PacketsReceived int       `json:"packets_received"`
}

type VOIPStatsJSON struct {
	ActiveSessions int     `json:"active_sessions"`
	TotalSessions  int     `json:"total_sessions"`
	TotalPackets   int64   `json:"total_packets"`
	TotalBytes     int64   `json:"total_bytes"`
	TotalLost      int64   `json:"total_lost"`
	AvgJitterMS    float64 `json:"avg_jitter_ms"`
	MinMOS         float64 `json:"min_mos"`
	AvgMOS         float64 `json:"avg_mos"`
}

type GlobalSnapshot struct {
	Nodes          []NodeInfo          `json:"nodes"`
	Hosts          []HostJSON          `json:"hosts"`
	Flows          []FlowJSON          `json:"flows"`
	Protocols      []ProtocolStat      `json:"protocols"`
	Traffic        TrafficSnapshot     `json:"traffic"`
	Alerts         []AlertJSON         `json:"alerts"`
	DnsQueries     []DNSQueryJSON      `json:"dns_queries"`
	PacketSizeDist *PacketSizeDistJSON `json:"packet_size_dist,omitempty"`
}
