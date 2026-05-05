package collector

import (
	"encoding/binary"
	"fmt"
	"net"
)

// sFlow v5 packet constants
const sflowHeaderSize = 28

// sFlowRecord represents a single flow or counter record.
type SFlowRecord struct {
	// Flow sample fields
	SrcIP      net.IP
	DstIP      net.IP
	SrcPort    uint16
	DstPort    uint16
	Protocol   uint8
	TCPFlags   uint8
	Bytes      uint64
	Packets    uint64
	InputSNMP  uint32
	OutputSNMP uint32
	Sampling   uint32
}

// SFlowHeader is the sFlow datagram header.
type SFlowHeader struct {
	Version    uint32
	IPVersion  uint32
	AgentIP    net.IP
	SubAgentID uint32
	Sequence   uint32
	Uptime     uint32
}

func parseSFlow(data []byte, srcIP net.IP) ([]SFlowRecord, error) {
	if len(data) < sflowHeaderSize {
		return nil, fmt.Errorf("sflow: packet too short (%d bytes)", len(data))
	}

	version := binary.BigEndian.Uint32(data[0:4])
	if version != 5 && version != 4 {
		return nil, fmt.Errorf("sflow: unsupported version %d", version)
	}

	ipVer := binary.BigEndian.Uint32(data[4:8])

	var agentIP net.IP
	var agentLen int
	switch ipVer {
	case 1: // IPv4
		agentIP = net.IP(data[8:12]).To4()
		agentLen = 4
	case 2: // IPv6
		agentIP = net.IP(data[8:24])
		agentLen = 16
	default:
		return nil, fmt.Errorf("sflow: unknown IP version %d", ipVer)
	}

	hdr := SFlowHeader{
		Version:    version,
		IPVersion:  ipVer,
		AgentIP:    agentIP,
		SubAgentID: binary.BigEndian.Uint32(data[8+agentLen : 12+agentLen]),
		Sequence:   binary.BigEndian.Uint32(data[12+agentLen : 16+agentLen]),
		Uptime:     binary.BigEndian.Uint32(data[16+agentLen : 20+agentLen]),
	}

	numSamples := binary.BigEndian.Uint32(data[20+agentLen : 24+agentLen])
	offset := sflowHeaderSize - 8 + agentLen // adjust for variable agent IP
	_ = numSamples
	_ = hdr

	var records []SFlowRecord
	for offset+8 <= len(data) {
		entFormat := binary.BigEndian.Uint32(data[offset : offset+4])
		sampleType := entFormat & 0xFFF
		sampleLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))

		if sampleLen < 8 || offset+8+sampleLen > len(data) {
			break
		}

		sampleData := data[offset+8 : offset+8+sampleLen]

		switch sampleType {
		case 1: // Flow sample
			recs := parseSFlowFlowSample(sampleData, agentIP)
			records = append(records, recs...)
		case 2: // Counter sample — skip for now
		}

		offset += 8 + sampleLen
	}

	_ = srcIP
	return records, nil
}

func parseSFlowFlowSample(data []byte, agentIP net.IP) []SFlowRecord {
	if len(data) < 20 {
		return nil
	}

	// Header: seq(4), source_id_type(1), source_id_index(3), sampling_rate(4), sample_pool(4),
	//          drops(4), input(4), output(4), num_records(4)
	samplingRate := binary.BigEndian.Uint32(data[8:12])
	inputSNMP := binary.BigEndian.Uint32(data[20:24])
	outputSNMP := binary.BigEndian.Uint32(data[24:28])
	numRecs := int(binary.BigEndian.Uint32(data[28:32]))

	if samplingRate == 0 {
		samplingRate = 1
	}

	offset := 32
	var records []SFlowRecord

	for i := 0; i < numRecs && offset+8 <= len(data); i++ {
		recEnt := binary.BigEndian.Uint32(data[offset : offset+4])
		recType := recEnt & 0xFFF
		recLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))

		if recLen < 4 || offset+8+recLen > len(data) {
			break
		}

		recData := data[offset+8 : offset+8+recLen]

		switch recType {
		case 1: // Raw packet header
			r := parseSFlowRawHeader(recData, inputSNMP, outputSNMP, samplingRate)
			records = append(records, r)
		}

		offset += 8 + recLen
	}

	return records
}

func parseSFlowRawHeader(data []byte, input, output, sampling uint32) SFlowRecord {
	r := SFlowRecord{
		InputSNMP:  input,
		OutputSNMP: output,
		Sampling:   sampling,
	}

	if len(data) < 16 {
		return r
	}

	headerProto := binary.BigEndian.Uint32(data[0:4])
	frameLen := binary.BigEndian.Uint32(data[4:8])
	stripped := int(binary.BigEndian.Uint32(data[8:12]))
	headerSize := int(binary.BigEndian.Uint32(data[12:16]))

	_ = frameLen
	_ = stripped

	headerData := data[16:]
	if len(headerData) < headerSize {
		headerSize = len(headerData)
	}

	if headerProto == 1 && headerSize >= 20 { // Ethernet → IP
		ipData := headerData[:headerSize]
		// Find IP header (skip MACs + EtherType=14 bytes if present)
		r.Bytes = uint64(sampling) * uint64(frameLen) // approximate
		r.Packets = uint64(sampling)
		_ = ipData
	}

	return r
}
