package models

import "time"

type Flow struct {
	ID          string    `json:"id"`
	SrcIP       string    `json:"src_ip"`
	DstIP       string    `json:"dst_ip"`
	SrcPort     uint16    `json:"src_port"`
	DstPort     uint16    `json:"dst_port"`
	Protocol    string    `json:"protocol"`
	AppProtocol string    `json:"app_protocol"`
	Bytes       uint64    `json:"bytes"`
	Packets     uint64    `json:"packets"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	NodeID      string    `json:"node_id"`
	Interface   string    `json:"interface"`
	VlanID      uint16    `json:"vlan_id"`
}

type FlowJSON struct {
	ID          string `json:"id"`
	SrcIP       string `json:"src_ip"`
	DstIP       string `json:"dst_ip"`
	SrcPort     uint16 `json:"src_port"`
	DstPort     uint16 `json:"dst_port"`
	Protocol    string `json:"protocol"`
	AppProtocol string `json:"app_protocol"`
	Bytes       uint64 `json:"bytes"`
	Packets     uint64 `json:"packets"`
	FirstSeen   int64  `json:"first_seen"`
	LastSeen    int64  `json:"last_seen"`
	NodeID      string `json:"node_id"`
	Interface   string `json:"interface"`
	VlanID      uint16 `json:"vlan_id"`
}

func (f *Flow) ToJSON() FlowJSON {
	return FlowJSON{
		ID:          f.ID,
		SrcIP:       f.SrcIP,
		DstIP:       f.DstIP,
		SrcPort:     f.SrcPort,
		DstPort:     f.DstPort,
		Protocol:    f.Protocol,
		AppProtocol: f.AppProtocol,
		Bytes:       f.Bytes,
		Packets:     f.Packets,
		FirstSeen:   f.FirstSeen.UnixMilli(),
		LastSeen:    f.LastSeen.UnixMilli(),
		NodeID:      f.NodeID,
		Interface:   f.Interface,
		VlanID:      f.VlanID,
	}
}

type HostPeer struct {
	PeerIP    string `json:"peer_ip"`
	Bytes     uint64 `json:"bytes"`
	Packets   uint64 `json:"packets"`
	FlowCount int    `json:"flow_count"`
}

type TrafficMatrixCell struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Bytes       uint64 `json:"bytes"`
}
