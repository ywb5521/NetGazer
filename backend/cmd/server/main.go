package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/netgazer/backend/internal/aggregator"
	"github.com/netgazer/backend/internal/alerting"
	"github.com/netgazer/backend/internal/api"
	"github.com/netgazer/backend/internal/collector"
	"github.com/netgazer/backend/internal/config"
	"github.com/netgazer/backend/internal/discovery"
	"github.com/netgazer/backend/internal/geoip"
	"github.com/netgazer/backend/internal/lua"
	"github.com/netgazer/backend/internal/models"
	"github.com/netgazer/backend/internal/receiver"
	"github.com/netgazer/backend/internal/snmp"
	"github.com/netgazer/backend/internal/storage"
	"github.com/netgazer/backend/internal/webhook"
)

func main() {
	cfg := config.ParseServerFlags()

	log.Printf("[server] netgazer-server starting")
	log.Printf("[server] gRPC port: %d | HTTP port: %d | DB: %s", cfg.GRPCPort, cfg.HTTPPort, cfg.DBPath)

	// Storage
	store, err := storage.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("[server] storage init failed: %v", err)
	}
	defer store.Close()
	log.Printf("[server] storage ready")

	// JWT secret persistence
	if persistedSecret, err := store.GetConfig("jwt_secret"); err == nil && persistedSecret != "" {
		cfg.JWTSecret = []byte(persistedSecret)
	} else {
		if err := store.SetConfig("jwt_secret", string(cfg.JWTSecret)); err != nil {
			log.Printf("[server] failed to persist JWT secret: %v", err)
		}
	}

	// Aggregator
	agg := aggregator.NewAggregator()

	// Alert engine
	thresholds := alerting.DefaultThresholds()
	if raw, err := store.GetConfig("alert_thresholds"); err == nil && raw != "" {
		var saved alerting.AlertThresholds
		if json.Unmarshal([]byte(raw), &saved) == nil {
			thresholds = saved
		}
	}
	if raw, err := store.GetConfig("bandwidth_threshold_bps"); err == nil && raw != "" {
		var savedBw float64
		if n, _ := fmt.Sscanf(raw, "%f", &savedBw); n >= 1 {
			cfg.BandwidthThresholdBps = savedBw
		}
	}
	// Load persisted BPF filter
	if raw, err := store.GetConfig("bpf_filter"); err == nil && raw != "" {
		agg.SetBPFFilter(raw)
		log.Printf("[server] loaded persisted BPF filter: %s", raw)
	}
	alertEng := alerting.NewEngine(cfg.BandwidthThresholdBps, thresholds)

	// Notification manager
	notifManager := webhook.NewManager()

	// Migrate legacy webhook URL
	if cfg.WebhookURL != "" {
		legacyID := "legacy-webhook"
		legacyCfg, _ := json.Marshal(map[string]string{"url": cfg.WebhookURL})
		legacyCh := models.NotificationChannel{
			ID: legacyID, Name: "Default Webhook",
			Type: models.ChannelGenericWebhook, Enabled: true,
			Config: json.RawMessage(legacyCfg),
		}
		if err := store.SaveChannel(legacyCh); err != nil {
			log.Printf("[server] failed to migrate legacy webhook: %v", err)
		}
	}

	// Load channels from DB
	if channels, err := store.ListChannels(); err == nil && len(channels) > 0 {
		notifManager.SetChannels(channels)
	}

	// WebSocket Hub
	hub := api.NewHub()
	go hub.Run()

	// Lua scripting engine
	luaEng := lua.NewEngine(
		func(nodeID string) []lua.Host {
			gs := agg.GlobalSnapshot()
			result := make([]lua.Host, 0)
			for _, h := range gs.Hosts {
				if nodeID == "" || h.NodeID == nodeID {
					result = append(result, lua.Host{
						IP:              h.IP,
						MAC:             h.MAC,
						Hostname:        h.Hostname,
						BytesSent:       h.BytesSent,
						BytesReceived:   h.BytesReceived,
						PacketsSent:     h.PacketsSent,
						PacketsReceived: h.PacketsReceived,
						Vendor:          h.Vendor,
						ActiveFlows:     h.ActiveFlows,
						NodeID:          h.NodeID,
					})
				}
			}
			return result
		},
		func(nodeID string) []lua.Flow {
			gs := agg.GlobalSnapshot()
			result := make([]lua.Flow, 0)
			for _, f := range gs.Flows {
				if nodeID == "" || f.NodeID == nodeID {
					result = append(result, lua.Flow{
						ID:          f.ID,
						SrcIP:       f.SrcIP,
						DstIP:       f.DstIP,
						SrcPort:     f.SrcPort,
						DstPort:     f.DstPort,
						Protocol:    f.Protocol,
						AppProtocol: f.AppProtocol,
						Bytes:       f.Bytes,
						Packets:     f.Packets,
					})
				}
			}
			return result
		},
		func(severity, alertType, message string) {
			alert := alertEng.EmitFromLua(severity, alertType, message)
			if alert != nil {
				hub.BroadcastMessage("new_alert", alert.ToJSON())
				store.InsertAlert(*alert)
				notifManager.Send(*alert)
			}
		},
	)

	// Load persisted Lua scripts
	if raw, err := store.GetConfig("lua_scripts"); err == nil && raw != "" {
		var scripts []lua.Script
		if json.Unmarshal([]byte(raw), &scripts) == nil {
			for _, s := range scripts {
				luaEng.RegisterScript(s.Name, s.Content, s.Enabled)
			}
			log.Printf("[server] loaded %d Lua scripts", len(scripts))
		}
	}

	// GeoIP Engine
	geoipEng := geoip.Default()
	if cfg.GeoIPCountryDB != "" {
		if err := geoipEng.LoadCountry(cfg.GeoIPCountryDB); err != nil {
			log.Printf("[server] failed to load GeoIP country DB: %v", err)
		}
	}
	if cfg.GeoIPASNDB != "" {
		if err := geoipEng.LoadASN(cfg.GeoIPASNDB); err != nil {
			log.Printf("[server] failed to load GeoIP ASN DB: %v", err)
		}
	}
	// Load persisted download URLs
	if url, _ := store.GetConfig("geoip_country_url"); url != "" {
		log.Printf("[server] GeoIP country download URL configured: %s", url)
	}
	if url, _ := store.GetConfig("geoip_asn_url"); url != "" {
		log.Printf("[server] GeoIP ASN download URL configured: %s", url)
	}

	// API Server
	srv := api.NewServer(cfg, agg, store, alertEng, hub, notifManager)
	srv.SetLuaEngine(luaEng)
	srv.SetGeoIPEngine(geoipEng)
	router := api.NewRouter(srv, cfg.WebDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Node token validator (used when --node-auth is set)
	tokenValidator := func(token string) bool {
		valid, err := store.ValidateNodeToken(token)
		return err == nil && valid
	}

	// Start gRPC server
	grpcReceiver, grpcSrv, err := receiver.StartGRPCServer(cfg, agg, cfg.NodeAuth, tokenValidator)
	if err != nil {
		log.Fatalf("[server] gRPC server init failed: %v", err)
	}

	// Wire config update callback from API to gRPC broadcast
	srv.SetConfigUpdateCallback(func(key, value string) {
		if key == "bpf_filter" {
			grpcReceiver.BroadcastConfigUpdate(value, 0)
		}
	})
	go func() {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
		if err != nil {
			log.Fatalf("[server] gRPC listen failed: %v", err)
		}
		log.Printf("[server] gRPC listening on :%d", cfg.GRPCPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Printf("[server] gRPC server error: %v", err)
		}
	}()

	// Start NetFlow/sFlow collector
	if cfg.NetFlowPort > 0 || cfg.SFlowPort > 0 {
		flowCollector := collector.New(collector.Config{
			NetFlowPort: cfg.NetFlowPort,
			SFlowPort:   cfg.SFlowPort,
		}, func(records []collector.FlowRecord) {
			// Convert collector.FlowRecord to aggregator.FlowRecord
			aggRecords := make([]aggregator.FlowRecord, len(records))
			for i, r := range records {
				aggRecords[i] = aggregator.FlowRecord{
					SrcIP:    r.SrcIP,
					DstIP:    r.DstIP,
					SrcPort:  r.SrcPort,
					DstPort:  r.DstPort,
					Protocol: r.Protocol,
					Bytes:    r.Bytes,
					Packets:  r.Packets,
					NodeID:   r.NodeID,
				}
			}
			agg.IngestFlowRecords(aggRecords)
		})
		if err := flowCollector.Start(ctx); err != nil {
			log.Printf("[server] collector start failed: %v", err)
		}
	}

	// Start SNMP poller
	if len(cfg.SNMPDevices) > 0 {
		snmpDevices := make([]snmp.DeviceConfig, len(cfg.SNMPDevices))
		for i, d := range cfg.SNMPDevices {
			customOIDs := make([]snmp.CustomOID, len(d.CustomOIDs))
			for j, coid := range d.CustomOIDs {
				customOIDs[j] = snmp.CustomOID{OID: coid.OID, Name: coid.Name, Type: coid.Type}
			}
			snmpDevices[i] = snmp.DeviceConfig{
				Name:             d.Name,
				Target:           d.Target,
				Community:        d.Community,
				Version:          d.Version,
				Port:             d.Port,
				V3Username:       d.V3Username,
				V3AuthProtocol:   d.V3AuthProtocol,
				V3AuthPassphrase: d.V3AuthPassphrase,
				V3PrivProtocol:   d.V3PrivProtocol,
				V3PrivPassphrase: d.V3PrivPassphrase,
				V3SecurityLevel:  d.V3SecurityLevel,
				CustomOIDs:       customOIDs,
			}
		}
		snmpPoller := snmp.NewPoller(snmpDevices, 30*time.Second, func(snap snmp.DeviceSnapshot) {
			// Convert to aggregator format
			ifaces := make([]aggregator.SNMPInterfaceSnapshot, len(snap.Interfaces))
			for i, iface := range snap.Interfaces {
				ifaces[i] = aggregator.SNMPInterfaceSnapshot{
					Index:      iface.Index,
					Name:       iface.Name,
					Alias:      iface.Alias,
					InOctets:   iface.InOctets,
					OutOctets:  iface.OutOctets,
					InErrors:   iface.InErrors,
					OutErrors:  iface.OutErrors,
					Speed:      iface.Speed,
					OperStatus: iface.OperStatus,
				}
			}
			agg.IngestSNMP(aggregator.SNMPDeviceSnapshot{
				NodeID:      snap.NodeID,
				DisplayName: snap.DeviceName,
				SysName:     snap.SysName,
				SysDescr:    snap.SysDescr,
				SysUptime:   snap.SysUptime,
				Interfaces:  ifaces,
				Timestamp:   snap.Timestamp,
			})
		})
		go snmpPoller.Start(ctx)
	}

	// Syslog collector
	if cfg.SyslogPort > 0 {
		syslogCollector := collector.NewSyslogCollector(cfg.SyslogPort, func(msg collector.SyslogMessage) {
			if err := store.InsertSyslog(msg.ID, msg.Timestamp.UnixMilli(), msg.Facility, msg.Severity, msg.Hostname, msg.AppName, msg.Message, msg.Source); err != nil {
				log.Printf("[syslog] storage error: %v", err)
			}
		})
		if err := syslogCollector.Start(ctx); err != nil {
			log.Printf("[syslog] start failed: %v", err)
		}
	}

	// SNMP Trap receiver
	if cfg.TrapPort > 0 {
		trapReceiver := collector.NewTrapReceiver(cfg.TrapPort, func(msg collector.TrapMessage) {
			collector.StoreTrap(msg)
		})
		if err := trapReceiver.Start(ctx); err != nil {
			log.Printf("[snmptrap] start failed: %v", err)
		}
	}

	// Network discovery
	if cfg.DiscoverySubnets != "" {
		subnets := splitAndTrim(cfg.DiscoverySubnets, ",")
		discoCfg := discovery.DefaultConfig()
		discoCfg.Subnets = subnets
		discoScanner := discovery.NewScanner(discoCfg, agg)
		go discoScanner.Start(ctx)
		log.Printf("[server] network discovery enabled for subnets: %v", subnets)
	}

	// Periodic snapshot broadcast
	go func() {
		ticker := time.NewTicker(cfg.SnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				gs := agg.GlobalSnapshot()
				rawAlerts := alertEng.GetAlerts()
				alertsJSON := make([]models.AlertJSON, len(rawAlerts))
				for i, a := range rawAlerts {
					alertsJSON[i] = a.ToJSON()
				}
				gs.Alerts = alertsJSON
				hub.BroadcastMessage("snapshot", gs)
			}
		}
	}()

	// Periodic history storage
	go func() {
		ticker := time.NewTicker(cfg.HistoryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				gs := agg.GlobalSnapshot()
				store.InsertSnapshot(gs.Traffic)
				now := time.Now()
				for _, node := range gs.Nodes {
					if node.BytesPerSec > 0 {
						snap := models.TrafficSnapshot{
							Timestamp:     now,
							BytesPerSec:   node.BytesPerSec,
							PacketsPerSec: node.PacketsPerSec,
							FlowsCount:    node.FlowsCount,
							NodeID:        node.NodeID,
						}
						store.InsertSnapshot(snap)
					}
				}
				// Per-host snapshots
				var hostSnaps []models.HostSnapshot
				for _, h := range gs.Hosts {
					hostSnaps = append(hostSnaps, models.HostSnapshot{
						Timestamp:       now,
						NodeID:          h.NodeID,
						HostIP:          h.IP,
						BytesSent:       float64(h.BytesSent),
						BytesReceived:   float64(h.BytesReceived),
						PacketsSent:     int(h.PacketsSent),
						PacketsReceived: int(h.PacketsReceived),
					})
				}
				if len(hostSnaps) > 0 {
					if err := store.InsertHostSnapshots(hostSnaps); err != nil {
						log.Printf("[server] failed to store host snapshots: %v", err)
					}
				}
			}
		}
	}()

	// Periodic alerting checks
	go func() {
		ticker := time.NewTicker(cfg.SnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				gs := agg.GlobalSnapshot()
				hosts := jsonHostsToModels(gs.Hosts)
				flows := jsonFlowsToModels(gs.Flows)
				alertEng.CheckNewHost(hosts)
				alertEng.CheckSuspiciousPort(flows)
				alertEng.CheckPortScan(flows)
				alertEng.CheckDNSSuspiciousPort(flows)
				alertEng.CheckDNSExfiltration(flows, gs.DnsQueries)
				alertEng.CheckHorizontalScan(flows)
				alertEng.CheckUnexpectedProtocol(flows)
				alertEng.CheckLongFlow(flows)
				alertEng.CheckARPSpoof(flows, hosts)
				alertEng.CheckDataExfiltration(hosts, "")
				for _, node := range gs.Nodes {
					nodeHosts := filterHostsByNode(hosts, node.NodeID)
					nodeFlows := filterFlowsByNode(flows, node.NodeID)
					alertEng.CheckHostBandwidth(nodeHosts, node.NodeID)
					alertEng.CheckFlowFlood(nodeFlows, node.NodeID)
					alertEng.CheckICMPFlood(nodeFlows, node.NodeID)
					alertEng.CheckSYNFlood(nodeFlows, node.NodeID)
					luaEng.OnCheck(node.NodeID)
				}
			}
		}
	}()

	// Alert forwarding to WebSocket + webhook
	go func() {
		ch := alertEng.AlertChannel()
		for {
			select {
			case <-ctx.Done():
				return
			case alert := <-ch:
				hub.BroadcastMessage("new_alert", alert.ToJSON())
				if err := store.InsertAlert(alert); err != nil {
					log.Printf("[server] failed to store alert: %v", err)
				}
				notifManager.Send(alert)
			}
		}
	}()

	// Node timeout check
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				agg.CheckNodeTimeouts(30 * time.Second)
			}
		}
	}()

	// Data retention cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				before := time.Now().Add(-cfg.Retention)
				if n, err := store.CleanupBefore(before); err != nil {
					log.Printf("[server] cleanup error: %v", err)
				} else if n > 0 {
					log.Printf("[server] cleaned up %d old records", n)
				}
			}
		}
	}()

	// Data aggregation
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Aggregate raw data older than 24h into hourly buckets
				if err := store.AggregateToHourly(time.Now().Add(-24 * time.Hour)); err != nil {
					log.Printf("[server] hourly aggregation error: %v", err)
				}
				// Aggregate hourly data older than 7d into daily buckets
				if err := store.AggregateToDaily(time.Now().Add(-7 * 24 * time.Hour)); err != nil {
					log.Printf("[server] daily aggregation error: %v", err)
				}
				// Aggregate daily data older than 30d into weekly buckets
				if err := store.AggregateToWeekly(time.Now().Add(-30 * 24 * time.Hour)); err != nil {
					log.Printf("[server] weekly aggregation error: %v", err)
				}
				// Clean up weekly data older than 365d
				weeklyCutoff := time.Now().Add(-365 * 24 * time.Hour)
				if n, err := store.CleanupWeekly(weeklyCutoff); err != nil {
					log.Printf("[server] weekly cleanup error: %v", err)
				} else if n > 0 {
					log.Printf("[server] cleaned up %d old weekly records", n)
				}
			}
		}
	}()

	// HTTP server
	// Periodic traffic matrix snapshot (every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now().UnixMilli()
				cells := agg.TrafficMatrix("", 100)
				for _, c := range cells {
					store.InsertMatrixSnapshot(c.Source, c.Destination, c.Bytes, now)
				}
			}
		}
	}()

	// Periodic VoIP session storage (every 60 seconds)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now().UnixMilli()
				sessions := agg.VOIPSessions()
				for _, s := range sessions {
					if s.Active {
						store.InsertVoipSession(s.SSRC, s.SrcIP, s.DstIP, s.SrcPort, s.DstPort, s.Packets, s.Bytes, s.LostPkts, s.JitterMS, s.MOS, s.Codec, now)
					}
				}
			}
		}
	}()

	// Periodic VoIP tracker cleanup (every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				agg.CleanupVOIPTracker(5 * time.Minute)
			}
		}
	}()

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}
	go func() {
		log.Printf("[server] HTTP/WebSocket listening on :%d", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[server] HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\n[server] shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	httpSrv.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
	cancel()
	log.Printf("[server] shutdown complete")
}

func jsonHostsToModels(hosts []models.HostJSON) []models.Host {
	result := make([]models.Host, len(hosts))
	for i, h := range hosts {
		result[i] = models.Host{
			IP:              h.IP,
			MAC:             h.MAC,
			Hostname:        h.Hostname,
			BytesSent:       h.BytesSent,
			BytesReceived:   h.BytesReceived,
			PacketsSent:     h.PacketsSent,
			PacketsReceived: h.PacketsReceived,
			FirstSeen:       time.UnixMilli(h.FirstSeen),
			LastSeen:        time.UnixMilli(h.LastSeen),
			Vendor:          h.Vendor,
			ActiveFlows:     h.ActiveFlows,
			NodeID:          h.NodeID,
		}
	}
	return result
}

func jsonFlowsToModels(flows []models.FlowJSON) []models.Flow {
	result := make([]models.Flow, len(flows))
	for i, f := range flows {
		result[i] = models.Flow{
			ID:          f.ID,
			SrcIP:       f.SrcIP,
			DstIP:       f.DstIP,
			SrcPort:     f.SrcPort,
			DstPort:     f.DstPort,
			Protocol:    f.Protocol,
			AppProtocol: f.AppProtocol,
			Bytes:       f.Bytes,
			Packets:     f.Packets,
			FirstSeen:   time.UnixMilli(f.FirstSeen),
			LastSeen:    time.UnixMilli(f.LastSeen),
			NodeID:      f.NodeID,
		}
	}
	return result
}

func filterHostsByNode(hosts []models.Host, nodeID string) []models.Host {
	filtered := make([]models.Host, 0)
	for _, h := range hosts {
		if h.NodeID == nodeID {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

func filterFlowsByNode(flows []models.Flow, nodeID string) []models.Flow {
	filtered := make([]models.Flow, 0)
	for _, f := range flows {
		if f.NodeID == nodeID {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
