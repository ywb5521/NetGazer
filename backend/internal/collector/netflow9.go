package collector

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	nf9HeaderSize = 20
	// NetFlow v9 field type IDs
	nf9InBytes  = 1
	nf9InPkts   = 2
	nf9Protocol = 4
	nf9SrcPort  = 7
	nf9SrcIPv4  = 8
	nf9SrcMask  = 9
	nf9InSnmp   = 10
	nf9DstPort  = 11
	nf9DstIPv4  = 12
	nf9DstMask  = 13
	nf9OutSnmp  = 14
	nf9NextHop4 = 15
	nf9SrcAS    = 16
	nf9DstAS    = 17
	nf9LastSw   = 21
	nf9FirstSw  = 22
	nf9IPv6Src  = 27
	nf9IPv6Dst  = 28
	nf9IPv6NHop = 62
	nf9TCPFlags = 6
)

type nf9Template struct {
	id     uint16
	fields []nf9Field
}

type nf9Field struct {
	typ    uint16
	length uint16
}

type NetFlow9Record struct {
	SrcIP    net.IP
	DstIP    net.IP
	NextHop  net.IP
	SrcPort  uint16
	DstPort  uint16
	Protocol uint8
	TCPFlags uint8
	Bytes    uint64
	Packets  uint64
	First    time.Time
	Last     time.Time
	SrcAS    uint16
	DstAS    uint16
}

type NetFlow9Header struct {
	Version  uint16
	Count    uint16
	SysTime  uint32
	UnixSecs uint32
	Sequence uint32
	SourceID uint32
}

func parseNetFlow9(data []byte, srcIP net.IP) ([]NetFlow9Record, []nf9Template, error) {
	if len(data) < nf9HeaderSize {
		return nil, nil, fmt.Errorf("netflow9: packet too short (%d bytes)", len(data))
	}

	version := binary.BigEndian.Uint16(data[0:2])
	if version != 9 {
		return nil, nil, fmt.Errorf("netflow9: unexpected version %d", version)
	}

	count := binary.BigEndian.Uint16(data[2:4])
	hdr := NetFlow9Header{
		Version:  version,
		Count:    count,
		SysTime:  binary.BigEndian.Uint32(data[4:8]),
		UnixSecs: binary.BigEndian.Uint32(data[8:12]),
		Sequence: binary.BigEndian.Uint32(data[12:16]),
		SourceID: binary.BigEndian.Uint32(data[16:20]),
	}

	offset := nf9HeaderSize
	var records []NetFlow9Record
	var templates []nf9Template

	for offset < len(data) {
		if offset+4 > len(data) {
			break
		}
		setID := binary.BigEndian.Uint16(data[offset : offset+2])
		setLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if setLen < 4 || offset+setLen > len(data) {
			break
		}

		if setID == 0 {
			// Template flowset
			tmpls := parseTemplates(data[offset+4:offset+setLen], hdr.SourceID)
			templates = append(templates, tmpls...)
		} else if setID > 255 {
			// Data flowset
			recs := parseDataFlowset(data[offset+4:offset+setLen], setID, templates)
			records = append(records, recs...)
		}

		offset += setLen
	}

	_ = srcIP
	return records, templates, nil
}

func parseTemplates(data []byte, sourceID uint32) []nf9Template {
	var templates []nf9Template
	offset := 0
	for offset+4 <= len(data) {
		tmplID := binary.BigEndian.Uint16(data[offset : offset+2])
		fieldCount := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		offset += 4

		if tmplID > 255 {
			// Options template — skip
			scopes := binary.BigEndian.Uint16(data[offset : offset+2])
			offset += 2
			fieldCount -= scopes
		}

		t := nf9Template{id: tmplID}
		for j := uint16(0); j < fieldCount; j++ {
			if offset+4 > len(data) {
				break
			}
			t.fields = append(t.fields, nf9Field{
				typ:    binary.BigEndian.Uint16(data[offset : offset+2]),
				length: binary.BigEndian.Uint16(data[offset+2 : offset+4]),
			})
			offset += 4
		}
		templates = append(templates, t)
	}
	return templates
}

func parseDataFlowset(data []byte, tmplID uint16, templates []nf9Template) []NetFlow9Record {
	var tmpl *nf9Template
	for i := range templates {
		if templates[i].id == tmplID {
			tmpl = &templates[i]
			break
		}
	}
	if tmpl == nil {
		return nil
	}

	// Calculate record size from template
	var recSize uint16
	for _, f := range tmpl.fields {
		if f.length == 65535 { // variable length, use 4 as default
			recSize += 4
		} else {
			recSize += f.length
		}
	}
	if recSize == 0 {
		return nil
	}

	var records []NetFlow9Record
	offset := 0
	for offset+int(recSize) <= len(data) {
		rec := data[offset : offset+int(recSize)]
		r := NetFlow9Record{}
		fieldOff := uint16(0)

		for _, f := range tmpl.fields {
			flen := f.length
			if flen == 65535 {
				flen = 4
			}
			if fieldOff+flen > uint16(len(rec)) {
				break
			}
			val := readField(rec[fieldOff:], flen)

			switch f.typ {
			case nf9SrcIPv4:
				r.SrcIP = val.(net.IP)
			case nf9DstIPv4:
				r.DstIP = val.(net.IP)
			case nf9NextHop4:
				r.NextHop = val.(net.IP)
			case nf9InBytes:
				r.Bytes = val.(uint64)
			case nf9InPkts:
				r.Packets = val.(uint64)
			case nf9Protocol:
				r.Protocol = val.(uint8)
			case nf9SrcPort:
				r.SrcPort = val.(uint16)
			case nf9DstPort:
				r.DstPort = val.(uint16)
			case nf9TCPFlags:
				r.TCPFlags = val.(uint8)
			case nf9SrcAS:
				r.SrcAS = val.(uint16)
			case nf9DstAS:
				r.DstAS = val.(uint16)
			}
			fieldOff += flen
		}
		records = append(records, r)
		offset += int(recSize)
	}
	return records
}

func readField(data []byte, length uint16) interface{} {
	switch length {
	case 1:
		return data[0]
	case 2:
		return binary.BigEndian.Uint16(data)
	case 3:
		return uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
	case 4:
		return net.IP(data).To4()
	case 8:
		return binary.BigEndian.Uint64(data)
	case 16:
		return net.IP(data)
	default:
		return binary.BigEndian.Uint64(data[len(data)-8:])
	}
}
