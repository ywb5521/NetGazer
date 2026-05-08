package discovery

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
	"syscall"
	"time"
)

// htons converts a uint16 from host to network byte order.
func htons(v uint16) uint16 {
	return (v<<8)&0xff00 | (v>>8)&0x00ff
}

// arpPacket builds a raw ARP request packet (Ethernet + ARP).
func arpPacket(srcMAC net.HardwareAddr, srcIP, targetIP net.IP) ([]byte, error) {
	// Ethernet header: dst MAC (6) + src MAC (6) + EtherType (2)
	// ARP: htype(2) + ptype(2) + hlen(1) + plen(1) + oper(2) + sha(6) + spa(4) + tha(6) + tpa(4)
	// Total: 14 + 28 = 42 bytes

	pkt := make([]byte, 42)

	// Ethernet header
	copy(pkt[0:6], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) // broadcast
	copy(pkt[6:12], srcMAC)
	binary.BigEndian.PutUint16(pkt[12:14], 0x0806) // EtherType ARP

	// ARP header
	offset := 14
	binary.BigEndian.PutUint16(pkt[offset:offset+2], 1)        // HTYPE Ethernet
	binary.BigEndian.PutUint16(pkt[offset+2:offset+4], 0x0800) // PTYPE IPv4
	pkt[offset+4] = 6                                          // HLEN
	pkt[offset+5] = 4                                          // PLEN
	binary.BigEndian.PutUint16(pkt[offset+6:offset+8], 1)      // OPER request
	copy(pkt[offset+8:offset+14], srcMAC[:6])                  // SHA
	copy(pkt[offset+14:offset+18], srcIP.To4()[:4])            // SPA
	// THA left as zeros (unknown)
	copy(pkt[offset+24:offset+28], targetIP.To4()[:4]) // TPA

	return pkt, nil
}

// scanARP performs an ARP scan on the given subnet using the specified interface.
func scanARP(iface *net.Interface, subnet *net.IPNet, timeout time.Duration) []DiscoveredHost {
	srcIP := getInterfaceIP(iface, subnet)
	if srcIP == nil {
		log.Printf("[discovery] ARP: no suitable IP on %s for subnet %s", iface.Name, subnet)
		return nil
	}

	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		log.Printf("[discovery] ARP: socket creation failed: %v", err)
		return nil
	}
	defer syscall.Close(fd)

	// Bind to interface
	addr := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),
		Ifindex:  iface.Index,
	}
	if err := syscall.Bind(fd, &addr); err != nil {
		log.Printf("[discovery] ARP: bind failed: %v", err)
		return nil
	}

	// Set receive timeout
	tv := syscall.Timeval{
		Sec:  int64(timeout.Seconds()),
		Usec: 0,
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		log.Printf("[discovery] ARP: setsockopt failed: %v", err)
		return nil
	}

	// Send ARP requests to all IPs in subnet
	ips := allSubnetIPs(subnet)
	srcMAC := iface.HardwareAddr

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, ip := range ips {
			pkt, err := arpPacket(srcMAC, srcIP, ip)
			if err != nil {
				continue
			}
			destAddr := syscall.SockaddrLinklayer{
				Protocol: htons(syscall.ETH_P_ARP),
				Ifindex:  iface.Index,
			}
			syscall.Sendto(fd, pkt, 0, &destAddr)
			time.Sleep(500 * time.Microsecond) // rate limit to avoid flooding
		}
	}()

	// Receive replies
	var hosts []DiscoveredHost
	seen := make(map[string]bool)
	buf := make([]byte, 4096)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		n, _, err := syscall.Recvfrom(fd, buf, syscall.MSG_DONTWAIT)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}

		if n < 42 {
			continue
		}

		// ARP reply: Ethernet(14) + ARP(28)
		arpData := buf[14:n]
		if len(arpData) < 28 {
			continue
		}

		oper := binary.BigEndian.Uint16(arpData[6:8])
		if oper != 2 { // ARP reply
			continue
		}

		senderIP := net.IP(arpData[14:18]).To4()
		senderMAC := net.HardwareAddr(arpData[8:14])

		if senderIP == nil {
			continue
		}

		ipStr := senderIP.String()
		if seen[ipStr] || !subnet.Contains(senderIP) {
			continue
		}
		seen[ipStr] = true

		hosts = append(hosts, DiscoveredHost{
			IP:     senderIP,
			MAC:    senderMAC,
			Method: "arp",
		})
	}

	wg.Wait()
	return hosts
}

// getInterfaceIP finds the first IPv4 address on the interface matching the subnet.
func getInterfaceIP(iface *net.Interface, subnet *net.IPNet) net.IP {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				if subnet.Contains(ip4) {
					return ip4
				}
			}
		}
	}
	return nil
}

// allSubnetIPs returns all host IPs in a subnet excluding network and broadcast addresses.
func allSubnetIPs(subnet *net.IPNet) []net.IP {
	ones, bits := subnet.Mask.Size()
	if bits != 32 {
		return nil
	}

	ip := subnet.IP.To4()
	if ip == nil {
		return nil
	}

	start := binary.BigEndian.Uint32(ip) & binary.BigEndian.Uint32(subnet.Mask)
	var ips []net.IP

	maxHosts := uint32(1<<(uint32(bits)-uint32(ones))) - 2
	if maxHosts > 4096 {
		// Subnet too large, only scan first 256 + gateway range
		for i := uint32(1); i <= 256 && i <= maxHosts; i++ {
			ipNum := start + i
			ips = append(ips, uint32ToIP(ipNum))
		}
		return ips
	}

	for i := uint32(1); i <= maxHosts; i++ {
		ipNum := start + i
		ips = append(ips, uint32ToIP(ipNum))
	}
	return ips
}

func uint32ToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}
