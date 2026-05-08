package collector

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
)

// TrapMessage represents a parsed SNMP trap.
type TrapMessage struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
	Version   string            `json:"version"`
	Community string            `json:"community,omitempty"`
	OID       string            `json:"oid"`
	Variables map[string]string `json:"variables"`
}

// TrapCallback is called with each received SNMP trap.
type TrapCallback func(msg TrapMessage)

// TrapReceiver listens for SNMP traps via UDP.
type TrapReceiver struct {
	port     int
	callback TrapCallback
	listener *gosnmp.TrapListener
}

// NewTrapReceiver creates a new SNMP trap receiver.
func NewTrapReceiver(port int, cb TrapCallback) *TrapReceiver {
	return &TrapReceiver{port: port, callback: cb}
}

// Start begins listening for SNMP traps.
func (r *TrapReceiver) Start(ctx context.Context) error {
	tl := gosnmp.NewTrapListener()
	tl.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		msg := TrapMessage{
			Timestamp: time.Now(),
			Source:    addr.IP.String(),
			Variables: make(map[string]string),
			ID:        fmt.Sprintf("trap-%s-%d", addr.IP.String(), time.Now().UnixNano()),
		}

		if packet.Version == gosnmp.Version3 {
			msg.Version = "3"
		} else if packet.Version == gosnmp.Version1 {
			msg.Version = "1"
		} else {
			msg.Version = "2c"
		}
		msg.Community = packet.Community

		for _, v := range packet.Variables {
			msg.Variables[v.Name] = fmt.Sprintf("%v", v.Value)
			if msg.OID == "" {
				msg.OID = v.Name
			}
		}

		if r.callback != nil {
			r.callback(msg)
		}
	}

	r.listener = tl

	go func() {
		addr := fmt.Sprintf(":%d", r.port)
		log.Printf("[snmptrap] listening on UDP %s", addr)
		if err := tl.Listen(addr); err != nil {
			log.Printf("[snmptrap] listen error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		tl.Close()
	}()

	return nil
}

var (
	trapStore   []TrapMessage
	trapStoreMu sync.Mutex
)

// StoreTrap stores a trap message in memory.
func StoreTrap(msg TrapMessage) {
	trapStoreMu.Lock()
	defer trapStoreMu.Unlock()
	trapStore = append(trapStore, msg)
	if len(trapStore) > 500 {
		trapStore = trapStore[len(trapStore)-500:]
	}
}

// GetTraps returns stored trap messages, newest first.
func GetTraps(limit int) []TrapMessage {
	trapStoreMu.Lock()
	defer trapStoreMu.Unlock()
	n := len(trapStore)
	if limit <= 0 || limit > n {
		limit = n
	}
	start := n - limit
	result := make([]TrapMessage, limit)
	for i := 0; i < limit; i++ {
		result[i] = trapStore[start+i]
	}
	// Reverse to newest first
	for i, j := 0, limit-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}
