package capture

import (
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// DecapsulatedPacket holds inner packets extracted from tunnel protocols.
type DecapsulatedPacket struct {
	InnerIP   net.IP
	InnerData []byte
	TunnelType string // "GRE", "GTP-U"
}

// DecapsulateTunnels checks a packet for GRE or GTP-U encapsulation and
// extracts inner IP packets. Returns nil if no tunnel is detected.
func DecapsulateTunnels(packet gopacket.Packet) []DecapsulatedPacket {
	var results []DecapsulatedPacket

	// Check for GRE (IP protocol 47)
	if ip4Layer := packet.Layer(layers.LayerTypeIPv4); ip4Layer != nil {
		ip4, _ := ip4Layer.(*layers.IPv4)
		if ip4.Protocol == layers.IPProtocolGRE {
			gre := extractGRE(ip4.Payload)
			if gre != nil {
				results = append(results, *gre)
			}
		}
	}

	// Check for GTP-U (UDP port 2152)
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		if udp.DstPort == 2152 || udp.SrcPort == 2152 {
			gtp := extractGTPU(udp.Payload)
			if gtp != nil {
				results = append(results, *gtp)
			}
		}
	}

	return results
}

func extractGRE(payload []byte) *DecapsulatedPacket {
	if len(payload) < 4 {
		return nil
	}
	// GRE header: flags(2) + protocol(2), minimum 4 bytes
	flags := (uint16(payload[0]) << 8) | uint16(payload[1])
	proto := (uint16(payload[2]) << 8) | uint16(payload[3])

	hasChecksum := flags&0x8000 != 0
	hasKey := flags&0x2000 != 0
	hasSeq := flags&0x1000 != 0

	offset := 4
	if hasChecksum {
		offset += 4
	}
	if hasKey {
		offset += 4
	}
	if hasSeq {
		offset += 4
	}

	if offset >= len(payload) {
		return nil
	}

	// Protocol 0x0800 = IPv4, 0x86DD = IPv6
	innerData := payload[offset:]
	if proto == 0x0800 && len(innerData) >= 20 {
		innerIP := net.IPv4(innerData[12], innerData[13], innerData[14], innerData[15])
		return &DecapsulatedPacket{InnerIP: innerIP, InnerData: innerData, TunnelType: "GRE"}
	}
	return nil
}

func extractGTPU(payload []byte) *DecapsulatedPacket {
	// GTP-U header: 8 bytes minimum (flags + message type + length + TEID)
	if len(payload) < 8 {
		return nil
	}
	// Check version (first nibble = 1 for GTPv1)
	version := payload[0] >> 5
	if version != 1 {
		return nil
	}
	msgType := payload[1]
	if msgType != 0xFF { // 0xFF = G-PDU (user data)
		return nil
	}
	// GTP-U header is variable length; minimum 8 bytes
	offset := 8
	// Check for extension header
	if payload[0]&0x04 != 0 {
		if offset+1 >= len(payload) {
			return nil
		}
		extLen := int(payload[offset])
		offset += 1 + extLen
	}
	if offset >= len(payload) {
		return nil
	}
	innerData := payload[offset:]
	if len(innerData) >= 20 && (innerData[0]>>4) == 4 {
		innerIP := net.IPv4(innerData[12], innerData[13], innerData[14], innerData[15])
		return &DecapsulatedPacket{InnerIP: innerIP, InnerData: innerData, TunnelType: "GTP-U"}
	}
	return nil
}
