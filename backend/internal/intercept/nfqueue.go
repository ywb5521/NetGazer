package intercept

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os/exec"
	"strings"
	"syscall"

	"github.com/coreos/go-iptables/iptables"
	"github.com/florianl/go-nfqueue"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

const (
	nfqueueNum              = 100
	nfqueueMaxPacketLen     = 0xFFFF
	nfqueueDefaultQueueSize = 128
)

var _ PacketIO = (*nfqueuePacketIO)(nil)

var errNotNFQueuePacket = errors.New("not an NFQueue packet")

type nfqueuePacketIO struct {
	n     *nfqueue.Nfqueue
	local bool
	rst   bool
	rSet  bool

	ipt4 *iptables.IPTables
	ipt6 *iptables.IPTables

	protectedDialer *net.Dialer
}

type NFQueuePacketIOConfig struct {
	QueueSize   uint32
	ReadBuffer  int
	WriteBuffer int
	Local       bool
	RST         bool
}

func NewNFQueuePacketIO(config NFQueuePacketIOConfig) (PacketIO, error) {
	if config.QueueSize == 0 {
		config.QueueSize = nfqueueDefaultQueueSize
	}
	var ipt4, ipt6 *iptables.IPTables
	var err error
	if nftCheck() != nil {
		ipt4, err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
		if err != nil {
			return nil, err
		}
		ipt6, err = iptables.NewWithProtocol(iptables.ProtocolIPv6)
		if err != nil {
			return nil, err
		}
	}
	n, err := nfqueue.Open(&nfqueue.Config{
		NfQueue:      nfqueueNum,
		MaxPacketLen: nfqueueMaxPacketLen,
		MaxQueueLen:  config.QueueSize,
		Copymode:     nfqueue.NfQnlCopyPacket,
		Flags:        nfqueue.NfQaCfgFlagConntrack,
	})
	if err != nil {
		return nil, err
	}
	if config.ReadBuffer > 0 {
		err = n.Con.SetReadBuffer(config.ReadBuffer)
		if err != nil {
			_ = n.Close()
			return nil, err
		}
	}
	if config.WriteBuffer > 0 {
		err = n.Con.SetWriteBuffer(config.WriteBuffer)
		if err != nil {
			_ = n.Close()
			return nil, err
		}
	}
	return &nfqueuePacketIO{
		n:     n,
		local: config.Local,
		rst:   config.RST,
		ipt4:  ipt4,
		ipt6:  ipt6,
		protectedDialer: &net.Dialer{
			Control: func(network, address string, c syscall.RawConn) error {
				var err error
				cErr := c.Control(func(fd uintptr) {
					err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK, nfqueueConnMarkAccept)
				})
				if cErr != nil {
					return cErr
				}
				return err
			},
		},
	}, nil
}

func (n *nfqueuePacketIO) Register(ctx context.Context, cb PacketCallback) error {
	err := n.n.RegisterWithErrorFunc(ctx,
		func(a nfqueue.Attribute) int {
			if ok, verdict := n.packetAttributeSanityCheck(a); !ok {
				if a.PacketID != nil {
					_ = n.n.SetVerdict(*a.PacketID, verdict)
				}
				return 0
			}
			p := &nfqueuePacket{
				id:       *a.PacketID,
				streamID: ctIDFromCtBytes(*a.Ct),
				data:     *a.Payload,
			}
			return okBoolToInt(cb(p, nil))
		},
		func(e error) int {
			if opErr := (*netlink.OpError)(nil); errors.As(e, &opErr) {
				if errors.Is(opErr.Err, unix.ENOBUFS) {
					return 0
				}
			}
			return okBoolToInt(cb(nil, e))
		})
	if err != nil {
		return err
	}
	if !n.rSet {
		if n.ipt4 != nil {
			err = n.setupIpt(n.local, n.rst, false)
		} else {
			err = n.setupNft(n.local, n.rst, false)
		}
		if err != nil {
			return err
		}
		n.rSet = true
	}
	return nil
}

func (n *nfqueuePacketIO) packetAttributeSanityCheck(a nfqueue.Attribute) (ok bool, verdict int) {
	if a.PacketID == nil {
		return false, -1
	}
	if a.Payload == nil || len(*a.Payload) < 20 {
		return false, nfqueue.NfDrop
	}
	if a.Ct == nil {
		if n.local {
			return false, nfqueue.NfAccept
		}
		return false, nfqueue.NfDrop
	}
	return true, -1
}

func (n *nfqueuePacketIO) SetVerdict(p Packet, v Verdict, newPacket []byte) error {
	nP, ok := p.(*nfqueuePacket)
	if !ok {
		return &ErrInvalidPacket{Err: errNotNFQueuePacket}
	}
	switch v {
	case VerdictAccept:
		return n.n.SetVerdict(nP.id, nfqueue.NfAccept)
	case VerdictAcceptModify:
		return n.n.SetVerdictModPacket(nP.id, nfqueue.NfAccept, newPacket)
	case VerdictAcceptStream:
		return n.n.SetVerdictWithConnMark(nP.id, nfqueue.NfAccept, nfqueueConnMarkAccept)
	case VerdictDrop:
		return n.n.SetVerdict(nP.id, nfqueue.NfDrop)
	case VerdictDropStream:
		return n.n.SetVerdictWithConnMark(nP.id, nfqueue.NfDrop, nfqueueConnMarkDrop)
	default:
		return nil
	}
}

func (n *nfqueuePacketIO) ProtectedDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return n.protectedDialer.DialContext(ctx, network, address)
}

func (n *nfqueuePacketIO) Close() error {
	if n.rSet {
		if n.ipt4 != nil {
			_ = n.setupIpt(n.local, n.rst, true)
		} else {
			_ = n.setupNft(n.local, n.rst, true)
		}
		n.rSet = false
	}
	return n.n.Close()
}

func (n *nfqueuePacketIO) setupNft(local, rst, remove bool) error {
	rules, err := generateNftRules(local, rst, nfqueueNum)
	if err != nil {
		return err
	}
	rulesText := rules.String()
	if remove {
		err = nftDelete(nftFamily, nftTable)
	} else {
		_ = nftDelete(nftFamily, nftTable)
		err = nftAdd(rulesText)
	}
	return err
}

func (n *nfqueuePacketIO) setupIpt(local, rst, remove bool) error {
	rules, err := generateIptRules(local, rst, nfqueueNum)
	if err != nil {
		return err
	}
	if remove {
		err = iptsBatchDeleteIfExists([]*iptables.IPTables{n.ipt4, n.ipt6}, rules)
	} else {
		err = iptsBatchAppendUnique([]*iptables.IPTables{n.ipt4, n.ipt6}, rules)
	}
	return err
}

var _ Packet = (*nfqueuePacket)(nil)

type nfqueuePacket struct {
	id       uint32
	streamID uint32
	data     []byte
}

func (p *nfqueuePacket) StreamID() uint32 {
	return p.streamID
}

func (p *nfqueuePacket) Data() []byte {
	return p.data
}

func okBoolToInt(ok bool) int {
	if ok {
		return 0
	}
	return 1
}

func nftCheck() error {
	_, err := exec.LookPath("nft")
	return err
}

func nftAdd(input string) error {
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(input)
	return cmd.Run()
}

func nftDelete(family, table string) error {
	cmd := exec.Command("nft", "delete", "table", family, table)
	return cmd.Run()
}

func ctIDFromCtBytes(ct []byte) uint32 {
	ctAttrs, err := netlink.UnmarshalAttributes(ct)
	if err != nil {
		return 0
	}
	for _, attr := range ctAttrs {
		if attr.Type == 12 { // CTA_ID
			return binary.BigEndian.Uint32(attr.Data)
		}
	}
	return 0
}
