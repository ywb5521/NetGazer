package discovery

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ping sends ICMP echo requests to the target IP and waits for a reply.
func ping(ip net.IP, count int, timeout time.Duration) (bool, time.Duration) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		log.Printf("[discovery] ping listener failed: %v", err)
		return false, 0
	}
	defer conn.Close()

	conn.IPv4PacketConn().SetTTL(64)

	var rttSum time.Duration
	replies := 0

	for seq := 0; seq < count; seq++ {
		// Build ICMP echo request
		msg := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   int(binary.BigEndian.Uint16([]byte{byte(0x1f), byte(0xf1)})) & 0xffff,
				Seq:  seq,
				Data: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			},
		}

		b, err := msg.Marshal(nil)
		if err != nil {
			continue
		}

		dest := &net.IPAddr{IP: ip}
		sendTime := time.Now()
		if _, err := conn.WriteTo(b, dest); err != nil {
			continue
		}

		// Wait for reply
		conn.SetReadDeadline(time.Now().Add(timeout))
		reply := make([]byte, 1500)
		n, _, err := conn.ReadFrom(reply)
		if err != nil {
			continue
		}

		rtt := time.Since(sendTime)

		// Parse reply
		rm, err := icmp.ParseMessage(1, reply[:n])
		if err != nil {
			continue
		}

		if rm.Type == ipv4.ICMPTypeEchoReply && rm.Code == 0 {
			rttSum += rtt
			replies++
		}
	}

	if replies == 0 {
		return false, 0
	}

	return true, rttSum / time.Duration(replies)
}

// pingSweep pings all hosts in a subnet and returns the ones that respond.
func pingSweep(subnet *net.IPNet, count int, timeout time.Duration, parallel int) []DiscoveredHost {
	ips := allSubnetIPs(subnet)
	if len(ips) == 0 {
		return nil
	}

	var hosts []DiscoveredHost
	var mu sync.Mutex
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip net.IP) {
			defer wg.Done()
			defer func() { <-sem }()

			alive, rtt := ping(ip, count, timeout)
			_ = rtt
			if alive {
				mu.Lock()
				hosts = append(hosts, DiscoveredHost{
					IP:     ip,
					Method: "ping",
				})
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	return hosts
}
