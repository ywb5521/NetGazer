package report

import (
	"time"

	"github.com/netgazer/backend/internal/models"
	"github.com/netgazer/backend/internal/storage"
)

// Generator produces historical reports from stored data.
type Generator struct {
	store *storage.Store
}

func NewGenerator(store *storage.Store) *Generator {
	return &Generator{store: store}
}

// SummaryReport is a high-level overview for a time range.
type SummaryReport struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	TotalBytes  float64   `json:"total_bytes"`
	AvgBps      float64   `json:"avg_bps"`
	PeakBps     float64   `json:"peak_bps"`
	UniqueHosts int       `json:"unique_hosts"`
	TotalFlows  int       `json:"total_flows"`
	AlertCount  int       `json:"alert_count"`
}

// TopTalker is a host with aggregate traffic stats.
type TopTalker struct {
	IP            string  `json:"ip"`
	Hostname      string  `json:"hostname"`
	TotalBytes    float64 `json:"total_bytes"`
	BytesSent     float64 `json:"bytes_sent"`
	BytesReceived float64 `json:"bytes_received"`
	FlowCount     int     `json:"flow_count"`
}

// TopProtocol is an application protocol with aggregate stats.
type TopProtocol struct {
	Name       string  `json:"name"`
	TotalBytes float64 `json:"total_bytes"`
	PctBytes   float64 `json:"pct_bytes"`
	FlowCount  int     `json:"flow_count"`
}

// AlertSummary groups alerts by type and severity.
type AlertSummary struct {
	Total      int            `json:"total"`
	ByType     map[string]int `json:"by_type"`
	BySeverity map[string]int `json:"by_severity"`
	Recent     []models.Alert `json:"recent"`
}

// TrendPoint is a single data point in a traffic trend.
type TrendPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	BytesPerSec   float64   `json:"bytes_per_sec"`
	PacketsPerSec float64   `json:"packets_per_sec"`
	FlowsCount    int       `json:"flows_count"`
}

// GenerateSummary generates a summary report for the given time range.
func (g *Generator) GenerateSummary(nodeID string, from, to time.Time) (*SummaryReport, error) {
	snaps, err := g.store.QuerySnapshotsGranular(nodeID, from, to, 0, bestGranularity(from, to))
	if err != nil {
		return nil, err
	}

	report := &SummaryReport{
		From: from,
		To:   to,
	}

	if len(snaps) == 0 {
		return report, nil
	}

	var totalBytes float64
	var peakBps float64
	for _, s := range snaps {
		// Approximate total bytes from bps * interval (use 1s for raw, 3600s for hourly, etc.)
		totalBytes += s.BytesPerSec * float64(snapInterval(from, to))
		if s.BytesPerSec > peakBps {
			peakBps = s.BytesPerSec
		}
	}
	report.TotalBytes = totalBytes
	report.AvgBps = totalBytes / float64(to.Sub(from).Seconds())
	report.PeakBps = peakBps

	// Unique hosts count
	hosts, err := g.store.QueryUniqueHosts(nodeID, from, to)
	if err == nil {
		report.UniqueHosts = hosts
	}

	// Alert count
	alertCount, err := g.store.CountAlerts(nodeID, from, to)
	if err == nil {
		report.AlertCount = alertCount
	}

	return report, nil
}

// GenerateTopTalkers returns the top N hosts by total traffic in a range.
func (g *Generator) GenerateTopTalkers(nodeID string, from, to time.Time, limit int) ([]TopTalker, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := g.store.QueryTopHosts(nodeID, from, to, limit)
	if err != nil {
		return nil, err
	}

	var talkers []TopTalker
	for _, r := range rows {
		talkers = append(talkers, TopTalker{
			IP:            r.IP,
			Hostname:      r.Hostname,
			TotalBytes:    r.TotalBytes,
			BytesSent:     r.BytesSent,
			BytesReceived: r.BytesReceived,
			FlowCount:     r.FlowCount,
		})
	}
	if talkers == nil {
		talkers = []TopTalker{}
	}
	return talkers, nil
}

// GenerateTopProtocols returns the top N protocols by traffic in a range.
func (g *Generator) GenerateTopProtocols(nodeID string, from, to time.Time, limit int) ([]TopProtocol, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := g.store.QueryTopProtocols(nodeID, from, to, limit)
	if err != nil {
		return nil, err
	}

	var totalBytes float64
	for _, r := range rows {
		totalBytes += r.TotalBytes
	}

	var protocols []TopProtocol
	for _, r := range rows {
		pct := 0.0
		if totalBytes > 0 {
			pct = (r.TotalBytes / totalBytes) * 100
		}
		protocols = append(protocols, TopProtocol{
			Name:       r.Name,
			TotalBytes: r.TotalBytes,
			PctBytes:   pct,
			FlowCount:  r.FlowCount,
		})
	}
	if protocols == nil {
		protocols = []TopProtocol{}
	}
	return protocols, nil
}

// GenerateAlertReport returns alert statistics for a time range.
func (g *Generator) GenerateAlertReport(nodeID string, from, to time.Time) (*AlertSummary, error) {
	alerts, err := g.store.QueryAlertsRange(nodeID, from, to)
	if err != nil {
		return nil, err
	}

	summary := &AlertSummary{
		Total:      len(alerts),
		ByType:     make(map[string]int),
		BySeverity: make(map[string]int),
	}

	for _, a := range alerts {
		summary.ByType[string(a.Type)]++
		summary.BySeverity[string(a.Severity)]++
	}

	// Recent 10
	recent := alerts
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	summary.Recent = recent

	return summary, nil
}

// GenerateTrend returns traffic trend data for a time range.
func (g *Generator) GenerateTrend(nodeID string, from, to time.Time) ([]TrendPoint, error) {
	granularity := bestGranularity(from, to)
	snaps, err := g.store.QuerySnapshotsGranular(nodeID, from, to, 0, granularity)
	if err != nil {
		return nil, err
	}

	var points []TrendPoint
	for _, s := range snaps {
		points = append(points, TrendPoint{
			Timestamp:     s.Timestamp,
			BytesPerSec:   s.BytesPerSec,
			PacketsPerSec: s.PacketsPerSec,
			FlowsCount:    s.FlowsCount,
		})
	}
	if points == nil {
		points = []TrendPoint{}
	}
	return points, nil
}

// bestGranularity picks the appropriate aggregation table based on the time range.
func bestGranularity(from, to time.Time) string {
	dur := to.Sub(from)
	switch {
	case dur <= 2*time.Hour:
		return "raw"
	case dur <= 48*time.Hour:
		return "hourly"
	case dur <= 14*24*time.Hour:
		return "daily"
	default:
		return "weekly"
	}
}

func snapInterval(from, to time.Time) int {
	switch bestGranularity(from, to) {
	case "raw":
		return 10
	case "hourly":
		return 3600
	case "daily":
		return 86400
	default:
		return 604800
	}
}

// TopHostRow is returned by QueryTopHosts.
type TopHostRow struct {
	IP            string
	Hostname      string
	TotalBytes    float64
	BytesSent     float64
	BytesReceived float64
	FlowCount     int
}

// TopProtocolRow is returned by QueryTopProtocols.
type TopProtocolRow struct {
	Name       string
	TotalBytes float64
	FlowCount  int
}
