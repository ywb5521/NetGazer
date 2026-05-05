package models

import "time"

type Host struct {
	IP              string    `json:"ip"`
	MAC             string    `json:"mac"`
	Hostname        string    `json:"hostname"`
	BytesSent       uint64    `json:"bytes_sent"`
	BytesReceived   uint64    `json:"bytes_received"`
	PacketsSent     uint64    `json:"packets_sent"`
	PacketsReceived uint64    `json:"packets_received"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	Vendor          string    `json:"vendor"`
	ActiveFlows     int       `json:"active_flows"`
	NodeID          string    `json:"node_id"`
	Interface       string    `json:"interface"`
	Country         string    `json:"country,omitempty"`
	Category        string    `json:"category,omitempty"`
	ASN             string    `json:"asn,omitempty"`
	OSInfo          string    `json:"os_info,omitempty"`
}

type HostJSON struct {
	IP              string `json:"ip"`
	MAC             string `json:"mac"`
	Hostname        string `json:"hostname"`
	BytesSent       uint64 `json:"bytes_sent"`
	BytesReceived   uint64 `json:"bytes_received"`
	PacketsSent     uint64 `json:"packets_sent"`
	PacketsReceived uint64 `json:"packets_received"`
	FirstSeen       int64  `json:"first_seen"`
	LastSeen        int64  `json:"last_seen"`
	Vendor          string `json:"vendor"`
	ActiveFlows     int    `json:"active_flows"`
	NodeID          string `json:"node_id"`
	Interface       string `json:"interface"`
	Country         string `json:"country,omitempty"`
	Category        string `json:"category,omitempty"`
	ASN             string `json:"asn,omitempty"`
	OSInfo          string `json:"os_info,omitempty"`
}

func (h *Host) ToJSON() HostJSON {
	return HostJSON{
		IP:              h.IP,
		MAC:             h.MAC,
		Hostname:        h.Hostname,
		BytesSent:       h.BytesSent,
		BytesReceived:   h.BytesReceived,
		PacketsSent:     h.PacketsSent,
		PacketsReceived: h.PacketsReceived,
		FirstSeen:       h.FirstSeen.UnixMilli(),
		LastSeen:        h.LastSeen.UnixMilli(),
		Vendor:          h.Vendor,
		ActiveFlows:     h.ActiveFlows,
		NodeID:          h.NodeID,
		Interface:       h.Interface,
		Country:         h.Country,
		Category:        h.Category,
		ASN:             h.ASN,
		OSInfo:          h.OSInfo,
	}
}

func HostCategory(ip string) string {
	if ip == "" {
		return ""
	}
	// Parse IP to get the first octet
	var firstOctet int
	for i := 0; i < len(ip); i++ {
		if ip[i] == '.' || ip[i] == ':' {
			break
		}
		if ip[i] >= '0' && ip[i] <= '9' {
			firstOctet = firstOctet*10 + int(ip[i]-'0')
		}
	}

	if firstOctet == 127 {
		return "Localhost"
	}
	if firstOctet == 10 {
		return "Local"
	}
	if firstOctet == 172 {
		// Check second octet for 172.16-31
		parts := splitIPOctets(ip)
		if len(parts) >= 2 {
			second := parts[1]
			if second >= 16 && second <= 31 {
				return "Local"
			}
		}
	}
	if firstOctet == 192 {
		parts := splitIPOctets(ip)
		if len(parts) >= 2 && parts[1] == 168 {
			return "Local"
		}
	}
	if firstOctet >= 224 && firstOctet <= 239 {
		return "Multicast"
	}
	if ip == "255.255.255.255" {
		return "Broadcast"
	}
	if firstOctet == 169 {
		parts := splitIPOctets(ip)
		if len(parts) >= 2 && parts[1] == 254 {
			return "Link-Local"
		}
	}
	return "Remote"
}

func splitIPOctets(ip string) []int {
	parts := make([]int, 0, 4)
	num := 0
	hasDigit := false
	for i := 0; i < len(ip); i++ {
		if ip[i] >= '0' && ip[i] <= '9' {
			num = num*10 + int(ip[i]-'0')
			hasDigit = true
		} else if ip[i] == '.' {
			parts = append(parts, num)
			num = 0
			hasDigit = false
		} else {
			break
		}
	}
	if hasDigit {
		parts = append(parts, num)
	}
	return parts
}
