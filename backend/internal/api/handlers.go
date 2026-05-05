package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/netgazer/backend/internal/aggregator"
	"github.com/netgazer/backend/internal/alerting"
	"github.com/netgazer/backend/internal/auth"
	"github.com/netgazer/backend/internal/config"
	"github.com/netgazer/backend/internal/models"
	"github.com/netgazer/backend/internal/collector"
	"github.com/netgazer/backend/internal/geoip"
	"github.com/netgazer/backend/internal/report"
	"github.com/netgazer/backend/internal/lua"
	"github.com/netgazer/backend/internal/storage"
	"github.com/netgazer/backend/internal/webhook"
)

type Server struct {
	cfg              *config.ServerConfig
	agg              *aggregator.Aggregator
	store            *storage.Store
	alert            *alerting.Engine
	Hub              *Hub
	start            time.Time
	notifier         *webhook.Manager
	reportGen        *report.Generator
	exporter         *report.Exporter
	onConfigUpdate   func(key, value string)
	luaEng           *lua.Engine
	onApplyIntercept func(targetNodes []string, rules []interceptRuleJSON)
	geoipEng         *geoip.Engine
}

func NewServer(cfg *config.ServerConfig, agg *aggregator.Aggregator, store *storage.Store, alert *alerting.Engine, hub *Hub, notifier *webhook.Manager) *Server {
	return &Server{
		cfg:       cfg,
		agg:       agg,
		store:     store,
		alert:     alert,
		Hub:       hub,
		start:     time.Now(),
		notifier:  notifier,
		reportGen: report.NewGenerator(store),
		exporter:  report.NewExporter(store),
	}
}

func (s *Server) SetConfigUpdateCallback(fn func(key, value string)) {
	s.onConfigUpdate = fn
}

func (s *Server) SetLuaEngine(eng *lua.Engine) {
	s.luaEng = eng
}

func (s *Server) SetGeoIPEngine(eng *geoip.Engine) {
	s.geoipEng = eng
}

func (s *Server) broadcastBPFFilter(filter string) {
	if s.onConfigUpdate != nil {
		s.onConfigUpdate("bpf_filter", filter)
	}
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if body.Username == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password required"})
		return
	}

	user, err := s.store.GetUserByUsername(body.Username)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !auth.VerifyPassword(body.Password, user.PasswordHash) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := auth.GenerateToken(s.cfg.JWTSecret, user.Username, 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "token generation failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":    token,
		"username": user.Username,
	})
}

func (s *Server) CheckSetup(w http.ResponseWriter, r *http.Request) {
	hasUsers := s.store.HasUsers()
	writeJSON(w, http.StatusOK, map[string]bool{"setup_required": !hasUsers})
}

func (s *Server) Setup(w http.ResponseWriter, r *http.Request) {
	if s.store.HasUsers() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "setup already completed"})
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if len(body.Username) < 2 || len(body.Username) > 64 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username must be 2-64 characters"})
		return
	}
	if len(body.Password) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must be at least 6 characters"})
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}

	if err := s.store.CreateUser(body.Username, hash); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) GetSummary(w http.ResponseWriter, r *http.Request) {
	gs := s.agg.GlobalSnapshot()

	online := 0
	for _, n := range gs.Nodes {
		if n.Online {
			online++
		}
	}

	totalBytes := uint64(0)
	totalPackets := uint64(0)
	for _, f := range gs.Flows {
		totalBytes += f.Bytes
		totalPackets += f.Packets
	}

	summary := models.Summary{
		HostsCount:   len(gs.Hosts),
		ActiveFlows:  len(gs.Flows),
		TotalBytes:   totalBytes,
		TotalPackets: totalPackets,
		Uptime:       time.Since(s.start).Round(time.Second).String(),
		NodesOnline:  online,
		NodesTotal:   len(gs.Nodes),
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) GetHosts(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	iface := r.URL.Query().Get("interface")
	search := r.URL.Query().Get("search")
	country := r.URL.Query().Get("country")
	asn := r.URL.Query().Get("asn")
	sortBy := r.URL.Query().Get("sort")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	hosts, total := s.agg.PaginatedHosts(nodeID, iface, search, country, asn, limit, offset, sortBy)
	if hosts == nil {
		hosts = []models.HostJSON{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": hosts,
		"total": total,
	})
}

func (s *Server) GetHost(w http.ResponseWriter, r *http.Request) {
	ip := chi.URLParam(r, "ip")
	nodeID := r.URL.Query().Get("node_id")

	gs := s.agg.GlobalSnapshot()
	for _, h := range gs.Hosts {
		if h.IP == ip && (nodeID == "" || h.NodeID == nodeID) {
			writeJSON(w, http.StatusOK, h)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
}

func (s *Server) GetFlows(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	iface := r.URL.Query().Get("interface")
	search := r.URL.Query().Get("search")
	protoFilter := r.URL.Query().Get("protocol")
	appFilter := r.URL.Query().Get("app")
	sortBy := r.URL.Query().Get("sort")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	flows, total := s.agg.PaginatedFlows(nodeID, iface, search, protoFilter, appFilter, limit, offset, sortBy)
	if flows == nil {
		flows = []models.FlowJSON{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": flows,
		"total": total,
	})
}

func (s *Server) GetProtocols(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	iface := r.URL.Query().Get("interface")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	protocols, total := s.agg.PaginatedProtocols(nodeID, iface, limit, offset)
	if protocols == nil {
		protocols = []models.ProtocolStat{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": protocols,
		"total": total,
	})
}

func (s *Server) GetAlerts(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	alerts := s.alert.GetAlerts()

	// Apply node filter
	if nodeID != "" {
		filtered := make([]models.Alert, 0)
		for _, a := range alerts {
			if a.NodeID == nodeID {
				filtered = append(filtered, a)
			}
		}
		alerts = filtered
	}

	if offset > len(alerts) {
		alerts = []models.Alert{}
	} else {
		alerts = alerts[offset:]
	}
	if limit < len(alerts) {
		alerts = alerts[:limit]
	}

	writeJSON(w, http.StatusOK, alerts)
}

func (s *Server) GetHostProtocols(w http.ResponseWriter, r *http.Request) {
	ip := chi.URLParam(r, "ip")
	nodeID := r.URL.Query().Get("node_id")
	protocols := s.agg.HostProtocols(nodeID, ip)
	if protocols == nil {
		protocols = []models.ProtocolStat{}
	}
	writeJSON(w, http.StatusOK, protocols)
}

func (s *Server) GetHostPeers(w http.ResponseWriter, r *http.Request) {
	ip := chi.URLParam(r, "ip")
	nodeID := r.URL.Query().Get("node_id")
	peers := s.agg.HostPeers(nodeID, ip)
	if peers == nil {
		peers = []models.HostPeer{}
	}
	writeJSON(w, http.StatusOK, peers)
}

func (s *Server) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if s.alert.Acknowledge(id) {
		if err := s.store.AcknowledgeAlert(id); err != nil {
			// Non-fatal: already in memory
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "alert not found"})
	}
}

func (s *Server) GetTrafficHistory(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	granularity := r.URL.Query().Get("granularity")
	interval, _ := strconv.Atoi(r.URL.Query().Get("interval"))
	if interval <= 0 {
		interval = 10
	}

	to := time.Now()
	from := to.Add(-10 * time.Minute)

	// Wider default range for aggregated data
	if granularity == "hourly" {
		from = to.Add(-7 * 24 * time.Hour)
	} else if granularity == "daily" {
		from = to.Add(-30 * 24 * time.Hour)
	}

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	var snapshots []models.TrafficSnapshot
	var err error
	if granularity != "" && granularity != "raw" {
		snapshots, err = s.store.QuerySnapshotsGranular(nodeID, from, to, interval, granularity)
	} else {
		snapshots, err = s.store.QuerySnapshots(nodeID, from, to, interval)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if snapshots == nil {
		snapshots = []models.TrafficSnapshot{}
	}

	writeJSON(w, http.StatusOK, snapshots)
}

func (s *Server) GetHostTrafficHistory(w http.ResponseWriter, r *http.Request) {
	ip := chi.URLParam(r, "ip")
	nodeID := r.URL.Query().Get("node_id")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	to := time.Now()
	from := to.Add(-30 * time.Minute)

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	snapshots, err := s.store.QueryHostSnapshots(ip, nodeID, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, snapshots)
}

func (s *Server) GetTrafficMatrix(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	matrix := s.agg.TrafficMatrix(nodeID, limit)
	if matrix == nil {
		matrix = []models.TrafficMatrixCell{}
	}
	writeJSON(w, http.StatusOK, matrix)
}

func (s *Server) GetNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.agg.GetNodeStates()
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]interface{}{
		"bandwidth_threshold_bps": s.cfg.BandwidthThresholdBps,
		"alert_thresholds":        s.alert.GetThresholds(),
		"bpf_filter":              s.agg.GetBPFFilter(),
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var update map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if v, ok := update["bandwidth_threshold_bps"]; ok {
		if fv, ok := v.(float64); ok {
			s.cfg.BandwidthThresholdBps = fv
			s.alert.SetBandwidthThreshold(fv)
			s.store.SetConfig("bandwidth_threshold_bps", fmt.Sprintf("%.0f", fv))
		}
	}

	if v, ok := update["alert_thresholds"]; ok {
		b, _ := json.Marshal(v)
		var t alerting.AlertThresholds
		if err := json.Unmarshal(b, &t); err == nil {
			s.alert.SetThresholds(t)
			s.store.SetConfig("alert_thresholds", string(b))
		}
	}

	if v, ok := update["bpf_filter"]; ok {
		if str, ok := v.(string); ok {
			s.agg.SetBPFFilter(str)
			s.store.SetConfig("bpf_filter", str)
			s.broadcastBPFFilter(str)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- Notification Channels ----

func (s *Server) ListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.store.ListChannels()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, channels)
}

func (s *Server) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var ch models.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if ch.ID == "" {
		ch.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if err := s.store.SaveChannel(ch); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Reload manager
	s.reloadNotifier()
	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := s.store.GetChannel(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
		return
	}
	var raw map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if v, ok := raw["name"]; ok {
		if s, ok := v.(string); ok {
			existing.Name = s
		}
	}
	if v, ok := raw["type"]; ok {
		if s, ok := v.(string); ok {
			existing.Type = models.NotificationChannelType(s)
		}
	}
	if v, ok := raw["enabled"]; ok {
		if b, ok := v.(bool); ok {
			existing.Enabled = b
		}
	}
	if v, ok := raw["config"]; ok {
		b, _ := json.Marshal(v)
		existing.Config = json.RawMessage(b)
	}
	if err := s.store.SaveChannel(*existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.reloadNotifier()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteChannel(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.reloadNotifier()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) TestChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.notifier.Test(id); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) reloadNotifier() {
	channels, err := s.store.ListChannels()
	if err != nil {
		return
	}
	s.notifier.SetChannels(channels)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func filterHostsByNode(hosts []models.HostJSON, nodeID string) []models.HostJSON {
	filtered := make([]models.HostJSON, 0)
	for _, h := range hosts {
		if h.NodeID == nodeID {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

func filterFlowsByNode(flows []models.FlowJSON, nodeID string) []models.FlowJSON {
	filtered := make([]models.FlowJSON, 0)
	for _, f := range flows {
		if f.NodeID == nodeID {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func filterProtocolsByNode(protocols []models.ProtocolStat, nodeID string) []models.ProtocolStat {
	filtered := make([]models.ProtocolStat, 0)
	for _, p := range protocols {
		if p.NodeID == nodeID {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// ---- Report handlers ----

func parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from, to time.Time
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return from, to, fmt.Errorf("invalid 'from' parameter: %w", err)
		}
		from = t
	} else {
		from = time.Now().Add(-24 * time.Hour)
	}

	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return from, to, fmt.Errorf("invalid 'to' parameter: %w", err)
		}
		to = t
	} else {
		to = time.Now()
	}
	return from, to, nil
}

// GetReportSummary returns a summary report for a time range.
func (s *Server) GetReportSummary(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	report, err := s.reportGen.GenerateSummary(nodeID, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// GetReportTopTalkers returns top talkers for a time range.
func (s *Server) GetReportTopTalkers(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	talkers, err := s.reportGen.GenerateTopTalkers(nodeID, from, to, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, talkers)
}

// GetReportTopProtocols returns top protocols for a time range.
func (s *Server) GetReportTopProtocols(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	protocols, err := s.reportGen.GenerateTopProtocols(nodeID, from, to, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, protocols)
}

// GetReportAlerts returns alert statistics for a time range.
func (s *Server) GetReportAlerts(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	alerts, err := s.reportGen.GenerateAlertReport(nodeID, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

// GetReportTrend returns traffic trend data for a time range.
func (s *Server) GetReportTrend(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	trend, err := s.reportGen.GenerateTrend(nodeID, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, trend)
}

// ---- Export handlers ----

// ExportSnapshots exports traffic snapshots in the requested format.
func (s *Server) ExportSnapshots(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	format := report.ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = report.FormatJSON
	}

	ct, filename := contentType(format, "traffic-snapshots")
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	if err := s.exporter.ExportSnapshots(nodeID, from, to, format, w); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
}

// ExportHosts exports host snapshots in the requested format.
func (s *Server) ExportHosts(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	format := report.ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = report.FormatJSON
	}

	ct, filename := contentType(format, "host-snapshots")
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	if err := s.exporter.ExportHostSnapshots(nodeID, from, to, format, w); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
}

// ExportAlerts exports alerts in the requested format.
func (s *Server) ExportAlerts(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	from, to, err := parseTimeRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	format := report.ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = report.FormatJSON
	}

	ct, filename := contentType(format, "alerts")
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	if err := s.exporter.ExportAlerts(nodeID, from, to, format, w); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
}

func contentType(format report.ExportFormat, base string) (string, string) {
	switch format {
	case report.FormatJSON:
		return "application/json", base + ".json"
	case report.FormatCSV:
		return "text/csv", base + ".csv"
	case report.FormatNDJSON:
		return "application/x-ndjson", base + ".ndjson"
	case report.FormatClickHouse:
		return "text/tab-separated-values", base + ".tsv"
	default:
		return "application/octet-stream", base + ".dat"
	}
}

// GetSyslog returns paginated syslog messages.
func (s *Server) GetSyslog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	severity := q.Get("severity")
	source := q.Get("source")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	offset, _ := strconv.Atoi(q.Get("offset"))

	records, total, err := s.store.QuerySyslog(severity, source, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if records == nil {
		records = []storage.SyslogRecord{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": records, "total": total})
}

// GetMatrixHistory returns traffic matrix snapshots over time.
func (s *Server) GetMatrixHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, _ := strconv.ParseInt(q.Get("from"), 10, 64)
	to, _ := strconv.ParseInt(q.Get("to"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 1000
	}
	if from == 0 {
		from = time.Now().Add(-1 * time.Hour).UnixMilli()
	}
	if to == 0 {
		to = time.Now().UnixMilli()
	}
	cells, err := s.store.QueryMatrixHistory(from, to, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": cells})
}

// GetTraps returns stored SNMP trap messages.
func (s *Server) GetTraps(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}
	traps := collector.GetTraps(limit)
	writeJSON(w, http.StatusOK, map[string]any{"items": traps, "total": len(traps)})
}

// GetVoipSessions returns current VoIP sessions.
func (s *Server) GetVoipSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.agg.VOIPSessions()
	writeJSON(w, http.StatusOK, map[string]any{"items": sessions, "total": len(sessions)})
}

// -- Lua script management --

func (s *Server) saveLuaScripts() {
	if s.luaEng == nil {
		return
	}
	scripts := s.luaEng.ListScripts()
	data, _ := json.Marshal(scripts)
	s.store.SetConfig("lua_scripts", string(data))
}

// ListLuaScripts returns all Lua scripts.
func (s *Server) ListLuaScripts(w http.ResponseWriter, r *http.Request) {
	if s.luaEng == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "total": 0})
		return
	}
	scripts := s.luaEng.ListScripts()
	writeJSON(w, http.StatusOK, map[string]any{"items": scripts, "total": len(scripts)})
}

// CreateLuaScript creates or updates a Lua script.
func (s *Server) CreateLuaScript(w http.ResponseWriter, r *http.Request) {
	if s.luaEng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Lua engine not available"})
		return
	}
	var req lua.Script
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	s.luaEng.RegisterScript(req.Name, req.Content, req.Enabled)
	s.saveLuaScripts()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// interceptRuleJSON is the JSON representation of an intercept rule.
type interceptRuleJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Action     string `json:"action"`
	Enabled    bool   `json:"enabled"`
}

// SetInterceptApplyFunc sets the callback used to apply rules to nodes.
func (s *Server) SetInterceptApplyFunc(fn func(targetNodes []string, rules []interceptRuleJSON)) {
	s.onApplyIntercept = fn
}

// ListInterceptRules returns all intercept rules.
func (s *Server) ListInterceptRules(w http.ResponseWriter, r *http.Request) {
	records, err := s.store.ListInterceptRules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rules := make([]interceptRuleJSON, 0, len(records))
	for _, rec := range records {
		rules = append(rules, interceptRuleJSON{
			ID:         rec.ID,
			Name:       rec.Name,
			Expression: rec.Expression,
			Action:     rec.Action,
			Enabled:    rec.Enabled,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

// CreateInterceptRule creates a new intercept rule.
func (s *Server) CreateInterceptRule(w http.ResponseWriter, r *http.Request) {
	var req interceptRuleJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Name == "" || req.Expression == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and expression are required"})
		return
	}
	now := time.Now().Unix()
	rec := storage.InterceptRuleRecord{
		ID:         req.ID,
		Name:       req.Name,
		Expression: req.Expression,
		Action:     req.Action,
		Enabled:    req.Enabled,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if rec.ID == "" {
		rec.ID = fmt.Sprintf("ir_%d", now)
	}
	if rec.Action == "" {
		rec.Action = "block"
	}
	if err := s.store.CreateInterceptRule(rec); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok", "id": rec.ID})
}

// UpdateInterceptRule updates an existing intercept rule.
func (s *Server) UpdateInterceptRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	var req interceptRuleJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	rec := storage.InterceptRuleRecord{
		ID:         id,
		Name:       req.Name,
		Expression: req.Expression,
		Action:     req.Action,
		Enabled:    req.Enabled,
		UpdatedAt:  time.Now().Unix(),
	}
	if err := s.store.UpdateInterceptRule(rec); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteInterceptRule deletes an intercept rule by ID.
func (s *Server) DeleteInterceptRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteInterceptRule(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ApplyInterceptRules sends intercept rules to specified nodes.
func (s *Server) ApplyInterceptRules(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeIDs []string `json:"node_ids"`
		RuleIDs []string `json:"rule_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	records, err := s.store.ListInterceptRules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	ruleIDSet := make(map[string]bool, len(req.RuleIDs))
	for _, id := range req.RuleIDs {
		ruleIDSet[id] = true
	}
	selectSpecific := len(req.RuleIDs) > 0
	var selected []interceptRuleJSON
	for _, rec := range records {
		if !rec.Enabled {
			continue
		}
		if selectSpecific && !ruleIDSet[rec.ID] {
			continue
		}
		selected = append(selected, interceptRuleJSON{
			ID:         rec.ID,
			Name:       rec.Name,
			Expression: rec.Expression,
			Action:     rec.Action,
			Enabled:    rec.Enabled,
		})
	}
	if s.onApplyIntercept != nil {
		s.onApplyIntercept(req.NodeIDs, selected)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"sent_to":     req.NodeIDs,
		"rules_count": len(selected),
	})
}

// GetInterceptNodeRules returns the current rules for a specific node.
func (s *Server) GetInterceptNodeRules(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "node")
	if nodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node id is required"})
		return
	}
	records, err := s.store.ListInterceptRules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	rules := make([]interceptRuleJSON, 0, len(records))
	for _, rec := range records {
		if rec.Enabled {
			rules = append(rules, interceptRuleJSON{
				ID:         rec.ID,
				Name:       rec.Name,
				Expression: rec.Expression,
				Action:     rec.Action,
				Enabled:    rec.Enabled,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"node": nodeID, "rules": rules})
}

// DeleteLuaScript deletes a Lua script by name.
func (s *Server) DeleteLuaScript(w http.ResponseWriter, r *http.Request) {
	if s.luaEng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Lua engine not available"})
		return
	}
	name := chi.URLParam(r, "name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	s.luaEng.RemoveScript(name)
	s.saveLuaScripts()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// TestLuaScript runs a Lua script snippet and returns results.
func (s *Server) TestLuaScript(w http.ResponseWriter, r *http.Request) {
	if s.luaEng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Lua engine not available"})
		return
	}
	var req struct {
		Content string `json:"content"`
		NodeID  string `json:"node_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	err := s.luaEng.RunTest(req.Content, req.NodeID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

	// ── GeoIP handlers ──

	func (s *Server) GetGeoIPStatus(w http.ResponseWriter, r *http.Request) {
		if s.geoipEng == nil {
			writeJSON(w, http.StatusOK, map[string]any{"ready": false})
			return
		}
		writeJSON(w, http.StatusOK, s.geoipEng.Status())
	}

	func (s *Server) UploadGeoIPDB(w http.ResponseWriter, r *http.Request) {
		if s.geoipEng == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GeoIP engine not available"})
			return
		}
		if err := r.ParseMultipartForm(100 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse multipart form"})
			return
		}
		dbType := r.FormValue("type")

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
			return
		}
		defer file.Close()

		if err := geoip.EnsureDir(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create data directory"})
			return
		}

		ext := filepath.Ext(header.Filename)
		savePath := filepath.Join(geoip.GeoipDir, dbType+ext)
		if dbType != "country" && dbType != "asn" {
			savePath = filepath.Join(geoip.GeoipDir, "geoip"+ext)
		}

		dst, err := os.Create(savePath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file: " + err.Error()})
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, file); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to write file: " + err.Error()})
			return
		}

		var loadErr error
		switch dbType {
		case "country":
			loadErr = s.geoipEng.LoadCountry(savePath)
		case "asn":
			loadErr = s.geoipEng.LoadASN(savePath)
		}

		if loadErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "file saved but failed to load: " + loadErr.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "path": savePath, "type": dbType, "size": header.Size})
	}

	func (s *Server) DownloadGeoIPDB(w http.ResponseWriter, r *http.Request) {
		if s.geoipEng == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GeoIP engine not available"})
			return
		}

		var req struct {
			URL  string `json:"url"`
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		if req.URL == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
			return
		}

		if err := geoip.EnsureDir(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create data directory"})
			return
		}

		savePath := filepath.Join(geoip.GeoipDir, req.Type+".mmdb")

		resp, err := http.Get(req.URL)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "download failed: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("download returned status %d", resp.StatusCode)})
			return
		}

		dst, err := os.Create(savePath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create file: " + err.Error()})
			return
		}
		defer dst.Close()

		written, err := io.Copy(dst, resp.Body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to write file: " + err.Error()})
			return
		}

		var loadErr error
		switch req.Type {
		case "country":
			loadErr = s.geoipEng.LoadCountry(savePath)
		case "asn":
			loadErr = s.geoipEng.LoadASN(savePath)
		}

		if loadErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "file downloaded but failed to load: " + loadErr.Error()})
			return
		}

		s.store.SetConfig("geoip_"+req.Type+"_url", req.URL)

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "path": savePath, "type": req.Type, "size": written})
	}

func (s *Server) ListNodeTokens(w http.ResponseWriter, r *http.Request) {
	records, err := s.store.ListNodeTokens()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type tokenJSON struct {
		ID          string `json:"id"`
		Token       string `json:"token"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
		CreatedAt   int64  `json:"created_at"`
		LastUsedAt  *int64 `json:"last_used_at"`
	}
	tokens := make([]tokenJSON, len(records))
	for i, r := range records {
		tok := r.Token
		if len(tok) > 8 {
			tok = tok[:4] + "..." + tok[len(tok)-4:]
		}
		tokens[i] = tokenJSON{
			ID:          r.ID,
			Token:       tok,
			Description: r.Description,
			Enabled:     r.Enabled,
			CreatedAt:   r.CreatedAt,
			LastUsedAt:  r.LastUsedAt,
		}
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) CreateNodeToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	id := "nt-" + randomHex(12)
	token := randomHex(64)
	if err := s.store.CreateNodeToken(id, token, req.Description); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"id":          id,
		"token":       token,
		"description": req.Description,
		"warning":     "Store this token securely. It will not be shown again.",
	})
}

func (s *Server) DeleteNodeToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteNodeToken(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}
