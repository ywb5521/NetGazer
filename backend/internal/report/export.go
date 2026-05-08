package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/netgazer/backend/internal/models"
	"github.com/netgazer/backend/internal/storage"
)

// ExportFormat represents a supported export format.
type ExportFormat string

const (
	FormatJSON       ExportFormat = "json"
	FormatCSV        ExportFormat = "csv"
	FormatNDJSON     ExportFormat = "ndjson"
	FormatClickHouse ExportFormat = "clickhouse"
)

// Exporter writes data in various formats.
type Exporter struct {
	store *storage.Store
}

func NewExporter(store *storage.Store) *Exporter {
	return &Exporter{store: store}
}

// ExportSnapshots writes traffic snapshots in the requested format.
func (e *Exporter) ExportSnapshots(nodeID string, from, to time.Time, format ExportFormat, w io.Writer) error {
	snaps, err := e.store.QuerySnapshotsGranular(nodeID, from, to, 0, bestGranularity(from, to))
	if err != nil {
		return err
	}

	switch format {
	case FormatJSON:
		return writeJSON(w, snaps)
	case FormatCSV:
		return writeSnapshotCSV(w, snaps)
	case FormatNDJSON:
		return writeSnapshotNDJSON(w, snaps)
	case FormatClickHouse:
		return writeSnapshotClickHouse(w, snaps)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// ExportHostSnapshots writes host-level snapshots in the requested format.
func (e *Exporter) ExportHostSnapshots(nodeID string, from, to time.Time, format ExportFormat, w io.Writer) error {
	hosts, err := e.store.QueryHostSnapshotsRange(nodeID, from, to)
	if err != nil {
		return err
	}

	switch format {
	case FormatJSON:
		return writeJSON(w, hosts)
	case FormatCSV:
		return writeHostCSV(w, hosts)
	case FormatNDJSON:
		return writeHostNDJSON(w, hosts)
	case FormatClickHouse:
		return writeHostClickHouse(w, hosts)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// ExportAlerts writes alerts in the requested format.
func (e *Exporter) ExportAlerts(nodeID string, from, to time.Time, format ExportFormat, w io.Writer) error {
	alerts, err := e.store.QueryAlertsRange(nodeID, from, to)
	if err != nil {
		return err
	}

	switch format {
	case FormatJSON:
		return writeJSON(w, alerts)
	case FormatCSV:
		return writeAlertCSV(w, alerts)
	case FormatNDJSON:
		return writeAlertNDJSON(w, alerts)
	case FormatClickHouse:
		return writeAlertClickHouse(w, alerts)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// writeJSON writes data as a JSON array.
func writeJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// writeSnapshotCSV writes traffic snapshots as CSV.
func writeSnapshotCSV(w io.Writer, snaps []models.TrafficSnapshot) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"node_id", "timestamp", "bytes_per_sec", "packets_per_sec", "flows_count"})
	for _, s := range snaps {
		cw.Write([]string{
			s.NodeID,
			s.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%.2f", s.BytesPerSec),
			fmt.Sprintf("%.2f", s.PacketsPerSec),
			fmt.Sprintf("%d", s.FlowsCount),
		})
	}
	cw.Flush()
	return cw.Error()
}

// writeSnapshotNDJSON writes traffic snapshots as newline-delimited JSON (bulk ES format).
func writeSnapshotNDJSON(w io.Writer, snaps []models.TrafficSnapshot) error {
	enc := json.NewEncoder(w)
	for _, s := range snaps {
		// ES bulk format: action line + document line
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": "netgazer-traffic",
				"_id":    fmt.Sprintf("%s-%d", s.NodeID, s.Timestamp.UnixMilli()),
			},
		}
		if err := enc.Encode(action); err != nil {
			return err
		}
		if err := enc.Encode(docFromSnapshot(s)); err != nil {
			return err
		}
	}
	return nil
}

// writeSnapshotClickHouse writes traffic snapshots as tab-separated values for ClickHouse.
func writeSnapshotClickHouse(w io.Writer, snaps []models.TrafficSnapshot) error {
	for _, s := range snaps {
		line := fmt.Sprintf("%s\t%s\t%.2f\t%.2f\t%d\n",
			s.NodeID,
			s.Timestamp.Format("2006-01-02 15:04:05"),
			s.BytesPerSec,
			s.PacketsPerSec,
			s.FlowsCount,
		)
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

// writeHostCSV writes host snapshots as CSV.
func writeHostCSV(w io.Writer, hosts []models.HostSnapshot) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"node_id", "host_ip", "timestamp", "bytes_sent", "bytes_received", "packets_sent", "packets_received"})
	for _, h := range hosts {
		cw.Write([]string{
			h.NodeID, h.HostIP, h.Timestamp.Format(time.RFC3339),
			fmt.Sprintf("%.2f", h.BytesSent), fmt.Sprintf("%.2f", h.BytesReceived),
			fmt.Sprintf("%d", h.PacketsSent), fmt.Sprintf("%d", h.PacketsReceived),
		})
	}
	cw.Flush()
	return cw.Error()
}

// writeHostNDJSON writes host snapshots as ndjson (ES bulk format).
func writeHostNDJSON(w io.Writer, hosts []models.HostSnapshot) error {
	enc := json.NewEncoder(w)
	for _, h := range hosts {
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": "netgazer-hosts",
				"_id":    fmt.Sprintf("%s-%s-%d", h.NodeID, h.HostIP, h.Timestamp.UnixMilli()),
			},
		}
		if err := enc.Encode(action); err != nil {
			return err
		}
		if err := enc.Encode(h); err != nil {
			return err
		}
	}
	return nil
}

// writeHostClickHouse writes host snapshots as tab-separated values.
func writeHostClickHouse(w io.Writer, hosts []models.HostSnapshot) error {
	for _, h := range hosts {
		line := fmt.Sprintf("%s\t%s\t%s\t%.2f\t%.2f\t%d\t%d\n",
			h.NodeID, h.HostIP, h.Timestamp.Format("2006-01-02 15:04:05"),
			h.BytesSent, h.BytesReceived, h.PacketsSent, h.PacketsReceived,
		)
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

// writeAlertCSV writes alerts as CSV.
func writeAlertCSV(w io.Writer, alerts []models.Alert) error {
	cw := csv.NewWriter(w)
	cw.Write([]string{"id", "type", "severity", "message", "source_ip", "node_id", "timestamp", "acknowledged"})
	for _, a := range alerts {
		ack := "false"
		if a.Acknowledged {
			ack = "true"
		}
		cw.Write([]string{
			a.ID, string(a.Type), string(a.Severity), a.Message,
			a.SourceIP, a.NodeID, a.Timestamp.Format(time.RFC3339), ack,
		})
	}
	cw.Flush()
	return cw.Error()
}

// writeAlertNDJSON writes alerts as ndjson (ES bulk format).
func writeAlertNDJSON(w io.Writer, alerts []models.Alert) error {
	enc := json.NewEncoder(w)
	for _, a := range alerts {
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": "netgazer-alerts",
				"_id":    a.ID,
			},
		}
		if err := enc.Encode(action); err != nil {
			return err
		}
		if err := enc.Encode(a); err != nil {
			return err
		}
	}
	return nil
}

// writeAlertClickHouse writes alerts as tab-separated values.
func writeAlertClickHouse(w io.Writer, alerts []models.Alert) error {
	for _, a := range alerts {
		ack := "0"
		if a.Acknowledged {
			ack = "1"
		}
		line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			a.ID, a.Type, a.Severity, escapeTSV(a.Message),
			a.SourceIP, a.NodeID,
			a.Timestamp.Format("2006-01-02 15:04:05"), ack,
		)
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

func escapeTSV(s string) string {
	// Replace tabs and newlines in text fields for ClickHouse TSV
	result := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		switch c {
		case '\t':
			result = append(result, ' ')
		case '\n':
			result = append(result, ' ')
		case '\r':
			result = append(result, ' ')
		default:
			result = append(result, c)
		}
	}
	return string(result)
}

func docFromSnapshot(s models.TrafficSnapshot) map[string]interface{} {
	return map[string]interface{}{
		"timestamp":       s.Timestamp.Format(time.RFC3339Nano),
		"node_id":         s.NodeID,
		"bytes_per_sec":   s.BytesPerSec,
		"packets_per_sec": s.PacketsPerSec,
		"flows_count":     s.FlowsCount,
	}
}
