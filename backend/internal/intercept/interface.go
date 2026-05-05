package intercept

import (
	"context"
	"net"
)

type Verdict int

const (
	VerdictAccept       Verdict = iota
	VerdictAcceptModify
	VerdictAcceptStream
	VerdictDrop
	VerdictDropStream
)

type Packet interface {
	StreamID() uint32
	Data() []byte
}

type PacketCallback func(Packet, error) bool

type PacketIO interface {
	Register(context.Context, PacketCallback) error
	SetVerdict(Packet, Verdict, []byte) error
	ProtectedDialContext(ctx context.Context, network, address string) (net.Conn, error)
	Close() error
}

type ErrInvalidPacket struct {
	Err error
}

func (e *ErrInvalidPacket) Error() string {
	return "invalid packet: " + e.Err.Error()
}
