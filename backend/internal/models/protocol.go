package models

type ProtocolStat struct {
	Protocol   string  `json:"protocol"`
	Bytes      uint64  `json:"bytes"`
	Packets    uint64  `json:"packets"`
	Percentage float64 `json:"percentage"`
	NodeID     string  `json:"node_id"`
	Interface  string  `json:"interface"`
}
