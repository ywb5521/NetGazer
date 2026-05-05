package intercept

import (
	"net"
	"sync"

	"github.com/gtopng/backend/internal/analyzer"
	"github.com/gtopng/backend/internal/ruleset"

	"github.com/bwmarrin/snowflake"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/reassembly"
)

type tcpVerdict Verdict

const (
	tcpVerdictAccept       = tcpVerdict(VerdictAccept)
	tcpVerdictAcceptStream = tcpVerdict(VerdictAcceptStream)
	tcpVerdictDropStream   = tcpVerdict(VerdictDropStream)
)

type tcpContext struct {
	*gopacket.PacketMetadata
	Verdict tcpVerdict
}

func (ctx *tcpContext) GetCaptureInfo() gopacket.CaptureInfo {
	return ctx.CaptureInfo
}

type tcpStreamFactory struct {
	WorkerID int
	Logger   engineLogger
	Node     *snowflake.Node

	RulesetMutex sync.RWMutex
	Ruleset      ruleset.Ruleset
}

func (f *tcpStreamFactory) New(ipFlow, tcpFlow gopacket.Flow, tcp *layers.TCP, ac reassembly.AssemblerContext) reassembly.Stream {
	id := f.Node.Generate()
	ipSrc, ipDst := net.IP(ipFlow.Src().Raw()), net.IP(ipFlow.Dst().Raw())
	info := ruleset.StreamInfo{
		ID:       id.Int64(),
		Protocol: ruleset.ProtocolTCP,
		SrcIP:    ipSrc,
		DstIP:    ipDst,
		SrcPort:  uint16(tcp.SrcPort),
		DstPort:  uint16(tcp.DstPort),
		Props:    make(analyzer.CombinedPropMap),
	}
	f.Logger.TCPStreamNew(f.WorkerID, info)
	f.RulesetMutex.RLock()
	rs := f.Ruleset
	f.RulesetMutex.RUnlock()
	ans := analyzersToTCPAnalyzers(rs.Analyzers(info))
	entries := make([]*tcpStreamEntry, 0, len(ans))
	for _, a := range ans {
		entries = append(entries, &tcpStreamEntry{
			Name: a.Name(),
			Stream: a.NewTCP(analyzer.TCPInfo{
				SrcIP:   ipSrc,
				DstIP:   ipDst,
				SrcPort: uint16(tcp.SrcPort),
				DstPort: uint16(tcp.DstPort),
			}, &analyzerLogger{
				StreamID: id.Int64(),
				Name:     a.Name(),
				Logger:   f.Logger,
			}),
			HasLimit: a.Limit() > 0,
			Quota:    a.Limit(),
		})
	}
	return &tcpStream{
		info:          info,
		virgin:        true,
		logger:        f.Logger,
		ruleset:       rs,
		activeEntries: entries,
	}
}

func (f *tcpStreamFactory) UpdateRuleset(r ruleset.Ruleset) error {
	f.RulesetMutex.Lock()
	defer f.RulesetMutex.Unlock()
	f.Ruleset = r
	return nil
}

type tcpStream struct {
	info          ruleset.StreamInfo
	virgin        bool
	logger        engineLogger
	ruleset       ruleset.Ruleset
	activeEntries []*tcpStreamEntry
	doneEntries   []*tcpStreamEntry
	lastVerdict   tcpVerdict
}

type tcpStreamEntry struct {
	Name     string
	Stream   analyzer.TCPStream
	HasLimit bool
	Quota    int
}

func (s *tcpStream) Accept(tcp *layers.TCP, ci gopacket.CaptureInfo, dir reassembly.TCPFlowDirection, nextSeq reassembly.Sequence, start *bool, ac reassembly.AssemblerContext) bool {
	if len(s.activeEntries) > 0 || s.virgin {
		return true
	}
	ctx := ac.(*tcpContext)
	ctx.Verdict = s.lastVerdict
	return false
}

func (s *tcpStream) ReassembledSG(sg reassembly.ScatterGather, ac reassembly.AssemblerContext) {
	dir, start, end, skip := sg.Info()
	rev := dir == reassembly.TCPDirServerToClient
	avail, _ := sg.Lengths()
	data := sg.Fetch(avail)
	updated := false
	for i := len(s.activeEntries) - 1; i >= 0; i-- {
		entry := s.activeEntries[i]
		update, closeUpdate, done := s.feedEntry(entry, rev, start, end, skip, data)
		up1 := processPropUpdate(s.info.Props, entry.Name, update)
		up2 := processPropUpdate(s.info.Props, entry.Name, closeUpdate)
		updated = updated || up1 || up2
		if done {
			s.activeEntries = append(s.activeEntries[:i], s.activeEntries[i+1:]...)
			s.doneEntries = append(s.doneEntries, entry)
		}
	}
	ctx := ac.(*tcpContext)
	if updated || s.virgin {
		s.virgin = false
		s.logger.TCPStreamPropUpdate(s.info, false)
		result := s.ruleset.Match(s.info)
		action := result.Action
		if action != ruleset.ActionMaybe && action != ruleset.ActionModify {
			verdict := actionToTCPVerdict(action)
			s.lastVerdict = verdict
			ctx.Verdict = verdict
			s.logger.TCPStreamAction(s.info, action, false)
			s.closeActiveEntries()
		}
	}
	if len(s.activeEntries) == 0 && ctx.Verdict == tcpVerdictAccept {
		s.lastVerdict = tcpVerdictAcceptStream
		ctx.Verdict = tcpVerdictAcceptStream
		s.logger.TCPStreamAction(s.info, ruleset.ActionAllow, true)
	}
}

func (s *tcpStream) ReassemblyComplete(ac reassembly.AssemblerContext) bool {
	s.closeActiveEntries()
	return true
}

func (s *tcpStream) closeActiveEntries() {
	updated := false
	for _, entry := range s.activeEntries {
		update := entry.Stream.Close(false)
		up := processPropUpdate(s.info.Props, entry.Name, update)
		updated = updated || up
	}
	if updated {
		s.logger.TCPStreamPropUpdate(s.info, true)
	}
	s.doneEntries = append(s.doneEntries, s.activeEntries...)
	s.activeEntries = nil
}

func (s *tcpStream) feedEntry(entry *tcpStreamEntry, rev, start, end bool, skip int, data []byte) (update *analyzer.PropUpdate, closeUpdate *analyzer.PropUpdate, done bool) {
	if !entry.HasLimit {
		update, done = entry.Stream.Feed(rev, start, end, skip, data)
	} else {
		qData := data
		if len(qData) > entry.Quota {
			qData = qData[:entry.Quota]
		}
		update, done = entry.Stream.Feed(rev, start, end, skip, qData)
		entry.Quota -= len(qData)
		if entry.Quota <= 0 {
			closeUpdate = entry.Stream.Close(true)
			done = true
		}
	}
	return
}

func analyzersToTCPAnalyzers(ans []analyzer.Analyzer) []analyzer.TCPAnalyzer {
	tcpAns := make([]analyzer.TCPAnalyzer, 0, len(ans))
	for _, a := range ans {
		if tcpM, ok := a.(analyzer.TCPAnalyzer); ok {
			tcpAns = append(tcpAns, tcpM)
		}
	}
	return tcpAns
}

func actionToTCPVerdict(a ruleset.Action) tcpVerdict {
	switch a {
	case ruleset.ActionMaybe, ruleset.ActionAllow, ruleset.ActionModify:
		return tcpVerdictAcceptStream
	case ruleset.ActionBlock, ruleset.ActionDrop:
		return tcpVerdictDropStream
	default:
		return tcpVerdictAcceptStream
	}
}
