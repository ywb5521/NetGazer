package tracker

import (
	"sync"
	"time"

	"github.com/gtopng/backend/internal/capture"
	"github.com/gtopng/backend/internal/models"
)

type HostTracker struct {
	mu    sync.RWMutex
	hosts map[string]*models.Host
}

func NewHostTracker() *HostTracker {
	return &HostTracker{
		hosts: make(map[string]*models.Host),
	}
}

func (t *HostTracker) Process(p capture.ParsedPacket, nodeID string) {
	now := time.Now()
	srcIP := p.SrcIP.String()
	dstIP := p.DstIP.String()

	t.mu.Lock()
	defer t.mu.Unlock()

	if srcIP != "" && srcIP != "<nil>" {
		t.updateHost(srcIP, p.SrcMAC.String(), nodeID, now, true, uint64(p.Length))
	}
	if dstIP != "" && dstIP != "<nil>" && dstIP != srcIP {
		t.updateHost(dstIP, p.DstMAC.String(), nodeID, now, false, uint64(p.Length))
	}
}

func (t *HostTracker) updateHost(ip, mac, nodeID string, now time.Time, isSent bool, bytes uint64) {
	h, ok := t.hosts[ip]
	if !ok {
		h = &models.Host{
			IP:        ip,
			MAC:       mac,
			Vendor:    lookupVendor(mac),
			Category:  models.HostCategory(ip),
			FirstSeen: now,
			NodeID:    nodeID,
		}
		t.hosts[ip] = h
	}
	h.LastSeen = now
	if mac != "" && h.MAC == "" {
		h.MAC = mac
		h.Vendor = lookupVendor(mac)
	}
	if isSent {
		h.BytesSent += bytes
		h.PacketsSent++
	} else {
		h.BytesReceived += bytes
		h.PacketsReceived++
	}
}

func (t *HostTracker) Snapshot() []models.Host {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]models.Host, 0, len(t.hosts))
	for _, h := range t.hosts {
		result = append(result, *h)
	}
	t.hosts = make(map[string]*models.Host)
	return result
}

func (t *HostTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.hosts)
}

func (t *HostTracker) GetHost(ip string) *models.Host {
	t.mu.RLock()
	defer t.mu.RUnlock()
	h, ok := t.hosts[ip]
	if !ok {
		return nil
	}
	copy := *h
	return &copy
}

func lookupVendor(mac string) string {
	if len(mac) < 8 {
		return ""
	}
	oui := mac[:8]
	v, ok := ouiDB[oui]
	if !ok {
		return ""
	}
	return v
}

var ouiDB = map[string]string{
	"00:50:56": "VMware",
	"00:0c:29": "VMware",
	"00:05:69": "VMware",
	"08:00:27": "Oracle VirtualBox",
	"00:1a:11": "Google",
	"3c:15:c2": "Apple",
	"00:1e:c2": "Apple",
	"00:25:00": "Apple",
	"a4:b1:e9": "Apple",
	"b8:27:eb": "Raspberry Pi",
	"dc:a6:32": "Raspberry Pi",
	"e4:5f:01": "Raspberry Pi",
	"00:e0:4c": "Realtek",
	"00:1b:21": "Intel",
	"00:1e:64": "Intel",
	"00:1f:3b": "Intel",
	"00:21:6a": "Intel",
	"00:24:d7": "Intel",
	"00:0c:f1": "Intel",
	"b8:ae:ed": "Dell",
	"00:14:22": "Dell",
	"00:1d:09": "Dell",
	"00:50:8b": "Compaq",
	"00:0e:7f": "Cisco",
	"00:1a:e2": "Cisco",
	"00:23:eb": "Cisco",
	"00:25:84": "Cisco",
	"6c:41:6a": "Cisco",
	"54:78:1a": "Cisco",
	"00:02:6c": "Cisco",
	"f4:4d:30": "Arista",
	"00:1c:73": "Arista",
	"00:1b:ed": "Blue Coat",
	"00:1c:df": "Belkin",
	"00:22:75": "Belkin",
	"e0:cb:4e": "D-Link",
	"00:21:91": "D-Link",
	"c8:d3:a3": "D-Link",
	"00:24:01": "D-Link",
	"00:1c:f0": "D-Link",
	"c0:a8:01": "D-Link",
	"08:00:09": "Hewlett Packard",
	"d8:9d:67": "HP",
	"00:1c:c4": "HP",
	"00:22:64": "HP",
	"28:92:4a": "HP",
	"00:22:19": "Juniper",
	"00:23:9c": "Juniper",
	"b0:c6:9a": "Juniper",
	"2c:21:72": "Juniper",
	"28:c0:da": "Juniper",
	"50:c5:8d": "Juniper",
	"00:1f:12": "NEC",
	"00:14:78": "Netgear",
	"00:1b:2f": "Netgear",
	"00:24:b2": "Netgear",
	"e0:46:9a": "Netgear",
	"fc:f8:ae": "Samsung",
	"00:1e:8c": "Samsung",
	"00:16:6b": "Samsung",
	"00:25:67": "Samsung",
	"f0:27:65": "Amazon",
	"00:1a:a0": "Amazon",
	"fc:65:de": "Amazon",
	"00:22:fa": "Intel",
	"00:15:5d": "Microsoft",
	"00:22:48": "Microsoft",
	"00:1d:d8": "Microsoft",
	"00:0d:3a": "Microsoft",
	"00:1c:25": "Huawei",
	"00:25:9e": "Huawei",
	"00:e0:fc": "Huawei",
	"48:8f:5a": "Huawei",
	"44:19:b6": "Huawei",
	"00:23:7a": "ZyXEL",
	"00:1d:0f": "TP-Link",
	"00:24:0b": "TP-Link",
	"e8:de:27": "TP-Link",
	"10:fe:ed": "TP-Link",
	"00:12:17": "Cisco-Linksys",
	"00:1e:e5": "Cisco-Linksys",
	"c0:56:27": "Linksys",
	"00:1f:c1": "Asus",
	"00:23:54": "Asus",
	"bc:ae:c5": "Asus",
	"00:1c:f2": "Synology",
	"00:11:32": "Synology",
	"00:0c:76": "QNAP",
	"00:08:9b": "QNAP",
	"00:24:8d": "QNAP",
	"00:07:e9": "Raritan",
	"00:1c:42": "Parallels",
	"00:25:ae": "Dell",
	"00:15:c5": "Dell",
	"00:24:e8": "Dell",
	"00:1e:c9": "Dell",
	"00:21:70": "Dell",
}
