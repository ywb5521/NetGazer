package collector

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// NetFlow v5 packet format constants
const (
	nf5HeaderSize = 24
	nf5RecSize    = 48
	nf5MaxRecs    = 30
)

// NetFlow5Record represents a single NetFlow v5 flow record.
type NetFlow5Record struct {
	SrcAddr   net.IP
	DstAddr   net.IP
	NextHop   net.IP
	Input     uint16
	Output    uint16
	Packets   uint32
	Bytes     uint32
	FirstSeen time.Time
	LastSeen  time.Time
	SrcPort   uint16
	DstPort   uint16
	TCPFlags  uint8
	Protocol  uint8
	TOS       uint8
	SrcAS     uint16
	DstAS     uint16
	SrcMask   uint8
	DstMask   uint8
}

// NetFlow5Header represents the NetFlow v5 packet header.
type NetFlow5Header struct {
	Version    uint16
	Count      uint16
	SysUptime  uint32
	UnixSecs   uint32
	UnixNsecs  uint32
	Sequence   uint32
	EngineType uint8
	EngineID   uint8
	Sampling   uint16
}

func parseNetFlow5(data []byte, srcIP net.IP) ([]NetFlow5Record, error) {
	if len(data) < nf5HeaderSize {
		return nil, fmt.Errorf("netflow5: packet too short (%d bytes)", len(data))
	}

	version := binary.BigEndian.Uint16(data[0:2])
	if version != 5 {
		return nil, fmt.Errorf("netflow5: unexpected version %d", version)
	}

	count := binary.BigEndian.Uint16(data[2:4])
	if count == 0 || count > nf5MaxRecs {
		return nil, fmt.Errorf("netflow5: invalid record count %d", count)
	}

	expectedLen := nf5HeaderSize + int(count)*nf5RecSize
	if len(data) < expectedLen {
		return nil, fmt.Errorf("netflow5: packet truncated (need %d, got %d)", expectedLen, len(data))
	}

	hdr := NetFlow5Header{
		Version:    version,
		Count:      count,
		SysUptime:  binary.BigEndian.Uint32(data[4:8]),
		UnixSecs:   binary.BigEndian.Uint32(data[8:12]),
		UnixNsecs:  binary.BigEndian.Uint32(data[12:16]),
		Sequence:   binary.BigEndian.Uint32(data[16:20]),
		EngineType: data[20],
		EngineID:   data[21],
		Sampling:   binary.BigEndian.Uint16(data[22:24]),
	}

	packetTime := time.Unix(int64(hdr.UnixSecs), int64(hdr.UnixNsecs))

	records := make([]NetFlow5Record, 0, count)
	for i := uint16(0); i < count; i++ {
		off := nf5HeaderSize + int(i)*nf5RecSize
		rec := data[off : off+nf5RecSize]

		sysUptimeMs := binary.BigEndian.Uint32(rec[32:36]) // first
		firstSeen := packetTime.Add(-time.Duration(hdr.SysUptime-sysUptimeMs) * time.Millisecond)

		sysUptimeMsEnd := binary.BigEndian.Uint32(rec[36:40]) // last
		lastSeen := packetTime.Add(-time.Duration(hdr.SysUptime-sysUptimeMsEnd) * time.Millisecond)

		r := NetFlow5Record{
			SrcAddr:   net.IP(rec[0:4]).To4(),
			DstAddr:   net.IP(rec[4:8]).To4(),
			NextHop:   net.IP(rec[8:12]).To4(),
			Input:     binary.BigEndian.Uint16(rec[12:14]),
			Output:    binary.BigEndian.Uint16(rec[14:16]),
			Packets:   binary.BigEndian.Uint32(rec[16:20]),
			Bytes:     binary.BigEndian.Uint32(rec[20:24]),
			FirstSeen: firstSeen,
			LastSeen:  lastSeen,
			SrcPort:   binary.BigEndian.Uint16(rec[32:34]),
			DstPort:   binary.BigEndian.Uint16(rec[34:36]),
			TCPFlags:  rec[37],
			Protocol:  rec[38],
			TOS:       rec[39],
			SrcAS:     binary.BigEndian.Uint16(rec[40:42]),
			DstAS:     binary.BigEndian.Uint16(rec[42:44]),
			SrcMask:   rec[44],
			DstMask:   rec[45],
		}
		records = append(records, r)
	}

	return records, nil
}

func protoName(p uint8) string {
	switch p {
	case 1:
		return "ICMP"
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	case 47:
		return "GRE"
	case 50:
		return "ESP"
	default:
		return fmt.Sprintf("IP/%d", p)
	}
}
