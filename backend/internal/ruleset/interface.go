package ruleset

import (
	"context"
	"net"
	"strconv"

	"github.com/netgazer/backend/internal/analyzer"
	"github.com/netgazer/backend/internal/modifier"
)

type Action int

const (
	ActionMaybe Action = iota
	ActionAllow
	ActionBlock
	ActionDrop
	ActionModify
)

func (a Action) String() string {
	switch a {
	case ActionMaybe:
		return "maybe"
	case ActionAllow:
		return "allow"
	case ActionBlock:
		return "block"
	case ActionDrop:
		return "drop"
	case ActionModify:
		return "modify"
	default:
		return "unknown"
	}
}

type Protocol int

func (p Protocol) String() string {
	switch p {
	case ProtocolTCP:
		return "tcp"
	case ProtocolUDP:
		return "udp"
	default:
		return "unknown"
	}
}

const (
	ProtocolTCP Protocol = iota
	ProtocolUDP
)

type StreamInfo struct {
	ID               int64
	Protocol         Protocol
	SrcIP, DstIP     net.IP
	SrcPort, DstPort uint16
	Props            analyzer.CombinedPropMap
}

func (i StreamInfo) SrcString() string {
	return net.JoinHostPort(i.SrcIP.String(), strconv.Itoa(int(i.SrcPort)))
}

func (i StreamInfo) DstString() string {
	return net.JoinHostPort(i.DstIP.String(), strconv.Itoa(int(i.DstPort)))
}

type MatchResult struct {
	Action      Action
	ModInstance modifier.Instance
}

type Ruleset interface {
	Analyzers(StreamInfo) []analyzer.Analyzer
	Match(StreamInfo) MatchResult
}

type Logger interface {
	Log(info StreamInfo, name string)
	MatchError(info StreamInfo, name string, err error)
}

type BuiltinConfig struct {
	Logger               Logger
	GeoSiteFilename      string
	GeoIpFilename        string
	ProtectedDialContext func(ctx context.Context, network, address string) (net.Conn, error)
	GeoipLookup          func(ip string) string // returns ISO country code (e.g. "CN", "US")
}
