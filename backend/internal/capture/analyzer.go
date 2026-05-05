package capture

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/gtopng/backend/internal/ndpi"
)

type ParsedPacket struct {
	Timestamp   int64
	SrcIP       net.IP
	DstIP       net.IP
	SrcMAC      net.HardwareAddr
	DstMAC      net.HardwareAddr
	SrcPort     uint16
	DstPort     uint16
	Length      int
	Protocol    string
	AppProto    string
	DNSQuery    string
	TLSSNI      string
	HTTPHost    string
	HTTPURL     string
	HTTPStatus  int
	Interface   string
	// TCP-level fields
	TCPSeq    uint32
	TCPAck    uint32
	TCPSYN    bool
	TCPRST    bool
	TCPFIN    bool
	TCPACK    bool
	TCPWindow uint16
	VlanID    uint16
}

type Analyzer struct {
	ndpiEngine    *ndpi.Engine
	ogfwDetector  *OGFWDetector
	protoEngine   string // "ndpi", "opengfw", "both"
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

func NewAnalyzerWithNDPI(engine *ndpi.Engine) *Analyzer {
	return &Analyzer{ndpiEngine: engine, protoEngine: "ndpi"}
}

func NewAnalyzerWithOpenGFW(engine *ndpi.Engine, detector *OGFWDetector, protoEngine string) *Analyzer {
	return &Analyzer{ndpiEngine: engine, ogfwDetector: detector, protoEngine: protoEngine}
}

func (a *Analyzer) Start(ctx context.Context, in <-chan gopacket.Packet) <-chan ParsedPacket {
	out := make(chan ParsedPacket, 4096)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case packet, ok := <-in:
				if !ok {
					return
				}
				parsed := a.parse(packet)
				if parsed.Protocol == "" {
					continue
				}
				select {
				case out <- parsed:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

func (a *Analyzer) parse(packet gopacket.Packet) ParsedPacket {
	p := ParsedPacket{
		Timestamp: packet.Metadata().Timestamp.UnixMilli(),
		Length:    packet.Metadata().Length,
	}

	if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
		eth, _ := ethLayer.(*layers.Ethernet)
		p.SrcMAC = eth.SrcMAC
		p.DstMAC = eth.DstMAC
	}

	if vlanLayer := packet.Layer(layers.LayerTypeDot1Q); vlanLayer != nil {
		dot1q, _ := vlanLayer.(*layers.Dot1Q)
		p.VlanID = dot1q.VLANIdentifier
	}

	if ip4Layer := packet.Layer(layers.LayerTypeIPv4); ip4Layer != nil {
		ip4, _ := ip4Layer.(*layers.IPv4)
		p.SrcIP = ip4.SrcIP
		p.DstIP = ip4.DstIP
		p.Protocol = strings.ToUpper(ip4.Protocol.String())
		return a.parseTransport(p, packet, p.Protocol)
	}

	if ip6Layer := packet.Layer(layers.LayerTypeIPv6); ip6Layer != nil {
		ip6, _ := ip6Layer.(*layers.IPv6)
		p.SrcIP = ip6.SrcIP
		p.DstIP = ip6.DstIP
		return a.parseTransport(p, packet, a.ipv6Proto(ip6))
	}

	if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
		arp, _ := arpLayer.(*layers.ARP)
		p.Protocol = "ARP"
		p.SrcIP = arp.SourceProtAddress
		p.DstIP = arp.DstProtAddress
		if arp.SourceHwAddress != nil {
			p.SrcMAC = arp.SourceHwAddress
		}
		if arp.DstHwAddress != nil {
			p.DstMAC = arp.DstHwAddress
		}
		p.AppProto = "ARP"
		return p
	}

	return p
}

func (a *Analyzer) parseTransport(p ParsedPacket, packet gopacket.Packet, proto string) ParsedPacket {
	p.Protocol = proto

	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		p.SrcPort = uint16(tcp.SrcPort)
		p.DstPort = uint16(tcp.DstPort)
		p.TCPSeq = tcp.Seq
		p.TCPAck = tcp.Ack
		p.TCPSYN = tcp.SYN
		p.TCPRST = tcp.RST
		p.TCPFIN = tcp.FIN
		p.TCPACK = tcp.ACK
		p.TCPWindow = tcp.Window
		p.AppProto = a.classifyApp(p.SrcPort, p.DstPort, tcp.Payload, "TCP", &p, packet)
		if strings.HasPrefix(p.AppProto, "DNS") {
			p.DNSQuery = parseDNSName(tcp.Payload)
		}
		return p
	}

	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		p.SrcPort = uint16(udp.SrcPort)
		p.DstPort = uint16(udp.DstPort)
		p.AppProto = a.classifyApp(p.SrcPort, p.DstPort, udp.Payload, "UDP", &p, packet)
		if strings.HasPrefix(p.AppProto, "DNS") {
			p.DNSQuery = parseDNSName(udp.Payload)
		}
		return p
	}

	if icmpLayer := packet.Layer(layers.LayerTypeICMPv4); icmpLayer != nil {
		_ = icmpLayer
		p.AppProto = "ICMP"
		return p
	}

	if icmp6Layer := packet.Layer(layers.LayerTypeICMPv6); icmp6Layer != nil {
		_ = icmp6Layer
		p.AppProto = "ICMPv6"
		return p
	}

	p.AppProto = "unknown"
	return p
}

func httpEnrich(p *ParsedPacket) string {
	if p.HTTPHost != "" {
		if p.HTTPStatus > 0 {
			return "HTTP (" + p.HTTPHost + ") [" + strconv.Itoa(p.HTTPStatus) + "]"
		}
		return "HTTP (" + p.HTTPHost + ")"
	}
	if p.HTTPStatus > 0 {
		return "HTTP [" + strconv.Itoa(p.HTTPStatus) + "]"
	}
	return "HTTP"
}

func (a *Analyzer) classifyApp(srcPort, dstPort uint16, payload []byte, transport string, p *ParsedPacket, packet gopacket.Packet) string {
	// Try OpenGFW analyzers first if configured
	if a.ogfwDetector != nil && (a.protoEngine == "opengfw" || a.protoEngine == "both") {
		result := a.ogfwDetector.AnalyzePacket(p.SrcIP, p.DstIP, srcPort, dstPort, payload, transport)
		if result != nil && result.ProtoName != "" {
			a.enrichFromOGFW(result, p)
			if a.protoEngine == "opengfw" {
				return result.ProtoName
			}
			// For "both" mode, still fall through to nDPI for better results
			if result.ProtoName != "Encrypted" && result.ProtoName != "" {
				return result.ProtoName
			}
			// For Encrypted (FET), let nDPI try too
		}
	}

	// Try nDPI
	if a.ndpiEngine != nil && p.SrcIP != nil && p.DstIP != nil {
		proto := a.classifyNDPI(p, srcPort, dstPort, transport, packet)
		if proto != "" && proto != "Unknown" {
			return proto
		}
	}

	// Fallback to OpenGFW if nDPI failed and we're in "both" mode
	if a.ogfwDetector != nil && a.protoEngine == "both" {
		result := a.ogfwDetector.AnalyzePacket(p.SrcIP, p.DstIP, srcPort, dstPort, payload, transport)
		if result != nil && result.ProtoName != "" {
			a.enrichFromOGFW(result, p)
			return result.ProtoName
		}
	}

	switch {
	case dstPort == 80 || srcPort == 80 || dstPort == 8080 || srcPort == 8080:
		extractHTTPInfo(payload, p)
		return httpEnrich(p)
	case dstPort == 443 || srcPort == 443:
		extractTLSSNI(payload, p)
		if p.TLSSNI != "" {
			return "TLS (" + p.TLSSNI + ")"
		}
		return "TLS"
	case dstPort == 53 || srcPort == 53:
		return "DNS"
	case dstPort == 22 || srcPort == 22:
		return "SSH"
	case dstPort == 25 || srcPort == 25:
		return "SMTP"
	case dstPort == 67 || srcPort == 67 || dstPort == 68 || srcPort == 68:
		return "DHCP"
	case dstPort == 161 || srcPort == 161 || dstPort == 162 || srcPort == 162:
		return "SNMP"
	case dstPort == 3306 || srcPort == 3306:
		return "MySQL"
	case dstPort == 5432 || srcPort == 5432:
		return "PostgreSQL"
	case dstPort == 6379 || srcPort == 6379:
		return "Redis"
	case dstPort == 27017 || srcPort == 27017:
		return "MongoDB"
	case dstPort == 1433 || srcPort == 1433:
		return "MSSQL"
	case dstPort == 389 || srcPort == 389 || dstPort == 636 || srcPort == 636:
		return "LDAP"
	case dstPort == 3389 || srcPort == 3389:
		return "RDP"
	case dstPort == 5900 || srcPort == 5900:
		return "VNC"
	case dstPort == 21 || srcPort == 21:
		return "FTP"
	case dstPort == 123 || srcPort == 123:
		return "NTP"
	}

	if len(payload) > 0 && payload[0] == 0x16 {
		extractTLSSNI(payload, p)
		if p.TLSSNI != "" {
			return "TLS (" + p.TLSSNI + ")"
		}
		return "TLS"
	}
	if len(payload) > 4 {
		b0, b1, b2, b3 := payload[0], payload[1], payload[2], payload[3]
		if (b0 == 'G' && b1 == 'E' && b2 == 'T' && b3 == ' ') ||
			(b0 == 'P' && b1 == 'O' && b2 == 'S' && b3 == 'T') ||
			(b0 == 'P' && b1 == 'U' && b2 == 'T' && b3 == ' ') ||
			(b0 == 'D' && b1 == 'E' && b2 == 'L' && b3 == 'E') ||
			(b0 == 'H' && b1 == 'E' && b2 == 'A' && b3 == 'D') ||
			(b0 == 'O' && b1 == 'P' && b2 == 'T' && b3 == 'I') ||
			(b0 == 'H' && b1 == 'T' && b2 == 'T' && b3 == 'P') {
			extractHTTPInfo(payload, p)
			return httpEnrich(p)
		}
		// Redis: *N (array) or +OK or -ERR or $N (bulk) or :N (integer)
		if (b0 == '*' || b0 == '+' || b0 == '-' || b0 == ':' || b0 == '$') && b1 >= '0' && b1 <= '9' {
			return "Redis"
		}
	}

	return transport
}

// classifyNDPI uses nDPI deep packet inspection to detect the application protocol.
// It extracts raw IP bytes from the packet, skipping any L2 headers.
func (a *Analyzer) classifyNDPI(p *ParsedPacket, srcPort, dstPort uint16, transport string, packet gopacket.Packet) string {
	// Extract raw IP packet bytes (skip Ethernet/VLAN headers)
	rawData := packet.Data()
	offset := 0
	if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
		offset = len(ethLayer.LayerContents())
		// Check for VLAN (802.1Q adds 4 bytes)
		if len(rawData) > offset+4 {
			tag := (uint16(rawData[offset]) << 8) | uint16(rawData[offset+1])
			if tag == 0x8100 {
				offset += 4
			}
		}
	}
	if offset >= len(rawData) {
		return ""
	}
	ipPacket := rawData[offset:]

	// Determine IP protocol number for the flow key
	var ipProto uint8
	switch transport {
	case "TCP":
		ipProto = 6
	case "UDP":
		ipProto = 17
	default:
		return ""
	}

	key := ndpi.FlowKey{
		SrcIP:    p.SrcIP.String(),
		DstIP:    p.DstIP.String(),
		SrcPort:  srcPort,
		DstPort:  dstPort,
		Protocol: ipProto,
	}

	result, err := a.ndpiEngine.Detect(ipPacket, key)
	if err != nil {
		return ""
	}

	return result.ProtoName
}

func (a *Analyzer) ipv6Proto(ip6 *layers.IPv6) string {
	switch ip6.NextHeader {
	case layers.IPProtocolTCP:
		return "TCP"
	case layers.IPProtocolUDP:
		return "UDP"
	case layers.IPProtocolICMPv6:
		return "ICMPv6"
	default:
		return "IPv6"
	}
}

// extractTLSSNI parses a TLS ClientHello to extract the SNI server name.
func extractTLSSNI(payload []byte, p *ParsedPacket) {
	if len(payload) < 43 {
		return
	}
	// TLS record: 1 byte type (0x16), 2 bytes version, 2 bytes length
	if payload[0] != 0x16 {
		return
	}
	recordLen := int(payload[3])<<8 | int(payload[4])
	if 5+recordLen > len(payload) {
		return
	}
	// Handshake: 1 byte type (0x01 ClientHello), 3 bytes length
	hs := payload[5:]
	if len(hs) < 38 || hs[0] != 0x01 {
		return
	}
	// hs[1:4] = handshake length
	// hs[4:6] = client version
	// hs[6:38] = client random (32 bytes)
	// Skip session ID
	offset := 38
	if offset >= len(hs) {
		return
	}
	sidLen := int(hs[offset])
	offset += 1 + sidLen
	if offset+2 > len(hs) {
		return
	}
	// Cipher suites
	csLen := int(hs[offset])<<8 | int(hs[offset+1])
	offset += 2 + csLen
	if offset+1 > len(hs) {
		return
	}
	// Compression methods
	compLen := int(hs[offset])
	offset += 1 + compLen
	if offset+2 > len(hs) {
		return
	}
	// Extensions
	extLen := int(hs[offset])<<8 | int(hs[offset+1])
	offset += 2
	end := offset + extLen
	if end > len(hs) {
		end = len(hs)
	}
	for offset+4 <= end {
		extType := int(hs[offset])<<8 | int(hs[offset+1])
		extDataLen := int(hs[offset+2])<<8 | int(hs[offset+3])
		offset += 4
		if offset+extDataLen > end {
			break
		}
		if extType == 0 { // server_name extension
			// server_name extension: 2 bytes server_name_list length, 1 byte name_type (0=hostname), 2 bytes length, name
			if extDataLen >= 5 {
				snListLen := int(hs[offset])<<8 | int(hs[offset+1])
				if snListLen <= extDataLen-2 && snListLen >= 3 {
					nameType := hs[offset+2]
					nameLen := int(hs[offset+3])<<8 | int(hs[offset+4])
					if nameType == 0 && nameLen > 0 && offset+5+nameLen <= len(hs) {
						p.TLSSNI = string(hs[offset+5 : offset+5+nameLen])
						return
					}
				}
			}
			return
		}
		offset += extDataLen
	}
}

// extractHTTPInfo extracts Host, URL path, and response status code from HTTP payload.
func extractHTTPInfo(payload []byte, p *ParsedPacket) {
	if len(payload) < 12 {
		return
	}
	text := string(payload)
	lines := strings.SplitN(text, "\r\n", 20)
	if len(lines) == 0 {
		return
	}

	firstLine := lines[0]

	// HTTP response: "HTTP/1.1 200 OK" or "HTTP/2 200"
	if strings.HasPrefix(firstLine, "HTTP/") {
		parts := strings.SplitN(firstLine, " ", 3)
		if len(parts) >= 2 {
			if code, err := strconv.Atoi(parts[1]); err == nil && code >= 100 && code < 600 {
				p.HTTPStatus = code
				// Try to find Content-Type for additional info
				for _, line := range lines[1:] {
					if len(line) == 0 {
						break
					}
					if strings.HasPrefix(strings.ToLower(line), "content-type:") {
						ct := strings.TrimSpace(line[13:])
						if idx := strings.IndexByte(ct, ';'); idx > 0 {
							ct = ct[:idx]
						}
						if ct != "" {
							p.HTTPURL = ct
						}
						break
					}
				}
			}
		}
		return
	}

	// HTTP request: METHOD /path HTTP/1.x
	parts := strings.SplitN(firstLine, " ", 3)
	if len(parts) >= 2 {
		p.HTTPURL = parts[1]
	}

	// Extract Host header
	for _, line := range lines[1:] {
		if len(line) == 0 {
			break
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "host:") {
			host := strings.TrimSpace(line[5:])
			host = strings.TrimSpace(host)
			if host != "" {
				p.HTTPHost = host
				return
			}
		}
	}
}

// parseDNSName extracts the first query name from a DNS payload.
func parseDNSName(payload []byte) string {
	if len(payload) < 13 {
		return ""
	}
	// Skip 12-byte DNS header
	offset := 12
	parts := make([]string, 0, 8)
	for offset < len(payload) {
		length := int(payload[offset])
		if length == 0 {
			break
		}
		if length >= 64 || offset+1+length > len(payload) {
			break
		}
		offset++
		parts = append(parts, string(payload[offset:offset+length]))
		offset += length
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ".")
}

// enrichFromOGFW fills ParsedPacket fields from OpenGFW analyzer results.
func (a *Analyzer) enrichFromOGFW(result *AnalyzeResult, p *ParsedPacket) {
	if result == nil || result.Props == nil {
		return
	}
	// Extract SNI from TLS or QUIC
	if sni := result.Props.Get("req.sni"); sni != nil {
		if s, ok := sni.(string); ok {
			p.TLSSNI = s
		}
	}
	// Extract HTTP Host
	if host := result.Props.Get("req.host"); host != nil {
		if s, ok := host.(string); ok {
			p.HTTPHost = s
		}
	}
	// Extract HTTP URL path
	if path := result.Props.Get("req.path"); path != nil {
		if s, ok := path.(string); ok {
			p.HTTPURL = s
		}
	}
	// Extract DNS query name
	if questions := result.Props.Get("questions"); questions != nil {
		if qs, ok := questions.([]interface{}); ok && len(qs) > 0 {
			if q0, ok := qs[0].(map[string]interface{}); ok {
				if name, ok := q0["name"]; ok {
					if s, ok := name.(string); ok && s != "" {
						p.DNSQuery = s
					}
				}
			}
		}
	}
}
