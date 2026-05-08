package capture

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/netgazer/backend/internal/analyzer"
	"github.com/netgazer/backend/internal/analyzer/tcp"
	"github.com/netgazer/backend/internal/analyzer/udp"
)

// defaultLogger implements analyzer.Logger using the standard log package.
type defaultLogger struct{}

func (l *defaultLogger) Debugf(format string, args ...interface{}) {}
func (l *defaultLogger) Infof(format string, args ...interface{})  {}
func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	log.Printf("analyzer error: "+format, args...)
}

// flowKey identifies a bidirectional flow.
type flowKey struct {
	srcIP   string
	dstIP   string
	srcPort uint16
	dstPort uint16
	proto   string // "TCP" or "UDP"
}

func (k flowKey) String() string {
	return fmt.Sprintf("%s:%d-%s:%d/%s", k.srcIP, k.srcPort, k.dstIP, k.dstPort, k.proto)
}

func canonicalFlowKey(srcIP, dstIP net.IP, srcPort, dstPort uint16, transport string) flowKey {
	src := srcIP.String()
	dst := dstIP.String()
	if src < dst || (src == dst && srcPort <= dstPort) {
		return flowKey{srcIP: src, dstIP: dst, srcPort: srcPort, dstPort: dstPort, proto: transport}
	}
	return flowKey{srcIP: dst, dstIP: src, srcPort: dstPort, dstPort: srcPort, proto: transport}
}

// ogfwFlowState tracks the analyzer streams for a single flow.
type ogfwFlowState struct {
	info        analyzer.TCPInfo
	udpInfo     analyzer.UDPInfo
	tcpStreams  map[string]analyzer.TCPStream
	udpStreams  map[string]analyzer.UDPStream
	packetCount int
	done        bool
}

// OGFWDetector wraps OpenGFW protocol analyzers for use in the capture pipeline.
type OGFWDetector struct {
	logger       analyzer.Logger
	tcpAnalyzers []analyzer.TCPAnalyzer
	udpAnalyzers []analyzer.UDPAnalyzer
	allAnalyzers []analyzer.Analyzer
	flows        map[flowKey]*ogfwFlowState
	mu           sync.RWMutex
}

// NewOGFWDetector creates a new OGFWDetector with all available analyzers.
func NewOGFWDetector() *OGFWDetector {
	logger := &defaultLogger{}
	return &OGFWDetector{
		logger: logger,
		tcpAnalyzers: []analyzer.TCPAnalyzer{
			&tcp.HTTPAnalyzer{},
			&tcp.TLSAnalyzer{},
			&tcp.SSHAnalyzer{},
			&tcp.SocksAnalyzer{},
			&tcp.TrojanAnalyzer{},
			&tcp.FETAnalyzer{},
		},
		udpAnalyzers: []analyzer.UDPAnalyzer{
			&udp.DNSAnalyzer{},
			&udp.QUICAnalyzer{},
			&udp.WireGuardAnalyzer{},
			&udp.OpenVPNAnalyzer{},
		},
		flows: make(map[flowKey]*ogfwFlowState),
	}
}

// AnalyzeResult holds the result of OpenGFW protocol analysis.
type AnalyzeResult struct {
	ProtoName string
	Props     analyzer.PropMap
}

// AnalyzePacket tries to detect the protocol of a single packet using OpenGFW analyzers.
func (d *OGFWDetector) AnalyzePacket(srcIP, dstIP net.IP, srcPort, dstPort uint16, payload []byte, transport string) *AnalyzeResult {
	if len(payload) == 0 {
		return nil
	}

	key := canonicalFlowKey(srcIP, dstIP, srcPort, dstPort, transport)
	rev := key.srcIP != srcIP.String() || key.srcPort != srcPort

	d.mu.Lock()
	state, exists := d.flows[key]
	if !exists {
		state = &ogfwFlowState{
			tcpStreams: make(map[string]analyzer.TCPStream),
			udpStreams: make(map[string]analyzer.UDPStream),
		}
		if transport == "TCP" {
			state.info = analyzer.TCPInfo{
				SrcIP:   net.ParseIP(key.srcIP),
				DstIP:   net.ParseIP(key.dstIP),
				SrcPort: key.srcPort,
				DstPort: key.dstPort,
			}
		} else {
			state.udpInfo = analyzer.UDPInfo{
				SrcIP:   net.ParseIP(key.srcIP),
				DstIP:   net.ParseIP(key.dstIP),
				SrcPort: key.srcPort,
				DstPort: key.dstPort,
			}
		}
		d.flows[key] = state
	}
	d.mu.Unlock()

	if state.done {
		return nil
	}

	state.packetCount++
	if state.packetCount > 10 {
		state.done = true
		return nil
	}

	if transport == "TCP" {
		result, terminal := d.analyzeTCP(state, payload, rev)
		if terminal {
			state.done = true
		}
		return result
	}
	result, terminal := d.analyzeUDP(state, payload, rev)
	if terminal {
		state.done = true
	}
	return result
}

func (d *OGFWDetector) analyzeTCP(state *ogfwFlowState, payload []byte, rev bool) (*AnalyzeResult, bool) {
	for _, an := range d.tcpAnalyzers {
		name := an.Name()
		stream, ok := state.tcpStreams[name]
		if !ok {
			stream = an.NewTCP(state.info, d.logger)
			state.tcpStreams[name] = stream
		}
		update, done := stream.Feed(rev, state.packetCount == 1, false, 0, payload)
		if update != nil && update.M != nil {
			result := resultFromProps(name, update.M)
			if result != nil {
				return result, done && result.ProtoName != "TLS"
			}
		}
		if done && (name == "trojan" || name == "fet") {
			continue
		}
	}
	return nil, false
}

func (d *OGFWDetector) analyzeUDP(state *ogfwFlowState, payload []byte, rev bool) (*AnalyzeResult, bool) {
	for _, an := range d.udpAnalyzers {
		name := an.Name()
		stream, ok := state.udpStreams[name]
		if !ok {
			stream = an.NewUDP(state.udpInfo, d.logger)
			state.udpStreams[name] = stream
		}
		update, done := stream.Feed(rev, payload)
		if update != nil && update.M != nil {
			result := resultFromProps(name, update.M)
			if result != nil {
				return result, done
			}
		}
	}
	return nil, false
}

func resultFromProps(analyzerName string, props analyzer.PropMap) *AnalyzeResult {
	if props == nil {
		return nil
	}
	result := &AnalyzeResult{Props: props}

	switch analyzerName {
	case "http", "HTTP":
		result.ProtoName = "HTTP"
	case "tls", "TLS":
		result.ProtoName = "TLS"
	case "ssh", "SSH":
		result.ProtoName = "SSH"
	case "socks", "SOCKS":
		result.ProtoName = "SOCKS"
		if v := props.Get("version"); v != nil {
			if s, ok := v.(string); ok {
				result.ProtoName = s
			}
		}
	case "trojan", "Trojan":
		if v := props.Get("yes"); v != nil {
			if yes, ok := v.(bool); ok && yes {
				result.ProtoName = "Trojan"
			}
		}
	case "fet", "FET":
		if v := props.Get("yes"); v != nil {
			if yes, ok := v.(bool); ok && yes {
				result.ProtoName = "Encrypted"
			}
		}
	case "dns", "DNS":
		result.ProtoName = "DNS"
	case "quic", "QUIC":
		result.ProtoName = "QUIC"
	case "wireguard", "WireGuard":
		result.ProtoName = "WireGuard"
	case "openvpn", "OpenVPN":
		result.ProtoName = "OpenVPN"
	}

	if result.ProtoName == "" {
		return nil
	}
	return result
}

// AllAnalyzers returns all registered analyzer instances.
func (d *OGFWDetector) AllAnalyzers() []analyzer.Analyzer {
	result := make([]analyzer.Analyzer, 0, len(d.tcpAnalyzers)+len(d.udpAnalyzers))
	for _, a := range d.tcpAnalyzers {
		result = append(result, a)
	}
	for _, a := range d.udpAnalyzers {
		result = append(result, a)
	}
	return result
}

// CleanupFlows removes stale flow states.
func (d *OGFWDetector) CleanupFlows() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for key, state := range d.flows {
		if state.done || state.packetCount > 20 {
			delete(d.flows, key)
		}
	}
}
