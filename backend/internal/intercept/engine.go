package intercept

import (
	"context"
	"log"
	"runtime"
	"sync/atomic"

	"github.com/netgazer/backend/internal/modifier"
	"github.com/netgazer/backend/internal/ruleset"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type EngineConfig struct {
	IO                               PacketIO
	Ruleset                          ruleset.Ruleset
	Workers                          int
	WorkerQueueSize                  int
	WorkerTCPMaxBufferedPagesTotal   int
	WorkerTCPMaxBufferedPagesPerConn int
	WorkerUDPMaxStreams              int
	Modifiers                        []modifier.Modifier
}

type Engine struct {
	io      PacketIO
	workers []*worker
	ruleset atomic.Value // ruleset.Ruleset
}

func NewEngine(config EngineConfig) (*Engine, error) {
	logger := &noopLogger{}
	workerCount := config.Workers
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}
	workers := make([]*worker, workerCount)
	for i := range workers {
		var err error
		workers[i], err = newWorker(workerConfig{
			ID:                         i,
			ChanSize:                   config.WorkerQueueSize,
			Logger:                     logger,
			Ruleset:                    config.Ruleset,
			TCPMaxBufferedPagesTotal:   config.WorkerTCPMaxBufferedPagesTotal,
			TCPMaxBufferedPagesPerConn: config.WorkerTCPMaxBufferedPagesPerConn,
			UDPMaxStreams:              config.WorkerUDPMaxStreams,
			Modifiers:                  config.Modifiers,
		})
		if err != nil {
			return nil, err
		}
	}
	e := &Engine{
		io:      config.IO,
		workers: workers,
	}
	e.ruleset.Store(config.Ruleset)
	return e, nil
}

func (e *Engine) Start(ctx context.Context) error {
	ioCtx, ioCancel := context.WithCancel(ctx)
	defer ioCancel()

	for _, w := range e.workers {
		go w.Run(ioCtx)
	}

	errChan := make(chan error, 1)
	err := e.io.Register(ioCtx, func(p Packet, err error) bool {
		if err != nil {
			errChan <- err
			return false
		}
		return e.dispatch(p)
	})
	if err != nil {
		return err
	}

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (e *Engine) UpdateRuleset(r ruleset.Ruleset) error {
	e.ruleset.Store(r)
	for _, w := range e.workers {
		if err := w.UpdateRuleset(r); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) Stop() error {
	return e.io.Close()
}

func (e *Engine) dispatch(p Packet) bool {
	data := p.Data()
	ipVersion := data[0] >> 4
	var layerType gopacket.LayerType
	if ipVersion == 4 {
		layerType = layers.LayerTypeIPv4
	} else if ipVersion == 6 {
		layerType = layers.LayerTypeIPv6
	} else {
		_ = e.io.SetVerdict(p, VerdictAcceptStream, nil)
		return true
	}
	index := p.StreamID() % uint32(len(e.workers))
	packet := gopacket.NewPacket(data, layerType, gopacket.DecodeOptions{Lazy: true, NoCopy: true})
	e.workers[index].Feed(&workerPacket{
		StreamID: p.StreamID(),
		Packet:   packet,
		SetVerdict: func(v Verdict, b []byte) error {
			return e.io.SetVerdict(p, v, b)
		},
	})
	return true
}

// noopLogger provides basic logger output for the engine.
type noopLogger struct{}

func (l *noopLogger) WorkerStart(id int)                                      { log.Printf("[intercept] worker %d started", id) }
func (l *noopLogger) WorkerStop(id int)                                       { log.Printf("[intercept] worker %d stopped", id) }
func (l *noopLogger) TCPStreamNew(id int, info ruleset.StreamInfo)            {}
func (l *noopLogger) TCPStreamPropUpdate(info ruleset.StreamInfo, close bool) {}
func (l *noopLogger) TCPStreamAction(info ruleset.StreamInfo, action ruleset.Action, noMatch bool) {
	if action == ruleset.ActionBlock {
		log.Printf("[intercept] BLOCK TCP %s -> %s", info.SrcString(), info.DstString())
	}
}
func (l *noopLogger) UDPStreamNew(id int, info ruleset.StreamInfo)            {}
func (l *noopLogger) UDPStreamPropUpdate(info ruleset.StreamInfo, close bool) {}
func (l *noopLogger) UDPStreamAction(info ruleset.StreamInfo, action ruleset.Action, noMatch bool) {
	if action == ruleset.ActionBlock || action == ruleset.ActionDrop {
		log.Printf("[intercept] BLOCK UDP %s -> %s", info.SrcString(), info.DstString())
	}
}
func (l *noopLogger) ModifyError(info ruleset.StreamInfo, err error) {
	log.Printf("[intercept] modify error: %v", err)
}
func (l *noopLogger) AnalyzerDebugf(streamID int64, name string, format string, args ...interface{}) {
}
func (l *noopLogger) AnalyzerInfof(streamID int64, name string, format string, args ...interface{}) {}
func (l *noopLogger) AnalyzerErrorf(streamID int64, name string, format string, args ...interface{}) {
}
func (l *noopLogger) Log(info ruleset.StreamInfo, name string) {
	log.Printf("[intercept] rule match: %s on %s -> %s", name, info.SrcString(), info.DstString())
}
func (l *noopLogger) MatchError(info ruleset.StreamInfo, name string, err error) {
	log.Printf("[intercept] match error: %s: %v", name, err)
}

// Ensure noopLogger implements required interfaces.
var (
	_ engineLogger   = (*noopLogger)(nil)
	_ ruleset.Logger = (*noopLogger)(nil)
)

// engineLogger is the combined logging interface.
type engineLogger interface {
	WorkerStart(id int)
	WorkerStop(id int)
	TCPStreamNew(id int, info ruleset.StreamInfo)
	TCPStreamPropUpdate(info ruleset.StreamInfo, close bool)
	TCPStreamAction(info ruleset.StreamInfo, action ruleset.Action, noMatch bool)
	UDPStreamNew(id int, info ruleset.StreamInfo)
	UDPStreamPropUpdate(info ruleset.StreamInfo, close bool)
	UDPStreamAction(info ruleset.StreamInfo, action ruleset.Action, noMatch bool)
	ModifyError(info ruleset.StreamInfo, err error)
	AnalyzerDebugf(streamID int64, name string, format string, args ...interface{})
	AnalyzerInfof(streamID int64, name string, format string, args ...interface{})
	AnalyzerErrorf(streamID int64, name string, format string, args ...interface{})
}
