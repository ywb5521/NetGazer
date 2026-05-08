package alerting

import (
	"fmt"
	"sync"
	"time"

	"github.com/netgazer/backend/internal/models"
)

type portScanState struct {
	dstPorts map[uint16]bool
	lastSeen time.Time
}

type Engine struct {
	mu                    sync.Mutex
	alerts                []models.Alert
	outCh                 chan models.Alert
	bandwidthThresholdBps float64
	thresholds            AlertThresholds
	bannedPorts           map[uint16]bool
	dnsSuspiciousPorts    map[uint16]bool
	unexpectedProtos      map[string]bool
	suppressedTypes       map[models.AlertType]bool
	seenHosts             map[string]bool
	portScans             map[string]*portScanState
	// Horizontal scan state: "nodeID:srcIP:dstPort" -> set of dstIPs
	horizontalScans map[string]map[string]bool
	// ARP state: "srcIP" -> set of MACs
	arpState map[string]map[string]bool
	// Deduplication
	firedRecently   map[string]time.Time
	lastFireCleanup time.Time
}

func NewEngine(bandwidthThresholdBps float64, thresholds AlertThresholds) *Engine {
	e := &Engine{
		bandwidthThresholdBps: bandwidthThresholdBps,
		thresholds:            thresholds,
		seenHosts:             make(map[string]bool),
		portScans:             make(map[string]*portScanState),
		horizontalScans:       make(map[string]map[string]bool),
		arpState:              make(map[string]map[string]bool),
		firedRecently:         make(map[string]time.Time),
		outCh:                 make(chan models.Alert, 64),
	}
	e.rebuildMaps()

	// Background cleanup: prune stale alerts every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			e.mu.Lock()
			e.removeStaleAlerts()
			// Also cleanup firedRecently
			cutoff := time.Now().Add(-10 * time.Minute)
			for k, t := range e.firedRecently {
				if t.Before(cutoff) {
					delete(e.firedRecently, k)
				}
			}
			e.mu.Unlock()
		}
	}()

	return e
}

func (e *Engine) rebuildMaps() {
	e.bannedPorts = make(map[uint16]bool)
	for _, p := range e.thresholds.BannedPorts {
		e.bannedPorts[p] = true
	}
	e.dnsSuspiciousPorts = make(map[uint16]bool)
	for _, p := range e.thresholds.DNSSuspiciousPorts {
		e.dnsSuspiciousPorts[p] = true
	}
	e.unexpectedProtos = make(map[string]bool)
	for _, p := range e.thresholds.UnexpectedProtocols {
		e.unexpectedProtos[p] = true
	}
	e.suppressedTypes = make(map[models.AlertType]bool)
	for _, t := range e.thresholds.SuppressedAlertTypes {
		e.suppressedTypes[models.AlertType(t)] = true
	}
}

func (e *Engine) SetBandwidthThreshold(bps float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bandwidthThresholdBps = bps
}

func (e *Engine) SetThresholds(t AlertThresholds) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.thresholds = t
	e.rebuildMaps()
}

func (e *Engine) GetThresholds() AlertThresholds {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.thresholds
}

func (e *Engine) AlertChannel() <-chan models.Alert {
	return e.outCh
}

// EmitFromLua creates and emits an alert from Lua script parameters.
func (e *Engine) EmitFromLua(severity, alertType, message string) *models.Alert {
	now := time.Now()
	alert := models.Alert{
		ID:        fmt.Sprintf("lua-%d", now.UnixMilli()),
		Type:      models.AlertType(alertType),
		Severity:  models.Severity(severity),
		Message:   message,
		Timestamp: now,
	}
	e.emit(alert)
	return &alert
}

func (e *Engine) shouldFire(fingerprint string) bool {
	last, ok := e.firedRecently[fingerprint]
	if !ok {
		return true
	}
	return time.Since(last) > time.Duration(e.thresholds.AlertCooldownMin)*time.Minute
}

func (e *Engine) recordFire(fingerprint string) {
	now := time.Now()
	e.firedRecently[fingerprint] = now
	// Cleanup firedRecently when it grows too large or periodically
	if len(e.firedRecently) > 2000 || now.Sub(e.lastFireCleanup) > 10*time.Minute {
		cutoff := now.Add(-10 * time.Minute)
		for k, t := range e.firedRecently {
			if t.Before(cutoff) {
				delete(e.firedRecently, k)
			}
		}
		e.lastFireCleanup = now
	}
}

func (e *Engine) emit(alert models.Alert) {
	if e.suppressedTypes[alert.Type] {
		return
	}
	e.alerts = append(e.alerts, alert)
	e.outCh <- alert
}

func (e *Engine) removeStaleAlerts() {
	ackCutoff := time.Now().Add(-10 * time.Minute)
	unackCutoff := time.Now().Add(-24 * time.Hour) // remove unacknowledged alerts after 24h
	maxAlerts := 10000                             // hard cap on in-memory alerts

	kept := make([]models.Alert, 0, len(e.alerts))
	for _, a := range e.alerts {
		if a.Acknowledged && a.Timestamp.Before(ackCutoff) {
			continue
		}
		if !a.Acknowledged && a.Timestamp.Before(unackCutoff) {
			continue
		}
		kept = append(kept, a)
	}

	// If still over cap, keep only the newest ones
	if len(kept) > maxAlerts {
		// Alerts are already in chronological order, keep the latest
		kept = kept[len(kept)-maxAlerts:]
	}

	e.alerts = kept
}

// ---- Existing checks ----

func (e *Engine) CheckHostBandwidth(hosts []models.Host, nodeID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	totalBW := float64(0)
	for _, h := range hosts {
		totalBW += float64(h.BytesSent+h.BytesReceived) / 10.0
	}

	if totalBW > e.bandwidthThresholdBps {
		fp := fmt.Sprintf("bw:%s", nodeID)
		if !e.shouldFire(fp) {
			return
		}
		e.recordFire(fp)
		e.removeStaleAlerts()
		e.emit(models.Alert{
			ID:        fmt.Sprintf("bw-%s", nodeID),
			Type:      models.AlertHighBandwidth,
			Severity:  models.SeverityWarning,
			Message:   fmt.Sprintf("节点 %s 带宽占用过高：%.1f Mbps", nodeID, totalBW/1_000_000),
			NodeID:    nodeID,
			Timestamp: time.Now(),
		})
	}
}

func (e *Engine) CheckNewHost(hosts []models.Host) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, h := range hosts {
		key := h.NodeID + ":" + h.IP
		if !e.seenHosts[key] {
			e.seenHosts[key] = true
			if len(e.seenHosts) > 10000 {
				e.seenHosts = make(map[string]bool)
			}
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("new-%s-%s", h.NodeID, h.IP),
				Type:      models.AlertNewDevice,
				Severity:  models.SeverityInfo,
				Message:   fmt.Sprintf("检测到新设备：%s (%s)，节点 %s", h.IP, h.MAC, h.NodeID),
				SourceIP:  h.IP,
				NodeID:    h.NodeID,
				Timestamp: time.Now(),
			})
		}
	}
}

func (e *Engine) CheckSuspiciousPort(flows []models.Flow) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, f := range flows {
		if e.bannedPorts[f.DstPort] {
			fp := fmt.Sprintf("port:%s:%d:%s", f.NodeID, f.DstPort, f.SrcIP)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("port-%s-%d-%s", f.NodeID, f.DstPort, f.SrcIP),
				Type:      models.AlertSuspiciousPort,
				Severity:  models.SeverityCritical,
				Message:   fmt.Sprintf("检测到访问可疑端口 %d：%s:%d → %s:%d，节点 %s", f.DstPort, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort, f.NodeID),
				SourceIP:  f.SrcIP,
				NodeID:    f.NodeID,
				Timestamp: time.Now(),
			})
		}
	}
}

func (e *Engine) GetAlerts() []models.Alert {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]models.Alert, len(e.alerts))
	copy(result, e.alerts)
	return result
}

func (e *Engine) Acknowledge(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, a := range e.alerts {
		if a.ID == id {
			e.alerts[i].Acknowledged = true
			return true
		}
	}
	return false
}

func (e *Engine) AlertCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.alerts)
}

func (e *Engine) CheckPortScan(flows []models.Flow) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for _, f := range flows {
		key := f.NodeID + ":" + f.SrcIP
		state, ok := e.portScans[key]
		if !ok {
			state = &portScanState{dstPorts: make(map[uint16]bool)}
			e.portScans[key] = state
		}
		state.dstPorts[f.DstPort] = true
		state.lastSeen = now
	}

	for key, state := range e.portScans {
		if now.Sub(state.lastSeen) > time.Duration(e.thresholds.PortScanWindowSec)*time.Second {
			delete(e.portScans, key)
			continue
		}
		if len(state.dstPorts) >= e.thresholds.PortScanThreshold {
			parts := splitNodeIP(key)
			fp := fmt.Sprintf("scan:%s:%s", parts[0], parts[1])
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("scan-%s-%s", parts[0], parts[1]),
				Type:      models.AlertPortScan,
				Severity:  models.SeverityWarning,
				Message:   fmt.Sprintf("疑似端口扫描：来源 %s，节点 %s，共扫描 %d 个端口", parts[1], parts[0], len(state.dstPorts)),
				SourceIP:  parts[1],
				NodeID:    parts[0],
				Timestamp: now,
			})
			delete(e.portScans, key)
		}
	}

	if len(e.portScans) > 5000 {
		for k := range e.portScans {
			delete(e.portScans, k)
			break
		}
	}
}

func (e *Engine) CheckDNSSuspiciousPort(flows []models.Flow) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, f := range flows {
		if f.AppProtocol != "DNS" {
			continue
		}
		if len(e.dnsSuspiciousPorts) > 0 {
			if !e.dnsSuspiciousPorts[f.DstPort] {
				continue
			}
		} else if f.DstPort == 53 {
			continue
		}
		fp := fmt.Sprintf("dnsport:%s:%d:%s", f.NodeID, f.DstPort, f.SrcIP)
		if !e.shouldFire(fp) {
			continue
		}
		e.recordFire(fp)
		e.removeStaleAlerts()
		e.emit(models.Alert{
			ID:        fmt.Sprintf("dnsport-%s-%d-%s", f.NodeID, f.DstPort, f.SrcIP),
			Type:      models.AlertDNSSuspiciousPort,
			Severity:  models.SeverityWarning,
			Message:   fmt.Sprintf("检测到非标准端口 DNS 流量 %d：%s:%d -> %s:%d，节点 %s", f.DstPort, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort, f.NodeID),
			SourceIP:  f.SrcIP,
			NodeID:    f.NodeID,
			Timestamp: time.Now(),
		})
	}
}

func (e *Engine) CheckFlowFlood(flows []models.Flow, nodeID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	srcCounts := make(map[string]int)
	for _, f := range flows {
		if f.NodeID == nodeID {
			srcCounts[f.SrcIP]++
		}
	}

	for srcIP, count := range srcCounts {
		if count > e.thresholds.FlowFloodThreshold {
			fp := fmt.Sprintf("flood:%s:%s", nodeID, srcIP)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("flood-%s-%s", nodeID, srcIP),
				Type:      models.AlertFlowFlood,
				Severity:  models.SeverityCritical,
				Message:   fmt.Sprintf("检测到流量洪泛：%s 在节点 %s 上存在 %d 条并发流", srcIP, count, nodeID),
				SourceIP:  srcIP,
				NodeID:    nodeID,
				Timestamp: time.Now(),
			})
		}
	}
}

// ---- Extended behavioral checks ----

// CheckDNSExfiltration detects DNS queries with unusually long names or excessive DNS traffic volume.
func (e *Engine) CheckDNSExfiltration(flows []models.Flow, dnsQueries []models.DNSQueryJSON) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, f := range flows {
		if f.AppProtocol != "DNS" {
			continue
		}
		fp := fmt.Sprintf("dnsx:%s:%s", f.NodeID, f.SrcIP)
		if !e.shouldFire(fp) {
			continue
		}
		// High-volume DNS traffic from a single flow (> MinBytes)
		if f.Bytes > e.thresholds.DNSExfilMinBytes && f.DstPort != 53 {
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("dnsx-%s-%s", f.NodeID, f.SrcIP),
				Type:      models.AlertDNSExfiltration,
				Severity:  models.SeverityCritical,
				Message:   fmt.Sprintf("疑似 DNS 外传：%s 通过 DNS（端口 %d）发送了 %d 字节，节点 %s", f.SrcIP, f.Bytes, f.DstPort, f.NodeID),
				SourceIP:  f.SrcIP,
				NodeID:    f.NodeID,
				Timestamp: time.Now(),
			})
		}
	}

	// Check for long DNS query names
	minLen := e.thresholds.DNSExfilQueryMinLen
	if minLen == 0 {
		minLen = 52
	}
	for _, q := range dnsQueries {
		if len(q.QueryName) > minLen {
			fp := fmt.Sprintf("dnslen:%s", q.QueryName)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("dnslen-%s", q.QueryName[:min(16, len(q.QueryName))]),
				Type:      models.AlertDNSExfiltration,
				Severity:  models.SeverityWarning,
				Message:   fmt.Sprintf("可疑的超长 DNS 查询：%s（%d 个字符，%d 次查询）", q.QueryName, len(q.QueryName), q.Count),
				Timestamp: time.Now(),
			})
		}
	}
}

// CheckICMPFlood detects excessive ICMP traffic from a single source.
func (e *Engine) CheckICMPFlood(flows []models.Flow, nodeID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	icmpBySrc := make(map[string]int)
	for _, f := range flows {
		if f.NodeID == nodeID && f.Protocol == "ICMP" {
			icmpBySrc[f.SrcIP]++
		}
	}

	threshold := e.thresholds.ICMPFloodThreshold
	if threshold == 0 {
		threshold = 50
	}
	for srcIP, count := range icmpBySrc {
		if count > threshold {
			fp := fmt.Sprintf("icmp:%s:%s", nodeID, srcIP)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("icmp-%s-%s", nodeID, srcIP),
				Type:      models.AlertICMPFlood,
				Severity:  models.SeverityWarning,
				Message:   fmt.Sprintf("检测到 ICMP 洪泛：%s 在节点 %s 上产生了 %d 条流", srcIP, count, nodeID),
				SourceIP:  srcIP,
				NodeID:    nodeID,
				Timestamp: time.Now(),
			})
		}
	}
}

// CheckSYNFlood detects possible SYN flood via pattern: many tiny TCP flows from one source to many ports.
func (e *Engine) CheckSYNFlood(flows []models.Flow, nodeID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	type srcStat struct {
		tinyFlows int
		destPorts map[uint16]bool
	}
	srcStats := make(map[string]*srcStat)
	for _, f := range flows {
		if f.NodeID != nodeID || f.Protocol != "TCP" {
			continue
		}
		ip := f.SrcIP
		if _, ok := srcStats[ip]; !ok {
			srcStats[ip] = &srcStat{destPorts: make(map[uint16]bool)}
		}
		s := srcStats[ip]
		s.destPorts[f.DstPort] = true
		if f.Packets <= 3 {
			s.tinyFlows++
		}
	}

	threshold := e.thresholds.FlowFloodThreshold
	if threshold == 0 {
		threshold = 100
	}
	for srcIP, s := range srcStats {
		if s.tinyFlows > threshold && len(s.destPorts) > 10 {
			fp := fmt.Sprintf("syn:%s:%s", nodeID, srcIP)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("syn-%s-%s", nodeID, srcIP),
				Type:      models.AlertSYNFlood,
				Severity:  models.SeverityCritical,
				Message:   fmt.Sprintf("疑似 SYN 洪泛：%s 在节点 %s 上向 %d 个端口发起了 %d 条小型 TCP 流", srcIP, s.tinyFlows, len(s.destPorts), nodeID),
				SourceIP:  srcIP,
				NodeID:    nodeID,
				Timestamp: time.Now(),
			})
		}
	}
}

// CheckHorizontalScan detects scanning of the same port across many destination IPs.
func (e *Engine) CheckHorizontalScan(flows []models.Flow) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	// Aggregate: "srcIP:nodeID:dstPort" -> set of dstIPs
	for _, f := range flows {
		key := fmt.Sprintf("%s:%s:%d", f.SrcIP, f.NodeID, f.DstPort)
		if _, ok := e.horizontalScans[key]; !ok {
			e.horizontalScans[key] = make(map[string]bool)
		}
		e.horizontalScans[key][f.DstIP] = true
	}

	threshold := e.thresholds.HorizontalScanThreshold
	if threshold == 0 {
		threshold = 20
	}

	for key, dstSet := range e.horizontalScans {
		if len(dstSet) >= threshold {
			parts := splitColons(key)
			if len(parts) < 2 {
				continue
			}
			srcIP, nodeID, port := parts[0], parts[1], parts[2]
			fp := fmt.Sprintf("hscan:%s:%s:%s", nodeID, srcIP, port)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("hscan-%s-%s-%s", nodeID, srcIP, port),
				Type:      models.AlertHorizontalScan,
				Severity:  models.SeverityWarning,
				Message:   fmt.Sprintf("水平扫描：%s 在节点 %s 上针对 %d 台主机探测了端口 %s", srcIP, nodeID, len(dstSet), port),
				SourceIP:  srcIP,
				NodeID:    nodeID,
				Timestamp: now,
			})
		}
	}

	// Cleanup old entries periodically
	if len(e.horizontalScans) > 1000 {
		e.horizontalScans = make(map[string]map[string]bool)
	}
}

// CheckDataExfiltration detects asymmetric traffic where outbound greatly exceeds inbound.
func (e *Engine) CheckDataExfiltration(hosts []models.Host, nodeID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ratio := e.thresholds.DataExfilRatio
	if ratio == 0 {
		ratio = 10.0
	}

	for _, h := range hosts {
		if h.BytesReceived > 0 && float64(h.BytesSent)/float64(h.BytesReceived) > ratio && h.BytesSent > 10_000_000 {
			fp := fmt.Sprintf("dx:%s:%s", nodeID, h.IP)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("dx-%s-%s", nodeID, h.IP),
				Type:      models.AlertDataExfiltration,
				Severity:  models.SeverityCritical,
				Message:   fmt.Sprintf("疑似数据外传：%s 在节点 %s 上发出 %d 字节 / 收入 %d 字节（比值 %.1f）", h.IP, nodeID, h.BytesSent, h.BytesReceived, float64(h.BytesSent)/float64(h.BytesReceived)),
				SourceIP:  h.IP,
				NodeID:    nodeID,
				Timestamp: time.Now(),
			})
		}
	}
}

// CheckUnexpectedProtocol detects protocols not in the expected whitelist.
func (e *Engine) CheckUnexpectedProtocol(flows []models.Flow) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.unexpectedProtos) == 0 {
		return
	}

	for _, f := range flows {
		proto := f.AppProtocol
		if proto == "" {
			proto = f.Protocol
		}
		if e.unexpectedProtos[proto] {
			continue
		}
		fp := fmt.Sprintf("uproto:%s:%s:%s", f.NodeID, f.SrcIP, proto)
		if !e.shouldFire(fp) {
			continue
		}
		e.recordFire(fp)
		e.removeStaleAlerts()
		e.emit(models.Alert{
			ID:        fmt.Sprintf("uproto-%s-%s-%s", f.NodeID, f.SrcIP, proto),
			Type:      models.AlertUnexpectedProtocol,
			Severity:  models.SeverityInfo,
			Message:   fmt.Sprintf("检测到异常协议 '%s'：来源 %s，节点 %s", proto, f.SrcIP, f.NodeID),
			SourceIP:  f.SrcIP,
			NodeID:    f.NodeID,
			Timestamp: time.Now(),
		})
	}
}

// CheckARPSpoof detects multiple MAC addresses claiming the same IP address.
func (e *Engine) CheckARPSpoof(flows []models.Flow, hosts []models.Host) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Track IP -> MAC mappings from hosts
	for _, h := range hosts {
		if h.MAC == "" || h.MAC == "00:00:00:00:00:00" {
			continue
		}
		if _, ok := e.arpState[h.IP]; !ok {
			e.arpState[h.IP] = make(map[string]bool)
		}
		e.arpState[h.IP][h.MAC] = true
	}

	threshold := e.thresholds.ARPSpoofThreshold
	if threshold == 0 {
		threshold = 2
	}

	for ip, macs := range e.arpState {
		if len(macs) >= threshold {
			fp := fmt.Sprintf("arp:%s", ip)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			macList := ""
			for m := range macs {
				macList += m + ", "
			}
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("arp-%s", ip),
				Type:      models.AlertARPSpoof,
				Severity:  models.SeverityCritical,
				Message:   fmt.Sprintf("疑似 ARP 欺骗：IP %s 对应了 %d 个 MAC 地址：%s", ip, len(macs), macList[:len(macList)-2]),
				SourceIP:  ip,
				Timestamp: time.Now(),
			})
		}
	}

	if len(e.arpState) > 5000 {
		e.arpState = make(map[string]map[string]bool)
	}
}

// CheckLongFlow detects flows that have been active for an unusually long time.
func (e *Engine) CheckLongFlow(flows []models.Flow) {
	e.mu.Lock()
	defer e.mu.Unlock()

	threshold := e.thresholds.LongFlowSeconds
	if threshold == 0 {
		threshold = 3600
	}

	now := time.Now()
	for _, f := range flows {
		duration := now.Sub(f.FirstSeen).Seconds()
		if duration > float64(threshold) {
			fp := fmt.Sprintf("long:%s:%s", f.NodeID, f.ID)
			if !e.shouldFire(fp) {
				continue
			}
			e.recordFire(fp)
			e.removeStaleAlerts()
			e.emit(models.Alert{
				ID:        fmt.Sprintf("long-%s-%s", f.NodeID, f.ID[:min(8, len(f.ID))]),
				Type:      models.AlertLongFlow,
				Severity:  models.SeverityInfo,
				Message:   fmt.Sprintf("长时间活跃流：%s:%d → %s:%d（%s）已持续 %.0f 秒，节点 %s", f.SrcIP, f.SrcPort, f.DstIP, f.DstPort, f.Protocol, duration, f.NodeID),
				SourceIP:  f.SrcIP,
				NodeID:    f.NodeID,
				Timestamp: now,
			})
		}
	}
}

// ---- Helpers ----

func splitNodeIP(key string) [2]string {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{key, ""}
}

func splitColons(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
