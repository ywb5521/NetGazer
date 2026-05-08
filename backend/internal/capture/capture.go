package capture

import (
	"context"
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

type Engine struct {
	handle  *pcap.Handle
	iface   string
	filter  string
	promisc bool
}

func NewEngine(iface, filter string, promisc bool) (*Engine, error) {
	handle, err := pcap.OpenLive(iface, 65535, promisc, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("open live: %w", err)
	}
	if filter != "" {
		if err := handle.SetBPFFilter(filter); err != nil {
			handle.Close()
			return nil, fmt.Errorf("set BPF filter: %w", err)
		}
	}
	return &Engine{handle: handle, iface: iface, filter: filter, promisc: promisc}, nil
}

func (e *Engine) Start(ctx context.Context) <-chan gopacket.Packet {
	out := make(chan gopacket.Packet, 4096)
	src := gopacket.NewPacketSource(e.handle, e.handle.LinkType())
	src.NoCopy = true

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			packet, err := src.NextPacket()
			if err != nil {
				// Temporary error or timeout, continue
				if err == pcap.NextErrorTimeoutExpired {
					select {
					case <-ctx.Done():
						return
					default:
						continue
					}
				}
				return
			}
			select {
			case out <- packet:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (e *Engine) Stop() {
	e.handle.Close()
}

func (e *Engine) Interface() string {
	return e.iface
}

func (e *Engine) LinkType() string {
	return e.handle.LinkType().String()
}

func init() {
	// Warm up: ensure pcap is available
	_ = time.Now
}
