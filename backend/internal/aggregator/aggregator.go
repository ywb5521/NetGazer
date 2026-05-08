package aggregator

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	netgazerv1 "github.com/netgazer/backend/gen/netgazer/v1"
	"github.com/netgazer/backend/internal/geoip"
	"github.com/netgazer/backend/internal/models"
	"github.com/netgazer/backend/internal/tracker"
)

type ifaceState struct {
	Name           string
	Snapshot       models.TrafficSnapshot
	Hosts          map[string]*models.Host
	Flows          map[string]*models.Flow
	Protocols      map[string]*models.ProtocolStat
	DnsQueries     []models.DNSQueryJSON
	PacketSizeDist *models.PacketSizeDistJSON
	TCPMetrics     *models.TCPMetricsJSON
	DNSLatency     *models.LatencyStatsJSON
	TLSLatency     *models.LatencyStatsJSON
	TCPLatency     *models.LatencyStatsJSON
	SNMPInOctets   uint64
	SNMPOutOctets  uint64
	SNMPPollTime   time.Time
}

type nodeState struct {
	NodeID       string
	Interfaces   []string
	Tags         []string
	Version      string
	Online       bool
	LastSeen     time.Time
	SystemHealth *models.SystemHealthJSON
	ifaces       map[string]*ifaceState
	mu           sync.RWMutex
}

func (ns *nodeState) getOrCreateIface(name string) *ifaceState {
	if is, ok := ns.ifaces[name]; ok {
		return is
	}
	is := &ifaceState{
		Name:      name,
		Hosts:     make(map[string]*models.Host),
		Flows:     make(map[string]*models.Flow),
		Protocols: make(map[string]*models.ProtocolStat),
	}
	ns.ifaces[name] = is
	return is
}

func (is *ifaceState) UpdateFrom(nodeID string, msg *netgazerv1.AgentMessage) {
	is.Snapshot = models.TrafficSnapshot{
		Timestamp:     time.UnixMilli(msg.TimestampUnixMs),
		BytesPerSec:   msg.Snapshot.BytesPerSec,
		PacketsPerSec: msg.Snapshot.PacketsPerSec,
		FlowsCount:    int(msg.Snapshot.FlowsCount),
		NodeID:        nodeID,
	}

	// Merge hosts — accumulate delta bytes into existing totals
	for _, h := range msg.Hosts {
		if existing, ok := is.Hosts[h.Ip]; ok {
			existing.BytesSent += h.BytesSent
			existing.BytesReceived += h.BytesReceived
			existing.PacketsSent += h.PacketsSent
			existing.PacketsReceived += h.PacketsReceived
			existing.LastSeen = time.UnixMilli(h.LastSeenUnixMs)
			existing.ActiveFlows = int(h.ActiveFlows)
			if h.Hostname != "" {
				existing.Hostname = h.Hostname
			}
			if h.Vendor != "" && existing.Vendor == "" {
				existing.Vendor = h.Vendor
			}
			if h.Mac != "" && existing.MAC == "" {
				existing.MAC = h.Mac
			}
			// Refresh country/ASN from GeoIP on each update (in case DB was loaded later)
			if c := geoip.Lookup(h.Ip); c != "" {
				existing.Country = c
			}
			if a := geoip.LookupASNString(h.Ip); a != "" {
				existing.ASN = a
			}
		} else {
			is.Hosts[h.Ip] = &models.Host{
				IP:              h.Ip,
				MAC:             h.Mac,
				Hostname:        h.Hostname,
				BytesSent:       h.BytesSent,
				BytesReceived:   h.BytesReceived,
				PacketsSent:     h.PacketsSent,
				PacketsReceived: h.PacketsReceived,
				FirstSeen:       time.UnixMilli(h.FirstSeenUnixMs),
				LastSeen:        time.UnixMilli(h.LastSeenUnixMs),
				Vendor:          h.Vendor,
				ActiveFlows:     int(h.ActiveFlows),
				NodeID:          nodeID,
				Interface:       is.Name,
				Country:         geoip.Lookup(h.Ip),
				ASN:             geoip.LookupASNString(h.Ip),
			}
		}
	}

	// Merge flows — accumulate delta bytes into existing flows, add new ones
	for _, f := range msg.Flows {
		if existing, ok := is.Flows[f.Id]; ok {
			existing.Bytes += f.Bytes
			existing.Packets += f.Packets
			existing.LastSeen = time.UnixMilli(f.LastSeenUnixMs)
			if f.AppProtocol != "" && f.AppProtocol != f.Protocol {
				existing.AppProtocol = f.AppProtocol
			}
		} else {
			is.Flows[f.Id] = &models.Flow{
				ID:          f.Id,
				SrcIP:       f.SrcIp,
				DstIP:       f.DstIp,
				SrcPort:     uint16(f.SrcPort),
				DstPort:     uint16(f.DstPort),
				Protocol:    f.Protocol,
				AppProtocol: f.AppProtocol,
				Bytes:       f.Bytes,
				Packets:     f.Packets,
				FirstSeen:   time.UnixMilli(f.FirstSeenUnixMs),
				LastSeen:    time.UnixMilli(f.LastSeenUnixMs),
				NodeID:      nodeID,
				Interface:   is.Name,
				VlanID:      uint16(f.VlanId),
			}
		}
	}

	// Merge protocols — accumulate delta bytes
	for _, p := range msg.Protocols {
		if existing, ok := is.Protocols[p.Protocol]; ok {
			existing.Bytes += p.Bytes
			existing.Packets += p.Packets
		} else {
			is.Protocols[p.Protocol] = &models.ProtocolStat{
				Protocol:   p.Protocol,
				Bytes:      p.Bytes,
				Packets:    p.Packets,
				Percentage: p.Percentage,
				NodeID:     nodeID,
				Interface:  is.Name,
			}
		}
	}

	// Merge DNS queries
	for _, q := range msg.DnsQueries {
		found := false
		for i := range is.DnsQueries {
			if is.DnsQueries[i].QueryName == q.QueryName {
				is.DnsQueries[i].Count += q.Count
				is.DnsQueries[i].Bytes += q.Bytes
				found = true
				break
			}
		}
		if !found {
			is.DnsQueries = append(is.DnsQueries, models.DNSQueryJSON{
				QueryName: q.QueryName,
				Count:     q.Count,
				Bytes:     q.Bytes,
			})
		}
	}

	// Merge packet size distribution
	if msg.PacketSizeDist != nil {
		if is.PacketSizeDist == nil {
			is.PacketSizeDist = &models.PacketSizeDistJSON{}
		}
		is.PacketSizeDist.Size64 += msg.PacketSizeDist.Size_64
		is.PacketSizeDist.Size128 += msg.PacketSizeDist.Size_128
		is.PacketSizeDist.Size256 += msg.PacketSizeDist.Size_256
		is.PacketSizeDist.Size512 += msg.PacketSizeDist.Size_512
		is.PacketSizeDist.Size1024 += msg.PacketSizeDist.Size_1024
		is.PacketSizeDist.Size1500 += msg.PacketSizeDist.Size_1500
		is.PacketSizeDist.SizeGt1500 += msg.PacketSizeDist.SizeGt1500
	}

	if msg.TcpMetrics != nil {
		is.TCPMetrics = &models.TCPMetricsJSON{
			ActiveTCPFlows:    int(msg.TcpMetrics.ActiveTcpFlows),
			TotalRetransmits:  msg.TcpMetrics.TotalRetransmits,
			TotalRSTs:         msg.TcpMetrics.TotalRsts,
			TotalZeroWindows:  msg.TcpMetrics.TotalZeroWindows,
			TotalOutOfOrder:   msg.TcpMetrics.TotalOutOfOrder,
			RTTAvgMS:          msg.TcpMetrics.RttAvgMs,
			RTTMinMS:          msg.TcpMetrics.RttMinMs,
			RTTMaxMS:          msg.TcpMetrics.RttMaxMs,
			RTTSamples:        msg.TcpMetrics.RttSamples,
			TotalExpectedPkts: msg.TcpMetrics.TotalExpectedPkts,
			TotalLostPkts:     msg.TcpMetrics.TotalLostPkts,
			PacketLossPct:     msg.TcpMetrics.PacketLossPct,
		}
	}
}

type Aggregator struct {
	mu          sync.RWMutex
	nodes       map[string]*nodeState
	bpfFilter   string
	voipTracker *voipTrackerHolder
}

type voipTrackerHolder struct {
	t *VoipTracker
}

// VoipTracker re-exports the tracker.VOIPTracker type.
type VoipTracker = tracker.VOIPTracker

func NewAggregator() *Aggregator {
	return &Aggregator{
		nodes:       make(map[string]*nodeState),
		voipTracker: &voipTrackerHolder{t: tracker.NewVOIPTracker()},
	}
}

func (a *Aggregator) GetBPFFilter() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.bpfFilter
}

func (a *Aggregator) SetBPFFilter(f string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bpfFilter = f
}

func (a *Aggregator) RegisterNode(nodeID string, ifaces []string, version string, tags []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if ns, ok := a.nodes[nodeID]; ok {
		ns.Online = true
		ns.Version = version
		ns.Tags = tags
		ns.LastSeen = time.Now()
		// Ensure all new interfaces have state
		ns.mu.Lock()
		for _, name := range ifaces {
			if !containsStr(ns.Interfaces, name) {
				ns.Interfaces = append(ns.Interfaces, name)
			}
			ns.getOrCreateIface(name)
		}
		ns.mu.Unlock()
		return
	}

	ns := &nodeState{
		NodeID:     nodeID,
		Interfaces: ifaces,
		Version:    version,
		Tags:       tags,
		Online:     true,
		LastSeen:   time.Now(),
		ifaces:     make(map[string]*ifaceState),
	}
	for _, name := range ifaces {
		ns.getOrCreateIface(name)
	}
	a.nodes[nodeID] = ns
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func (a *Aggregator) SetNodeOffline(nodeID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if ns, ok := a.nodes[nodeID]; ok {
		ns.Online = false
	}
}

func (a *Aggregator) Ingest(msg *netgazerv1.AgentMessage) {
	if len(msg.InterfaceSnapshots) > 0 {
		a.ingestInterfaceSnapshots(msg)
		return
	}

	iface := msg.Interface

	a.mu.Lock()
	ns, ok := a.nodes[msg.NodeId]
	if !ok {
		ns = &nodeState{
			NodeID: msg.NodeId,
			Online: true,
			ifaces: make(map[string]*ifaceState),
		}
		a.nodes[msg.NodeId] = ns
	}
	// Backward compat: if no interface in message, use first registered interface
	if iface == "" && len(ns.Interfaces) > 0 {
		iface = ns.Interfaces[0]
	}
	a.mu.Unlock()

	ns.mu.Lock()
	ns.LastSeen = time.Now()
	ns.Online = true
	is := ns.getOrCreateIface(iface)
	ns.mu.Unlock()

	is.UpdateFrom(msg.NodeId, msg)

	// System health (per-node, not per-interface)
	if msg.SystemHealth != nil {
		ns.mu.Lock()
		ns.SystemHealth = &models.SystemHealthJSON{
			CPUPercent:     msg.SystemHealth.CpuPercent,
			MemPercent:     msg.SystemHealth.MemPercent,
			MemUsedBytes:   msg.SystemHealth.MemUsedBytes,
			MemTotalBytes:  msg.SystemHealth.MemTotalBytes,
			DiskFreeBytes:  msg.SystemHealth.DiskFreeBytes,
			DiskTotalBytes: msg.SystemHealth.DiskTotalBytes,
			UptimeSeconds:  msg.SystemHealth.UptimeSeconds,
		}
		ns.mu.Unlock()
	}
}

func (a *Aggregator) ingestInterfaceSnapshots(msg *netgazerv1.AgentMessage) {
	a.mu.Lock()
	ns, ok := a.nodes[msg.NodeId]
	if !ok {
		ns = &nodeState{
			NodeID: msg.NodeId,
			Online: true,
			ifaces: make(map[string]*ifaceState),
		}
		a.nodes[msg.NodeId] = ns
	}
	a.mu.Unlock()

	ns.mu.Lock()
	ns.LastSeen = time.Now()
	ns.Online = true
	ns.mu.Unlock()

	for _, ifaceMsg := range msg.InterfaceSnapshots {
		iface := ifaceMsg.Interface
		if iface == "" && len(ns.Interfaces) > 0 {
			iface = ns.Interfaces[0]
		}
		ns.mu.Lock()
		is := ns.getOrCreateIface(iface)
		ns.mu.Unlock()

		is.UpdateFrom(msg.NodeId, &netgazerv1.AgentMessage{
			NodeId:          msg.NodeId,
			TimestampUnixMs: msg.TimestampUnixMs,
			Interface:       iface,
			Snapshot:        ifaceMsg.Snapshot,
			Hosts:           ifaceMsg.Hosts,
			Flows:           ifaceMsg.Flows,
			Protocols:       ifaceMsg.Protocols,
			DnsQueries:      ifaceMsg.DnsQueries,
			PacketSizeDist:  ifaceMsg.PacketSizeDist,
			TcpMetrics:      ifaceMsg.TcpMetrics,
			Latency:         ifaceMsg.Latency,
		})
	}

	if msg.SystemHealth != nil {
		ns.mu.Lock()
		ns.SystemHealth = &models.SystemHealthJSON{
			CPUPercent:     msg.SystemHealth.CpuPercent,
			MemPercent:     msg.SystemHealth.MemPercent,
			MemUsedBytes:   msg.SystemHealth.MemUsedBytes,
			MemTotalBytes:  msg.SystemHealth.MemTotalBytes,
			DiskFreeBytes:  msg.SystemHealth.DiskFreeBytes,
			DiskTotalBytes: msg.SystemHealth.DiskTotalBytes,
			UptimeSeconds:  msg.SystemHealth.UptimeSeconds,
		}
		ns.mu.Unlock()
	}
}

// IngestFlowRecords processes flow records from NetFlow/sFlow collectors.
// Each unique source IP becomes a virtual node.
func (a *Aggregator) IngestFlowRecords(records []FlowRecord) {
	type flowKey struct {
		nodeID, srcIP, dstIP string
		srcPort, dstPort     uint16
		protocol             string
	}

	now := time.Now()
	hostMap := make(map[string]map[string]*models.Host) // nodeID -> ip -> host
	flowMap := make(map[flowKey]*models.Flow)

	for _, r := range records {
		if r.NodeID == "" {
			continue
		}

		// Ensure virtual node exists
		a.mu.Lock()
		ns, ok := a.nodes[r.NodeID]
		if !ok {
			iface := "flow"
			ns = &nodeState{
				NodeID:     r.NodeID,
				Interfaces: []string{iface},
				Online:     true,
				Tags:       []string{"netflow"},
				Version:    "flow",
				ifaces:     make(map[string]*ifaceState),
			}
			ns.ifaces[iface] = &ifaceState{
				Name:  iface,
				Hosts: make(map[string]*models.Host),
				Flows: make(map[string]*models.Flow),
			}
			a.nodes[r.NodeID] = ns
		}
		a.mu.Unlock()

		ns.mu.Lock()
		ns.LastSeen = now
		ns.Online = true
		is := ns.getOrCreateIface("flow")
		// Update snapshot inline while holding ns.mu
		is.Snapshot.BytesPerSec += float64(r.Bytes)
		is.Snapshot.PacketsPerSec += float64(r.Packets)
		is.Snapshot.FlowsCount++
		ns.mu.Unlock()

		if _, ok := hostMap[r.NodeID]; !ok {
			hostMap[r.NodeID] = make(map[string]*models.Host)
		}

		// Track hosts (source)
		if r.SrcIP != nil && !r.SrcIP.IsUnspecified() {
			srcKey := r.SrcIP.String()
			if h, ok := hostMap[r.NodeID][srcKey]; ok {
				h.BytesSent += r.Bytes
				h.PacketsSent += uint64(r.Packets)
				h.LastSeen = now
				h.ActiveFlows++
			} else {
				hostMap[r.NodeID][srcKey] = &models.Host{
					IP:          srcKey,
					BytesSent:   r.Bytes,
					PacketsSent: uint64(r.Packets),
					FirstSeen:   now,
					LastSeen:    now,
					NodeID:      r.NodeID,
					ActiveFlows: 1,
				}
			}
		}

		// Track hosts (destination)
		if r.DstIP != nil && !r.DstIP.IsUnspecified() {
			dstKey := r.DstIP.String()
			if h, ok := hostMap[r.NodeID][dstKey]; ok {
				h.BytesReceived += r.Bytes
				h.PacketsReceived += uint64(r.Packets)
				h.LastSeen = now
			} else {
				hostMap[r.NodeID][dstKey] = &models.Host{
					IP:              dstKey,
					BytesReceived:   r.Bytes,
					PacketsReceived: uint64(r.Packets),
					FirstSeen:       now,
					LastSeen:        now,
					NodeID:          r.NodeID,
				}
			}
		}

		// Track flows
		fk := flowKey{
			nodeID:   r.NodeID,
			srcIP:    r.SrcIP.String(),
			dstIP:    r.DstIP.String(),
			srcPort:  r.SrcPort,
			dstPort:  r.DstPort,
			protocol: r.Protocol,
		}
		if f, ok := flowMap[fk]; ok {
			f.Bytes += r.Bytes
			f.Packets += r.Packets
			if now.After(f.LastSeen) {
				f.LastSeen = now
			}
		} else {
			id := fmt.Sprintf("%s:%d-%s:%d-%s", fk.srcIP, fk.srcPort, fk.dstIP, fk.dstPort, fk.protocol)
			flowMap[fk] = &models.Flow{
				ID:        id,
				SrcIP:     fk.srcIP,
				DstIP:     fk.dstIP,
				SrcPort:   fk.srcPort,
				DstPort:   fk.dstPort,
				Protocol:  fk.protocol,
				Bytes:     r.Bytes,
				Packets:   r.Packets,
				FirstSeen: now,
				LastSeen:  now,
				NodeID:    r.NodeID,
			}
		}

	}

	// Merge hosts into iface state
	for nodeID, hosts := range hostMap {
		a.mu.RLock()
		ns := a.nodes[nodeID]
		a.mu.RUnlock()
		if ns == nil {
			continue
		}
		ns.mu.Lock()
		is := ns.getOrCreateIface("flow")
		for ip, h := range hosts {
			if existing, ok := is.Hosts[ip]; ok {
				existing.BytesSent += h.BytesSent
				existing.BytesReceived += h.BytesReceived
				existing.PacketsSent += h.PacketsSent
				existing.PacketsReceived += h.PacketsReceived
				existing.LastSeen = h.LastSeen
				existing.ActiveFlows = h.ActiveFlows
			} else {
				is.Hosts[ip] = h
			}
		}
		ns.mu.Unlock()
	}

	// Merge flows into iface state
	for _, f := range flowMap {
		a.mu.RLock()
		ns := a.nodes[f.NodeID]
		a.mu.RUnlock()
		if ns == nil {
			continue
		}
		ns.mu.Lock()
		is := ns.getOrCreateIface("flow")
		if existing, ok := is.Flows[f.ID]; ok {
			existing.Bytes += f.Bytes
			existing.Packets += f.Packets
		} else {
			is.Flows[f.ID] = f
		}
		ns.mu.Unlock()
	}
}

// FlowRecord is a normalized flow record for the IngestFlowRecords method.
type FlowRecord struct {
	SrcIP    net.IP
	DstIP    net.IP
	SrcPort  uint16
	DstPort  uint16
	Protocol string
	Bytes    uint64
	Packets  uint64
	NodeID   string
}

// SNMPInterfaceSnapshot holds per-interface data from an SNMP poll.
type SNMPInterfaceSnapshot struct {
	Index      int
	Name       string
	Alias      string
	InOctets   uint64
	OutOctets  uint64
	InErrors   uint64
	OutErrors  uint64
	Speed      uint64
	OperStatus int
}

// SNMPDeviceSnapshot holds a complete SNMP device poll result.
type SNMPDeviceSnapshot struct {
	NodeID      string
	DisplayName string
	SysName     string
	SysDescr    string
	SysUptime   uint64
	Interfaces  []SNMPInterfaceSnapshot
	Timestamp   time.Time
}

// IngestSNMP processes an SNMP device poll and updates the aggregator.
func (a *Aggregator) IngestSNMP(snap SNMPDeviceSnapshot) {
	if snap.NodeID == "" {
		return
	}

	now := time.Now()

	a.mu.Lock()
	ns, ok := a.nodes[snap.NodeID]
	if !ok {
		ns = &nodeState{
			NodeID:  snap.NodeID,
			Tags:    []string{"snmp"},
			Version: "snmp",
			Online:  true,
			ifaces:  make(map[string]*ifaceState),
		}
		a.nodes[snap.NodeID] = ns
	}
	a.mu.Unlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.LastSeen = now
	ns.Online = true

	// Update system health from SNMP
	if snap.SysUptime > 0 {
		ns.SystemHealth = &models.SystemHealthJSON{
			UptimeSeconds: snap.SysUptime / 100, // SNMP uptime is in hundredths of seconds
		}
	}

	// Build list of interface names
	ifaceNames := make([]string, 0, len(snap.Interfaces))
	totalBytesPerSec := float64(0)
	totalPacketsPerSec := float64(0)

	for _, iface := range snap.Interfaces {
		name := iface.Name
		if name == "" {
			name = fmt.Sprintf("if%d", iface.Index)
		}
		ifaceNames = append(ifaceNames, name)

		is := ns.getOrCreateIface(name)

		// Calculate delta rates from previous poll
		inRate := float64(0)
		outRate := float64(0)
		if is.SNMPInOctets > 0 && iface.InOctets >= is.SNMPInOctets {
			elapsed := now.Sub(is.SNMPPollTime).Seconds()
			if elapsed > 0 {
				inRate = float64(iface.InOctets-is.SNMPInOctets) / elapsed
				outRate = float64(iface.OutOctets-is.SNMPOutOctets) / elapsed
			}
		}

		// Save current counters for next poll's delta calculation
		is.SNMPInOctets = iface.InOctets
		is.SNMPOutOctets = iface.OutOctets
		is.SNMPPollTime = now

		// Update snapshot with computed rates
		is.Snapshot = models.TrafficSnapshot{
			Timestamp:     now,
			BytesPerSec:   inRate + outRate,
			PacketsPerSec: 0,
			FlowsCount:    0,
			NodeID:        snap.NodeID,
		}

		totalBytesPerSec += inRate + outRate
		totalPacketsPerSec += 0
	}

	ns.Interfaces = ifaceNames
}

func (a *Aggregator) GetFirstInterface(nodeID string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if ns, ok := a.nodes[nodeID]; ok && len(ns.Interfaces) > 0 {
		return ns.Interfaces[0]
	}
	return ""
}

// TopN limits for WebSocket snapshot (keeps payload small for live dashboard)
const (
	wsMaxHosts = 100
	wsMaxFlows = 100
	wsMaxDNS   = 30
)

func (a *Aggregator) GlobalSnapshot() *models.GlobalSnapshot {
	gs := &models.GlobalSnapshot{
		Nodes:     make([]models.NodeInfo, 0),
		Hosts:     make([]models.HostJSON, 0),
		Flows:     make([]models.FlowJSON, 0),
		Protocols: make([]models.ProtocolStat, 0),
		Alerts:    make([]models.AlertJSON, 0),
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	var totalBytes, totalPackets float64
	globalProtocols := make(map[string]*models.ProtocolStat)
	globalDNS := make(map[string]*models.DNSQueryJSON)
	var globalPSD models.PacketSizeDistJSON
	var globalTCP *models.TCPMetricsJSON
	flowIDs := make(map[string]bool)
	allHosts := make([]models.HostJSON, 0)
	allFlows := make([]models.FlowJSON, 0)

	for _, ns := range a.nodes {
		ns.mu.RLock()

		// Aggregate per-node stats from all interfaces
		var nodeBytes, nodePackets float64
		totalHosts := 0
		totalFlows := 0
		var ifaceInfos []models.InterfaceInfo

		for _, is := range ns.ifaces {
			nodeBytes += is.Snapshot.BytesPerSec
			nodePackets += is.Snapshot.PacketsPerSec
			totalHosts += len(is.Hosts)
			totalFlows += len(is.Flows)

			ifaceInfos = append(ifaceInfos, models.InterfaceInfo{
				Name:          is.Name,
				BytesPerSec:   is.Snapshot.BytesPerSec,
				PacketsPerSec: is.Snapshot.PacketsPerSec,
				HostsCount:    len(is.Hosts),
				FlowsCount:    len(is.Flows),
			})

			if ns.Online {
				for _, host := range is.Hosts {
					allHosts = append(allHosts, host.ToJSON())
				}

				for _, flow := range is.Flows {
					fj := flow.ToJSON()
					if !flowIDs[fj.ID] {
						flowIDs[fj.ID] = true
						allFlows = append(allFlows, fj)
					}
				}

				for _, proto := range is.Protocols {
					key := proto.Protocol
					if existing, ok := globalProtocols[key]; ok {
						existing.Bytes += proto.Bytes
						existing.Packets += proto.Packets
					} else {
						globalProtocols[key] = &models.ProtocolStat{
							Protocol: proto.Protocol,
							Bytes:    proto.Bytes,
							Packets:  proto.Packets,
						}
					}
				}

				for _, q := range is.DnsQueries {
					if existing, ok := globalDNS[q.QueryName]; ok {
						existing.Count += q.Count
						existing.Bytes += q.Bytes
					} else {
						globalDNS[q.QueryName] = &models.DNSQueryJSON{
							QueryName: q.QueryName,
							Count:     q.Count,
							Bytes:     q.Bytes,
						}
					}
				}

				if is.PacketSizeDist != nil {
					globalPSD.Size64 += is.PacketSizeDist.Size64
					globalPSD.Size128 += is.PacketSizeDist.Size128
					globalPSD.Size256 += is.PacketSizeDist.Size256
					globalPSD.Size512 += is.PacketSizeDist.Size512
					globalPSD.Size1024 += is.PacketSizeDist.Size1024
					globalPSD.Size1500 += is.PacketSizeDist.Size1500
					globalPSD.SizeGt1500 += is.PacketSizeDist.SizeGt1500
				}
			}

			if is.TCPMetrics != nil {
				if globalTCP == nil {
					globalTCP = &models.TCPMetricsJSON{}
				}
				globalTCP.ActiveTCPFlows += is.TCPMetrics.ActiveTCPFlows
				globalTCP.TotalRetransmits += is.TCPMetrics.TotalRetransmits
				globalTCP.TotalRSTs += is.TCPMetrics.TotalRSTs
				globalTCP.TotalZeroWindows += is.TCPMetrics.TotalZeroWindows
				globalTCP.TotalOutOfOrder += is.TCPMetrics.TotalOutOfOrder
				globalTCP.TotalExpectedPkts += is.TCPMetrics.TotalExpectedPkts
				globalTCP.TotalLostPkts += is.TCPMetrics.TotalLostPkts
				globalTCP.PacketLossPct = is.TCPMetrics.PacketLossPct
				globalTCP.RTTSamples += is.TCPMetrics.RTTSamples
				if is.TCPMetrics.RTTMinMS > 0 && (globalTCP.RTTMinMS == 0 || is.TCPMetrics.RTTMinMS < globalTCP.RTTMinMS) {
					globalTCP.RTTMinMS = is.TCPMetrics.RTTMinMS
				}
				if is.TCPMetrics.RTTMaxMS > globalTCP.RTTMaxMS {
					globalTCP.RTTMaxMS = is.TCPMetrics.RTTMaxMS
				}
				if globalTCP.TotalExpectedPkts > 0 {
					globalTCP.PacketLossPct = float64(globalTCP.TotalLostPkts) / float64(globalTCP.TotalExpectedPkts) * 100
				}
				if is.TCPMetrics.RTTAvgMS > 0 && is.TCPMetrics.RTTSamples > 0 {
					globalTCP.RTTAvgMS = (globalTCP.RTTAvgMS*float64(globalTCP.RTTSamples-is.TCPMetrics.RTTSamples) + is.TCPMetrics.RTTAvgMS*float64(is.TCPMetrics.RTTSamples)) / float64(globalTCP.RTTSamples)
				}
			}
		}

		firstIface := ""
		if len(ns.Interfaces) > 0 {
			firstIface = ns.Interfaces[0]
		}
		ni := models.NodeInfo{
			NodeID:        ns.NodeID,
			Interface:     firstIface,
			Interfaces:    ns.Interfaces,
			InterfaceInfo: ifaceInfos,
			Tags:          ns.Tags,
			Online:        ns.Online,
			BytesPerSec:   nodeBytes,
			PacketsPerSec: nodePackets,
			HostsCount:    totalHosts,
			FlowsCount:    totalFlows,
			LastSeen:      ns.LastSeen.UnixMilli(),
			Version:       ns.Version,
			SystemHealth:  ns.SystemHealth,
			VOIPStats:     a.voipStatsJSON(),
		}
		gs.Nodes = append(gs.Nodes, ni)

		if ns.Online {
			totalBytes += nodeBytes
			totalPackets += nodePackets
		}

		ns.mu.RUnlock()
	}

	// Sort hosts by total bytes descending, keep top N
	sort.Slice(allHosts, func(i, j int) bool {
		return (allHosts[i].BytesSent + allHosts[i].BytesReceived) > (allHosts[j].BytesSent + allHosts[j].BytesReceived)
	})
	if len(allHosts) > wsMaxHosts {
		allHosts = allHosts[:wsMaxHosts]
	}
	gs.Hosts = allHosts

	// Sort flows by bytes descending, keep top N
	sort.Slice(allFlows, func(i, j int) bool {
		return allFlows[i].Bytes > allFlows[j].Bytes
	})
	if len(allFlows) > wsMaxFlows {
		allFlows = allFlows[:wsMaxFlows]
	}
	gs.Flows = allFlows

	// Calculate percentages for protocols
	var totalProtoBytes uint64
	for _, p := range globalProtocols {
		totalProtoBytes += p.Bytes
	}
	for _, p := range globalProtocols {
		if totalProtoBytes > 0 {
			p.Percentage = float64(p.Bytes) / float64(totalProtoBytes) * 100
		}
		gs.Protocols = append(gs.Protocols, *p)
	}
	sort.Slice(gs.Protocols, func(i, j int) bool {
		return gs.Protocols[i].Bytes > gs.Protocols[j].Bytes
	})

	gs.Traffic = models.TrafficSnapshot{
		Timestamp:     time.Now(),
		BytesPerSec:   totalBytes,
		PacketsPerSec: totalPackets,
		FlowsCount:    len(allFlows),
	}

	sorted := make([]*models.DNSQueryJSON, 0, len(globalDNS))
	for _, q := range globalDNS {
		sorted = append(sorted, q)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})
	if len(sorted) > wsMaxDNS {
		sorted = sorted[:wsMaxDNS]
	}
	for _, q := range sorted {
		gs.DnsQueries = append(gs.DnsQueries, *q)
	}

	hasPSD := globalPSD.Size64 > 0 || globalPSD.Size128 > 0 || globalPSD.Size256 > 0 ||
		globalPSD.Size512 > 0 || globalPSD.Size1024 > 0 || globalPSD.Size1500 > 0 || globalPSD.SizeGt1500 > 0
	if hasPSD {
		psd := globalPSD
		gs.PacketSizeDist = &psd
	}

	return gs
}

// PaginatedHosts returns hosts with pagination/sorting from the full in-memory dataset.
func (a *Aggregator) PaginatedHosts(nodeID, iface, search, country, asn string, limit, offset int, sortBy string) ([]models.HostJSON, int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var all []models.HostJSON
	for _, ns := range a.nodes {
		if nodeID != "" && ns.NodeID != nodeID {
			continue
		}
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			if iface != "" && is.Name != iface {
				continue
			}
			for _, host := range is.Hosts {
				if search != "" {
					q := strings.ToLower(search)
					if !strings.Contains(strings.ToLower(host.IP), q) &&
						!strings.Contains(strings.ToLower(host.Hostname), q) &&
						!strings.Contains(strings.ToLower(host.MAC), q) {
						continue
					}
				}
				if country != "" {
					if !strings.EqualFold(host.Country, country) {
						continue
					}
				}
				if asn != "" {
					if !strings.Contains(strings.ToLower(host.ASN), strings.ToLower(asn)) {
						continue
					}
				}
				all = append(all, host.ToJSON())
			}
		}
		ns.mu.RUnlock()
	}

	total := len(all)

	// Sort
	switch sortBy {
	case "bytes-asc":
		sort.Slice(all, func(i, j int) bool {
			return (all[i].BytesSent + all[i].BytesReceived) < (all[j].BytesSent + all[j].BytesReceived)
		})
	case "packets-desc":
		sort.Slice(all, func(i, j int) bool {
			return (all[i].PacketsSent + all[i].PacketsReceived) > (all[j].PacketsSent + all[j].PacketsReceived)
		})
	default: // "bytes-desc"
		sort.Slice(all, func(i, j int) bool {
			return (all[i].BytesSent + all[i].BytesReceived) > (all[j].BytesSent + all[j].BytesReceived)
		})
	}

	// Paginate
	if offset >= len(all) {
		return []models.HostJSON{}, total
	}
	all = all[offset:]
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}

	return all, total
}

// PaginatedFlows returns flows with pagination/sorting from the full in-memory dataset.
func (a *Aggregator) PaginatedFlows(nodeID, iface, search, protoFilter, appFilter string, limit, offset int, sortBy string) ([]models.FlowJSON, int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	seen := make(map[string]bool)
	var all []models.FlowJSON
	for _, ns := range a.nodes {
		if nodeID != "" && ns.NodeID != nodeID {
			continue
		}
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			if iface != "" && is.Name != iface {
				continue
			}
			for _, flow := range is.Flows {
				fj := flow.ToJSON()
				if seen[fj.ID] {
					continue
				}
				seen[fj.ID] = true

				if search != "" {
					q := strings.ToLower(search)
					if !strings.Contains(strings.ToLower(fj.SrcIP), q) &&
						!strings.Contains(strings.ToLower(fj.DstIP), q) &&
						!strings.Contains(strings.ToLower(fmtPort(fj.SrcPort)), q) &&
						!strings.Contains(strings.ToLower(fmtPort(fj.DstPort)), q) &&
						!strings.Contains(strings.ToLower(fj.Protocol), q) &&
						!strings.Contains(strings.ToLower(fj.AppProtocol), q) {
						continue
					}
				}
				if protoFilter != "" && protoFilter != "all" && fj.Protocol != protoFilter {
					continue
				}
				if appFilter != "" && appFilter != "all" && fj.AppProtocol != appFilter {
					continue
				}
				all = append(all, fj)
			}
		}
		ns.mu.RUnlock()
	}

	total := len(all)

	// Sort
	switch sortBy {
	case "bytes-asc":
		sort.Slice(all, func(i, j int) bool { return all[i].Bytes < all[j].Bytes })
	case "packets-desc":
		sort.Slice(all, func(i, j int) bool { return all[i].Packets > all[j].Packets })
	case "newest", "recent-active":
		sort.Slice(all, func(i, j int) bool { return all[i].LastSeen > all[j].LastSeen })
	default: // "bytes-desc"
		sort.Slice(all, func(i, j int) bool { return all[i].Bytes > all[j].Bytes })
	}

	// Paginate
	if offset >= len(all) {
		return []models.FlowJSON{}, total
	}
	all = all[offset:]
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}

	return all, total
}

func fmtPort(p uint16) string {
	if p == 0 {
		return ""
	}
	s := ""
	for p > 0 {
		s = string(byte('0'+p%10)) + s
		p /= 10
	}
	return s
}

func (a *Aggregator) PaginatedProtocols(nodeID, iface string, limit, offset int) ([]models.ProtocolStat, int) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	protoMap := make(map[string]*models.ProtocolStat)
	for _, ns := range a.nodes {
		if nodeID != "" && ns.NodeID != nodeID {
			continue
		}
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			if iface != "" && is.Name != iface {
				continue
			}
			for _, proto := range is.Protocols {
				if existing, ok := protoMap[proto.Protocol]; ok {
					existing.Bytes += proto.Bytes
					existing.Packets += proto.Packets
				} else {
					protoMap[proto.Protocol] = &models.ProtocolStat{
						Protocol: proto.Protocol,
						Bytes:    proto.Bytes,
						Packets:  proto.Packets,
					}
				}
			}
		}
		ns.mu.RUnlock()
	}

	var total uint64
	for _, p := range protoMap {
		total += p.Bytes
	}

	all := make([]models.ProtocolStat, 0, len(protoMap))
	for _, p := range protoMap {
		if total > 0 {
			p.Percentage = float64(p.Bytes) / float64(total) * 100
		}
		all = append(all, *p)
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Bytes > all[j].Bytes })

	totalCount := len(all)
	if offset >= len(all) {
		return []models.ProtocolStat{}, totalCount
	}
	all = all[offset:]
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}

	return all, totalCount
}

func (a *Aggregator) GetNodeStates() []models.NodeInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]models.NodeInfo, 0, len(a.nodes))
	for _, ns := range a.nodes {
		ns.mu.RLock()
		firstIface := ""
		if len(ns.Interfaces) > 0 {
			firstIface = ns.Interfaces[0]
		}
		var nodeBytes, nodePackets float64
		totalHosts := 0
		totalFlows := 0
		var ifaceInfos []models.InterfaceInfo
		for _, is := range ns.ifaces {
			nodeBytes += is.Snapshot.BytesPerSec
			nodePackets += is.Snapshot.PacketsPerSec
			totalHosts += len(is.Hosts)
			totalFlows += len(is.Flows)
			ifaceInfos = append(ifaceInfos, models.InterfaceInfo{
				Name:          is.Name,
				BytesPerSec:   is.Snapshot.BytesPerSec,
				PacketsPerSec: is.Snapshot.PacketsPerSec,
				HostsCount:    len(is.Hosts),
				FlowsCount:    len(is.Flows),
			})
		}
		ni := models.NodeInfo{
			NodeID:        ns.NodeID,
			Interface:     firstIface,
			Interfaces:    ns.Interfaces,
			InterfaceInfo: ifaceInfos,
			Tags:          ns.Tags,
			Online:        ns.Online,
			BytesPerSec:   nodeBytes,
			PacketsPerSec: nodePackets,
			HostsCount:    totalHosts,
			FlowsCount:    totalFlows,
			LastSeen:      ns.LastSeen.UnixMilli(),
			Version:       ns.Version,
			SystemHealth:  ns.SystemHealth,
		}
		ns.mu.RUnlock()
		result = append(result, ni)
	}
	return result
}

func (a *Aggregator) HostProtocols(nodeID, ip string) []models.ProtocolStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	protoMap := make(map[string]*models.ProtocolStat)
	for _, ns := range a.nodes {
		if nodeID != "" && ns.NodeID != nodeID {
			continue
		}
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			for _, f := range is.Flows {
				if f.SrcIP != ip && f.DstIP != ip {
					continue
				}
				key := f.AppProtocol
				if key == "" {
					key = f.Protocol
				}
				if existing, ok := protoMap[key]; ok {
					existing.Bytes += f.Bytes
					existing.Packets += f.Packets
				} else {
					protoMap[key] = &models.ProtocolStat{
						Protocol: key,
						Bytes:    f.Bytes,
						Packets:  f.Packets,
					}
				}
			}
		}
		ns.mu.RUnlock()
	}

	var totalBytes uint64
	result := make([]models.ProtocolStat, 0, len(protoMap))
	for _, p := range protoMap {
		totalBytes += p.Bytes
		result = append(result, *p)
	}
	for i := range result {
		if totalBytes > 0 {
			result[i].Percentage = float64(result[i].Bytes) / float64(totalBytes) * 100
		}
	}

	return result
}

func (a *Aggregator) HostPeers(nodeID, ip string) []models.HostPeer {
	a.mu.RLock()
	defer a.mu.RUnlock()

	peerMap := make(map[string]*models.HostPeer)
	for _, ns := range a.nodes {
		if nodeID != "" && ns.NodeID != nodeID {
			continue
		}
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			for _, f := range is.Flows {
				var peerIP string
				if f.SrcIP == ip {
					peerIP = f.DstIP
				} else if f.DstIP == ip {
					peerIP = f.SrcIP
				} else {
					continue
				}
				if existing, ok := peerMap[peerIP]; ok {
					existing.Bytes += f.Bytes
					existing.Packets += f.Packets
					existing.FlowCount++
				} else {
					peerMap[peerIP] = &models.HostPeer{
						PeerIP:    peerIP,
						Bytes:     f.Bytes,
						Packets:   f.Packets,
						FlowCount: 1,
					}
				}
			}
		}
		ns.mu.RUnlock()
	}

	result := make([]models.HostPeer, 0, len(peerMap))
	for _, p := range peerMap {
		result = append(result, *p)
	}
	return result
}

func (a *Aggregator) CheckNodeTimeouts(timeout time.Duration) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, ns := range a.nodes {
		ns.mu.RLock()
		if ns.Online && time.Since(ns.LastSeen) > timeout {
			ns.mu.RUnlock()
			ns.mu.Lock()
			ns.Online = false
			ns.mu.Unlock()
			continue
		}
		ns.mu.RUnlock()
	}
}

func (a *Aggregator) TrafficMatrix(nodeID string, limit int) []models.TrafficMatrixCell {
	if limit <= 0 {
		limit = 20
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	type pair struct{ src, dst string }
	matrix := make(map[pair]uint64)
	hostTraffic := make(map[string]uint64)

	for _, ns := range a.nodes {
		if nodeID != "" && ns.NodeID != nodeID {
			continue
		}
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			for _, f := range is.Flows {
				p := pair{f.SrcIP, f.DstIP}
				matrix[p] += f.Bytes
				hostTraffic[f.SrcIP] += f.Bytes
				hostTraffic[f.DstIP] += f.Bytes
			}
		}
		ns.mu.RUnlock()
	}

	type hostTotal struct {
		ip    string
		total uint64
	}
	all := make([]hostTotal, 0, len(hostTraffic))
	for ip, t := range hostTraffic {
		all = append(all, hostTotal{ip, t})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].total > all[j].total })
	if len(all) > limit {
		all = all[:limit]
	}
	topSet := make(map[string]bool, len(all))
	for _, h := range all {
		topSet[h.ip] = true
	}

	result := make([]models.TrafficMatrixCell, 0)
	for p, bytes := range matrix {
		if topSet[p.src] && topSet[p.dst] {
			result = append(result, models.TrafficMatrixCell{
				Source:      p.src,
				Destination: p.dst,
				Bytes:       bytes,
			})
		}
	}
	return result
}

// UpsertHost adds or updates a host in the aggregator, creating the node and
// interface if they don't exist. Used by network discovery and external data sources.
func (a *Aggregator) UpsertHost(host *models.Host) {
	nodeID := host.NodeID
	if nodeID == "" {
		nodeID = "discovery"
	}

	a.mu.Lock()
	ns, ok := a.nodes[nodeID]
	if !ok {
		ns = &nodeState{
			NodeID:     nodeID,
			Interfaces: []string{"discovery"},
			Tags:       []string{"discovery"},
			Online:     true,
			LastSeen:   time.Now(),
			ifaces:     make(map[string]*ifaceState),
		}
		ns.getOrCreateIface("discovery")
		a.nodes[nodeID] = ns
	}
	a.mu.Unlock()

	ns.mu.Lock()
	defer ns.mu.Unlock()

	is := ns.getOrCreateIface("discovery")
	if existing, ok := is.Hosts[host.IP]; ok {
		existing.LastSeen = host.LastSeen
		if host.MAC != "" && host.MAC != "00:00:00:00:00:00" {
			existing.MAC = host.MAC
		}
		if host.Hostname != "" {
			existing.Hostname = host.Hostname
		}
		if host.Vendor != "" && existing.Vendor == "" {
			existing.Vendor = host.Vendor
		}
	} else {
		host.FirstSeen = host.LastSeen
		is.Hosts[host.IP] = host
	}
	ns.LastSeen = time.Now()
}

// IngestRTP feeds an RTP packet into the VoIP tracker for quality analysis.
func (a *Aggregator) IngestRTP(info tracker.RTPPacketInfo) {
	a.voipTracker.t.Process(info)
}

// VOIPSessions returns all current VoIP sessions with quality metrics.
func (a *Aggregator) VOIPSessions() []tracker.VOIPSessionJSON {
	sessions, _ := a.voipTracker.t.Snapshot()
	return sessions
}

func (a *Aggregator) voipStatsJSON() *models.VOIPStatsJSON {
	_, stats := a.voipTracker.t.Snapshot()
	return &models.VOIPStatsJSON{
		ActiveSessions: stats.ActiveSessions,
		TotalSessions:  stats.TotalSessions,
		TotalPackets:   stats.TotalPackets,
		TotalBytes:     stats.TotalBytes,
		TotalLost:      stats.TotalLost,
		AvgJitterMS:    stats.AvgJitterMS,
		MinMOS:         stats.MinMOS,
		AvgMOS:         stats.AvgMOS,
	}
}

// CleanupVOIPTracker marks old sessions as inactive.
func (a *Aggregator) CleanupVOIPTracker(timeout time.Duration) {
	a.voipTracker.t.MarkInactive(timeout)
}

// CountryStat holds aggregated traffic stats per country.
type CountryStat struct {
	Country    string  `json:"country"`
	ISOCode    string  `json:"iso_code"`
	Bytes      uint64  `json:"bytes"`
	Packets    uint64  `json:"packets"`
	Hosts      int     `json:"hosts"`
	Percentage float64 `json:"percentage"`
}

// CountryStats returns traffic stats aggregated by country from all hosts.
func (a *Aggregator) CountryStats() []CountryStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	type acc struct {
		bytes   uint64
		packets uint64
		hosts   map[string]bool
		isoCode string
	}
	countryMap := make(map[string]*acc)

	for _, ns := range a.nodes {
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			for _, h := range is.Hosts {
				name := h.Country
				if name == "" {
					name = "Unknown"
				}
				c, ok := countryMap[name]
				if !ok {
					c = &acc{hosts: make(map[string]bool)}
					countryMap[name] = c
				}
				c.bytes += h.BytesSent + h.BytesReceived
				c.packets += h.PacketsSent + h.PacketsReceived
				c.hosts[h.IP] = true
				if c.isoCode == "" {
					c.isoCode = geoip.Default().LookupISOCode(h.IP)
				}
			}
		}
		ns.mu.RUnlock()
	}

	var totalBytes uint64
	for _, c := range countryMap {
		totalBytes += c.bytes
	}

	stats := make([]CountryStat, 0, len(countryMap))
	for name, c := range countryMap {
		pct := 0.0
		if totalBytes > 0 {
			pct = float64(c.bytes) / float64(totalBytes) * 100
		}
		stats = append(stats, CountryStat{
			Country:    name,
			ISOCode:    c.isoCode,
			Bytes:      c.bytes,
			Packets:    c.packets,
			Hosts:      len(c.hosts),
			Percentage: pct,
		})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Bytes > stats[j].Bytes })
	return stats
}

// ASStat holds aggregated traffic stats per ASN.
type ASStat struct {
	ASN        string  `json:"asn"`
	ASNumber   uint32  `json:"as_number"`
	Org        string  `json:"org"`
	Bytes      uint64  `json:"bytes"`
	Packets    uint64  `json:"packets"`
	Hosts      int     `json:"hosts"`
	Percentage float64 `json:"percentage"`
}

// ASStats returns traffic stats aggregated by ASN from all hosts.
func (a *Aggregator) ASStats() []ASStat {
	a.mu.RLock()
	defer a.mu.RUnlock()

	type acc struct {
		bytes   uint64
		packets uint64
		hosts   map[string]bool
		asNum   uint32
		org     string
	}
	asMap := make(map[string]*acc)

	for _, ns := range a.nodes {
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			for _, h := range is.Hosts {
				key := h.ASN
				if key == "" {
					key = "Unknown"
				}
				c, ok := asMap[key]
				if !ok {
					c = &acc{hosts: make(map[string]bool)}
					// Parse ASN string like "AS15169 (Google LLC)"
					if info := geoip.LookupASN(h.IP); info != nil {
						c.asNum = info.ASNumber
						c.org = info.ASOrg
					}
					asMap[key] = c
				}
				c.bytes += h.BytesSent + h.BytesReceived
				c.packets += h.PacketsSent + h.PacketsReceived
				c.hosts[h.IP] = true
			}
		}
		ns.mu.RUnlock()
	}

	var totalBytes uint64
	for _, c := range asMap {
		totalBytes += c.bytes
	}

	stats := make([]ASStat, 0, len(asMap))
	for key, c := range asMap {
		pct := 0.0
		if totalBytes > 0 {
			pct = float64(c.bytes) / float64(totalBytes) * 100
		}
		asn := key
		if c.asNum > 0 && c.org != "" {
			asn = fmt.Sprintf("AS%d (%s)", c.asNum, c.org)
		}
		stats = append(stats, ASStat{
			ASN:        asn,
			ASNumber:   c.asNum,
			Org:        c.org,
			Bytes:      c.bytes,
			Packets:    c.packets,
			Hosts:      len(c.hosts),
			Percentage: pct,
		})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Bytes > stats[j].Bytes })
	return stats
}

// ServiceNode represents a service in the service map.
type ServiceNode struct {
	Name  string `json:"name"`
	Bytes uint64 `json:"bytes"`
	Hosts int    `json:"hosts"`
}

// ServiceEdge represents traffic between two services.
type ServiceEdge struct {
	Src   string `json:"src"`
	Dst   string `json:"dst"`
	Bytes uint64 `json:"bytes"`
}

// ServiceMap returns service-level communication graph based on current flows.
func (a *Aggregator) ServiceMap() ([]ServiceNode, []ServiceEdge) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	type svcAcc struct {
		bytes uint64
		hosts map[string]bool
	}
	svcMap := make(map[string]*svcAcc)
	edgeMap := make(map[string]uint64)

	for _, ns := range a.nodes {
		ns.mu.RLock()
		for _, is := range ns.ifaces {
			for _, f := range is.Flows {
				srcSvc := svcName(f.AppProtocol, f.SrcPort, f.DstPort)
				dstSvc := svcName(f.AppProtocol, f.SrcPort, f.DstPort)

				s, ok := svcMap[srcSvc]
				if !ok {
					s = &svcAcc{hosts: make(map[string]bool)}
					svcMap[srcSvc] = s
				}
				s.bytes += f.Bytes
				s.hosts[f.SrcIP] = true

				d, ok := svcMap[dstSvc]
				if !ok {
					d = &svcAcc{hosts: make(map[string]bool)}
					svcMap[dstSvc] = d
				}
				d.bytes += f.Bytes
				d.hosts[f.DstIP] = true

				if srcSvc != dstSvc {
					ek := srcSvc + "|" + dstSvc
					edgeMap[ek] += f.Bytes
				}
			}
		}
		ns.mu.RUnlock()
	}

	services := make([]ServiceNode, 0, len(svcMap))
	for name, c := range svcMap {
		services = append(services, ServiceNode{
			Name:  name,
			Bytes: c.bytes,
			Hosts: len(c.hosts),
		})
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Bytes > services[j].Bytes })

	edges := make([]ServiceEdge, 0, len(edgeMap))
	for ek, bytes := range edgeMap {
		parts := strings.SplitN(ek, "|", 2)
		if len(parts) == 2 {
			edges = append(edges, ServiceEdge{Src: parts[0], Dst: parts[1], Bytes: bytes})
		}
	}
	return services, edges
}

func svcName(appProto string, srcPort, dstPort uint16) string {
	if appProto != "" && appProto != "TCP" && appProto != "UDP" {
		base := baseProtoName(appProto)
		if base != "" {
			return base
		}
	}
	if dstPort == 443 || dstPort == 8443 {
		return "HTTPS"
	}
	if dstPort == 80 || dstPort == 8080 {
		return "HTTP"
	}
	if dstPort == 53 {
		return "DNS"
	}
	if dstPort == 22 {
		return "SSH"
	}
	if dstPort == 25 || dstPort == 465 || dstPort == 587 {
		return "SMTP"
	}
	if dstPort == 3306 {
		return "MySQL"
	}
	if dstPort == 5432 {
		return "PostgreSQL"
	}
	if dstPort == 6379 {
		return "Redis"
	}
	if dstPort == 3389 {
		return "RDP"
	}
	if srcPort <= 1024 && dstPort > 1024 {
		return fmt.Sprintf(":%d", dstPort)
	}
	return "Other"
}

func baseProtoName(appProto string) string {
	m := strings.IndexByte(appProto, '.')
	if m >= 0 {
		appProto = appProto[m+1:]
	}
	idx := strings.IndexByte(appProto, '(')
	if idx >= 0 {
		return strings.TrimSpace(appProto[:idx])
	}
	return appProto
}
