package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/netgazer/backend/internal/models"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS traffic_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			interface TEXT NOT NULL DEFAULT '',
			timestamp INTEGER NOT NULL,
			bytes_per_sec REAL NOT NULL DEFAULT 0,
			packets_per_sec REAL NOT NULL DEFAULT 0,
			flows_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_ts ON traffic_snapshots(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_node ON traffic_snapshots(node_id)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			severity TEXT NOT NULL,
			message TEXT NOT NULL,
			source_ip TEXT DEFAULT '',
			node_id TEXT DEFAULT '',
			timestamp INTEGER NOT NULL,
			acknowledged BOOLEAN DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_ts ON alerts(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_node ON alerts(node_id)`,
		`CREATE TABLE IF NOT EXISTS host_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			interface TEXT NOT NULL DEFAULT '',
			host_ip TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			bytes_sent REAL NOT NULL DEFAULT 0,
			bytes_received REAL NOT NULL DEFAULT 0,
			packets_sent INTEGER NOT NULL DEFAULT 0,
			packets_received INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_host_snap_ts ON host_snapshots(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_host_snap_ip ON host_snapshots(host_ip)`,
		`CREATE TABLE IF NOT EXISTS traffic_hourly (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			interface TEXT NOT NULL DEFAULT '',
			timestamp INTEGER NOT NULL,
			bytes_per_sec REAL NOT NULL DEFAULT 0,
			packets_per_sec REAL NOT NULL DEFAULT 0,
			flows_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hourly_ts ON traffic_hourly(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_hourly_node ON traffic_hourly(node_id)`,
		`CREATE TABLE IF NOT EXISTS traffic_daily (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			interface TEXT NOT NULL DEFAULT '',
			timestamp INTEGER NOT NULL,
			bytes_per_sec REAL NOT NULL DEFAULT 0,
			packets_per_sec REAL NOT NULL DEFAULT 0,
			flows_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_ts ON traffic_daily(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_node ON traffic_daily(node_id)`,
		`CREATE TABLE IF NOT EXISTS traffic_weekly (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			interface TEXT NOT NULL DEFAULT '',
			timestamp INTEGER NOT NULL,
			bytes_per_sec REAL NOT NULL DEFAULT 0,
			packets_per_sec REAL NOT NULL DEFAULT 0,
			flows_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_weekly_ts ON traffic_weekly(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_weekly_node ON traffic_weekly(node_id)`,
		`CREATE TABLE IF NOT EXISTS notification_channels (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			enabled BOOLEAN DEFAULT 1,
			config TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE TABLE IF NOT EXISTS config_kv (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS syslog (
			id TEXT PRIMARY KEY,
			timestamp INTEGER NOT NULL,
			facility TEXT NOT NULL DEFAULT '',
			severity TEXT NOT NULL DEFAULT 'info',
			hostname TEXT NOT NULL DEFAULT '',
			app_name TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_syslog_ts ON syslog(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_syslog_source ON syslog(source)`,
		`CREATE TABLE IF NOT EXISTS traffic_matrix_snapshots (
			timestamp INTEGER NOT NULL,
			source TEXT NOT NULL,
			dest TEXT NOT NULL,
			bytes INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_matrix_ts ON traffic_matrix_snapshots(timestamp)`,
		`CREATE TABLE IF NOT EXISTS voip_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ssrc INTEGER NOT NULL,
			src_ip TEXT NOT NULL,
			dst_ip TEXT NOT NULL,
			src_port INTEGER NOT NULL,
			dst_port INTEGER NOT NULL,
			packets INTEGER NOT NULL DEFAULT 0,
			bytes INTEGER NOT NULL DEFAULT 0,
			lost_packets INTEGER NOT NULL DEFAULT 0,
			jitter_ms REAL NOT NULL DEFAULT 0,
			mos REAL NOT NULL DEFAULT 0,
			codec TEXT NOT NULL DEFAULT '',
			timestamp INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_voip_ts ON voip_sessions(timestamp)`,
		`CREATE TABLE IF NOT EXISTS intercept_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			expression TEXT NOT NULL,
			action TEXT NOT NULL DEFAULT 'block',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS node_tokens (
			id TEXT PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at INTEGER NOT NULL,
			last_used_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS host_pools (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			cidrs TEXT NOT NULL DEFAULT '[]',
			created_at INTEGER NOT NULL
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}

	// Migration: add interface column to existing tables (ignore error if already exists)
	migrations := []string{
		`ALTER TABLE traffic_snapshots ADD COLUMN interface TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE host_snapshots ADD COLUMN interface TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE traffic_hourly ADD COLUMN interface TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE traffic_daily ADD COLUMN interface TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE traffic_weekly ADD COLUMN interface TEXT NOT NULL DEFAULT ''`,
	}
	for _, m := range migrations {
		db.Exec(m) // ignore error (column may already exist)
	}
	return nil
}

func (s *Store) InsertSnapshot(snap models.TrafficSnapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO traffic_snapshots (node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count)
		 VALUES (?, ?, ?, ?, ?)`,
		snap.NodeID, snap.Timestamp.UnixMilli(), snap.BytesPerSec, snap.PacketsPerSec, snap.FlowsCount,
	)
	return err
}

func (s *Store) InsertAlert(alert models.Alert) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO alerts (id, type, severity, message, source_ip, node_id, timestamp, acknowledged)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		alert.ID, string(alert.Type), string(alert.Severity), alert.Message,
		alert.SourceIP, alert.NodeID, alert.Timestamp.UnixMilli(), alert.Acknowledged,
	)
	return err
}

func (s *Store) QuerySnapshots(nodeID string, from, to time.Time, intervalSec int) ([]models.TrafficSnapshot, error) {
	return s.QuerySnapshotsGranular(nodeID, from, to, intervalSec, "raw")
}

func (s *Store) QuerySnapshotsGranular(nodeID string, from, to time.Time, intervalSec int, granularity string) ([]models.TrafficSnapshot, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	table := "traffic_snapshots"
	switch granularity {
	case "hourly":
		table = "traffic_hourly"
	case "daily":
		table = "traffic_daily"
	case "weekly":
		table = "traffic_weekly"
	}

	query := fmt.Sprintf(`SELECT node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count
		FROM %s WHERE timestamp >= ? AND timestamp <= ?`, table)
	args := []interface{}{fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	query += ` ORDER BY timestamp ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []models.TrafficSnapshot
	for rows.Next() {
		var snap models.TrafficSnapshot
		var ts int64
		if err := rows.Scan(&snap.NodeID, &ts, &snap.BytesPerSec, &snap.PacketsPerSec, &snap.FlowsCount); err != nil {
			return nil, err
		}
		snap.Timestamp = time.UnixMilli(ts)
		snapshots = append(snapshots, snap)
	}

	// Downsample only for raw data
	if granularity == "raw" && intervalSec > 1 && len(snapshots) > 0 {
		snapshots = downsample(snapshots, intervalSec)
	}

	if snapshots == nil {
		snapshots = []models.TrafficSnapshot{}
	}

	return snapshots, nil
}

func (s *Store) InsertHostSnapshots(snaps []models.HostSnapshot) error {
	for _, snap := range snaps {
		_, err := s.db.Exec(
			`INSERT INTO host_snapshots (node_id, host_ip, timestamp, bytes_sent, bytes_received, packets_sent, packets_received)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			snap.NodeID, snap.HostIP, snap.Timestamp.UnixMilli(),
			snap.BytesSent, snap.BytesReceived, snap.PacketsSent, snap.PacketsReceived,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) QueryHostSnapshots(hostIP, nodeID string, from, to time.Time) ([]models.HostSnapshot, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	query := `SELECT node_id, host_ip, timestamp, bytes_sent, bytes_received, packets_sent, packets_received
		FROM host_snapshots WHERE host_ip = ? AND timestamp >= ? AND timestamp <= ?`
	args := []interface{}{hostIP, fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	query += ` ORDER BY timestamp ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []models.HostSnapshot
	for rows.Next() {
		var snap models.HostSnapshot
		var ts int64
		if err := rows.Scan(&snap.NodeID, &snap.HostIP, &ts, &snap.BytesSent, &snap.BytesReceived, &snap.PacketsSent, &snap.PacketsReceived); err != nil {
			return nil, err
		}
		snap.Timestamp = time.UnixMilli(ts)
		snapshots = append(snapshots, snap)
	}
	if snapshots == nil {
		snapshots = []models.HostSnapshot{}
	}
	return snapshots, nil
}

func (s *Store) QueryAlerts(nodeID string, limit, offset int) ([]models.Alert, error) {
	query := `SELECT id, type, severity, message, source_ip, node_id, timestamp, acknowledged
		FROM alerts`
	var args []interface{}

	if nodeID != "" {
		query += ` WHERE node_id = ?`
		args = append(args, nodeID)
	}
	query += ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var ts int64
		if err := rows.Scan(&a.ID, &a.Type, &a.Severity, &a.Message, &a.SourceIP, &a.NodeID, &ts, &a.Acknowledged); err != nil {
			return nil, err
		}
		a.Timestamp = time.UnixMilli(ts)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (s *Store) AcknowledgeAlert(id string) error {
	_, err := s.db.Exec(`UPDATE alerts SET acknowledged = 1 WHERE id = ?`, id)
	return err
}

func (s *Store) CleanupBefore(before time.Time) (int64, error) {
	beforeMs := before.UnixMilli()
	res, err := s.db.Exec(`DELETE FROM traffic_snapshots WHERE timestamp < ?`, beforeMs)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	s.db.Exec(`DELETE FROM alerts WHERE timestamp < ? AND acknowledged = 1`, beforeMs)
	s.db.Exec(`DELETE FROM host_snapshots WHERE timestamp < ?`, beforeMs)
	return n, nil
}

// AggregateToHourly aggregates raw traffic_snapshots older than cutoff into traffic_hourly.
func (s *Store) AggregateToHourly(cutoff time.Time) error {
	cutoffMs := cutoff.UnixMilli()
	// Query raw data older than cutoff
	rows, err := s.db.Query(
		`SELECT node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count
		FROM traffic_snapshots WHERE timestamp < ? ORDER BY node_id, timestamp ASC`, cutoffMs)
	if err != nil {
		return fmt.Errorf("query raw: %w", err)
	}
	defer rows.Close()

	type bucket struct {
		nodeID   string
		ts       int64
		sumBytes float64
		sumPkts  float64
		sumFlows float64
		count    int
	}
	buckets := make(map[int64]*bucket) // key: hour-aligned ts * 100000 + node idx
	nodeIdx := make(map[string]int)
	nextNodeIdx := 1

	hourMs := int64(3600 * 1000)
	var idsToDelete []int64

	for rows.Next() {
		var snap models.TrafficSnapshot
		var ts int64
		if err := rows.Scan(&snap.NodeID, &ts, &snap.BytesPerSec, &snap.PacketsPerSec, &snap.FlowsCount); err != nil {
			return err
		}
		hourTs := (ts / hourMs) * hourMs

		if _, ok := nodeIdx[snap.NodeID]; !ok {
			nodeIdx[snap.NodeID] = nextNodeIdx
			nextNodeIdx++
		}
		key := hourTs*100000 + int64(nodeIdx[snap.NodeID])

		if b, ok := buckets[key]; ok {
			b.sumBytes += snap.BytesPerSec
			b.sumPkts += snap.PacketsPerSec
			b.sumFlows += float64(snap.FlowsCount)
			b.count++
		} else {
			buckets[key] = &bucket{
				nodeID:   snap.NodeID,
				ts:       hourTs,
				sumBytes: snap.BytesPerSec,
				sumPkts:  snap.PacketsPerSec,
				sumFlows: float64(snap.FlowsCount),
				count:    1,
			}
		}
		idsToDelete = append(idsToDelete, int64(snap.FlowsCount)) // placeholder, we use max timestamp for deletion
	}

	// Insert aggregated rows
	for _, b := range buckets {
		avgBytes := b.sumBytes / float64(b.count)
		avgPkts := b.sumPkts / float64(b.count)
		avgFlows := int(b.sumFlows / float64(b.count))
		_, err := s.db.Exec(
			`INSERT INTO traffic_hourly (node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count)
			VALUES (?, ?, ?, ?, ?)`, b.nodeID, b.ts, avgBytes, avgPkts, avgFlows)
		if err != nil {
			return fmt.Errorf("insert hourly: %w", err)
		}
	}

	// Delete raw data that was aggregated
	if len(buckets) > 0 {
		_, err = s.db.Exec(`DELETE FROM traffic_snapshots WHERE timestamp < ?`, cutoffMs)
		if err != nil {
			return fmt.Errorf("delete raw: %w", err)
		}
	}

	return nil
}

// AggregateToDaily aggregates hourly data older than cutoff into traffic_daily.
func (s *Store) AggregateToWeekly(cutoff time.Time) error {
	cutoffMs := cutoff.UnixMilli()
	rows, err := s.db.Query(
		`SELECT node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count
		FROM traffic_daily WHERE timestamp < ? ORDER BY node_id, timestamp ASC`, cutoffMs)
	if err != nil {
		return fmt.Errorf("query daily: %w", err)
	}
	defer rows.Close()

	type bucket struct {
		nodeID   string
		ts       int64
		sumBytes float64
		sumPkts  float64
		sumFlows float64
		count    int
	}
	buckets := make(map[int64]*bucket)
	nodeIdx := make(map[string]int)
	nextNodeIdx := 1
	weekMs := int64(7 * 86400 * 1000)

	for rows.Next() {
		var nodeID string
		var ts int64
		var bps, pps float64
		var fc int
		if err := rows.Scan(&nodeID, &ts, &bps, &pps, &fc); err != nil {
			return err
		}
		weekTs := (ts / weekMs) * weekMs

		if _, ok := nodeIdx[nodeID]; !ok {
			nodeIdx[nodeID] = nextNodeIdx
			nextNodeIdx++
		}
		key := weekTs*100000 + int64(nodeIdx[nodeID])

		if b, ok := buckets[key]; ok {
			b.sumBytes += bps
			b.sumPkts += pps
			b.sumFlows += float64(fc)
			b.count++
		} else {
			buckets[key] = &bucket{
				nodeID:   nodeID,
				ts:       weekTs,
				sumBytes: bps,
				sumPkts:  pps,
				sumFlows: float64(fc),
				count:    1,
			}
		}
	}

	for _, b := range buckets {
		avgBytes := b.sumBytes / float64(b.count)
		avgPkts := b.sumPkts / float64(b.count)
		avgFlows := int(b.sumFlows / float64(b.count))
		_, err := s.db.Exec(
			`INSERT INTO traffic_weekly (node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count)
			VALUES (?, ?, ?, ?, ?)`, b.nodeID, b.ts, avgBytes, avgPkts, avgFlows)
		if err != nil {
			return fmt.Errorf("insert weekly: %w", err)
		}
	}

	if len(buckets) > 0 {
		_, err = s.db.Exec(`DELETE FROM traffic_daily WHERE timestamp < ?`, cutoffMs)
		if err != nil {
			return fmt.Errorf("delete daily: %w", err)
		}
	}

	return nil
}
func (s *Store) AggregateToDaily(cutoff time.Time) error {
	cutoffMs := cutoff.UnixMilli()
	rows, err := s.db.Query(
		`SELECT node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count
		FROM traffic_hourly WHERE timestamp < ? ORDER BY node_id, timestamp ASC`, cutoffMs)
	if err != nil {
		return fmt.Errorf("query hourly: %w", err)
	}
	defer rows.Close()

	type bucket struct {
		nodeID   string
		ts       int64
		sumBytes float64
		sumPkts  float64
		sumFlows float64
		count    int
	}
	buckets := make(map[int64]*bucket)
	nodeIdx := make(map[string]int)
	nextNodeIdx := 1
	dayMs := int64(86400 * 1000)

	for rows.Next() {
		var nodeID string
		var ts int64
		var bps, pps float64
		var fc int
		if err := rows.Scan(&nodeID, &ts, &bps, &pps, &fc); err != nil {
			return err
		}
		dayTs := (ts / dayMs) * dayMs

		if _, ok := nodeIdx[nodeID]; !ok {
			nodeIdx[nodeID] = nextNodeIdx
			nextNodeIdx++
		}
		key := dayTs*100000 + int64(nodeIdx[nodeID])

		if b, ok := buckets[key]; ok {
			b.sumBytes += bps
			b.sumPkts += pps
			b.sumFlows += float64(fc)
			b.count++
		} else {
			buckets[key] = &bucket{
				nodeID:   nodeID,
				ts:       dayTs,
				sumBytes: bps,
				sumPkts:  pps,
				sumFlows: float64(fc),
				count:    1,
			}
		}
	}

	for _, b := range buckets {
		avgBytes := b.sumBytes / float64(b.count)
		avgPkts := b.sumPkts / float64(b.count)
		avgFlows := int(b.sumFlows / float64(b.count))
		_, err := s.db.Exec(
			`INSERT INTO traffic_daily (node_id, timestamp, bytes_per_sec, packets_per_sec, flows_count)
			VALUES (?, ?, ?, ?, ?)`, b.nodeID, b.ts, avgBytes, avgPkts, avgFlows)
		if err != nil {
			return fmt.Errorf("insert daily: %w", err)
		}
	}

	if len(buckets) > 0 {
		_, err = s.db.Exec(`DELETE FROM traffic_hourly WHERE timestamp < ?`, cutoffMs)
		if err != nil {
			return fmt.Errorf("delete hourly: %w", err)
		}
	}

	return nil
}

func (s *Store) ListChannels() ([]models.NotificationChannel, error) {
	rows, err := s.db.Query(`SELECT id, name, type, enabled, config FROM notification_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var channels []models.NotificationChannel
	for rows.Next() {
		var ch models.NotificationChannel
		var configStr string
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Type, &ch.Enabled, &configStr); err != nil {
			return nil, err
		}
		ch.Config = json.RawMessage(configStr)
		channels = append(channels, ch)
	}
	if channels == nil {
		channels = []models.NotificationChannel{}
	}
	return channels, nil
}

func (s *Store) SaveChannel(ch models.NotificationChannel) error {
	configStr := string(ch.Config)
	if configStr == "" {
		configStr = "{}"
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO notification_channels (id, name, type, enabled, config) VALUES (?, ?, ?, ?, ?)`,
		ch.ID, ch.Name, string(ch.Type), ch.Enabled, configStr,
	)
	return err
}

func (s *Store) GetChannel(id string) (*models.NotificationChannel, error) {
	var ch models.NotificationChannel
	var configStr string
	err := s.db.QueryRow(`SELECT id, name, type, enabled, config FROM notification_channels WHERE id = ?`, id).
		Scan(&ch.ID, &ch.Name, &ch.Type, &ch.Enabled, &configStr)
	if err != nil {
		return nil, err
	}
	ch.Config = json.RawMessage(configStr)
	return &ch, nil
}

func (s *Store) DeleteChannel(id string) error {
	_, err := s.db.Exec(`DELETE FROM notification_channels WHERE id = ?`, id)
	return err
}

func (s *Store) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM config_kv WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO config_kv (key, value) VALUES (?, ?)`, key, value)
	return err
}

func (s *Store) CleanupDaily(before time.Time) (int64, error) {
	beforeMs := before.UnixMilli()
	res, err := s.db.Exec(`DELETE FROM traffic_daily WHERE timestamp < ?`, beforeMs)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) CleanupWeekly(before time.Time) (int64, error) {
	beforeMs := before.UnixMilli()
	res, err := s.db.Exec(`DELETE FROM traffic_weekly WHERE timestamp < ?`, beforeMs)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func downsample(snapshots []models.TrafficSnapshot, intervalSec int) []models.TrafficSnapshot {
	if len(snapshots) == 0 {
		return snapshots
	}
	var result []models.TrafficSnapshot
	interval := time.Duration(intervalSec) * time.Second
	windowStart := snapshots[0].Timestamp
	var sum models.TrafficSnapshot
	count := 0

	flush := func(ts time.Time) {
		if count > 0 {
			sum.BytesPerSec /= float64(count)
			sum.PacketsPerSec /= float64(count)
			sum.Timestamp = ts
			result = append(result, sum)
		}
	}

	for _, s := range snapshots {
		if s.Timestamp.Sub(windowStart) >= interval {
			flush(windowStart)
			windowStart = s.Timestamp
			sum = models.TrafficSnapshot{NodeID: s.NodeID}
			count = 0
		}
		sum.BytesPerSec += s.BytesPerSec
		sum.PacketsPerSec += s.PacketsPerSec
		sum.FlowsCount += s.FlowsCount
		count++
	}
	flush(windowStart)

	return result
}

// ---- Users ----

type User struct {
	ID           int
	Username     string
	PasswordHash string
	CreatedAt    int64
}

func (s *Store) HasUsers() bool {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (s *Store) CreateUser(username, passwordHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, time.Now().Unix(),
	)
	return err
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`,
		username,
	)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

// QueryUniqueHosts returns the count of distinct host IPs in a time range.
func (s *Store) QueryUniqueHosts(nodeID string, from, to time.Time) (int, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	query := `SELECT COUNT(DISTINCT host_ip) FROM host_snapshots WHERE timestamp >= ? AND timestamp <= ?`
	args := []interface{}{fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}

	var count int
	if err := s.db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CountAlerts returns the count of alerts in a time range.
func (s *Store) CountAlerts(nodeID string, from, to time.Time) (int, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	query := `SELECT COUNT(*) FROM alerts WHERE timestamp >= ? AND timestamp <= ?`
	args := []interface{}{fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}

	var count int
	if err := s.db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// QueryAlertsRange returns all alerts in a time range.
func (s *Store) QueryAlertsRange(nodeID string, from, to time.Time) ([]models.Alert, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	query := `SELECT id, type, severity, message, source_ip, node_id, timestamp, acknowledged
		FROM alerts WHERE timestamp >= ? AND timestamp <= ?`
	args := []interface{}{fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	query += ` ORDER BY timestamp DESC LIMIT 1000`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var ts int64
		if err := rows.Scan(&a.ID, &a.Type, &a.Severity, &a.Message, &a.SourceIP, &a.NodeID, &ts, &a.Acknowledged); err != nil {
			return nil, err
		}
		a.Timestamp = time.UnixMilli(ts)
		alerts = append(alerts, a)
	}
	if alerts == nil {
		alerts = []models.Alert{}
	}
	return alerts, nil
}

// QueryHostSnapshotsRange returns all host snapshots in a time range.
func (s *Store) QueryHostSnapshotsRange(nodeID string, from, to time.Time) ([]models.HostSnapshot, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	query := `SELECT node_id, host_ip, timestamp, bytes_sent, bytes_received, packets_sent, packets_received
		FROM host_snapshots WHERE timestamp >= ? AND timestamp <= ?`
	args := []interface{}{fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	query += ` ORDER BY timestamp ASC LIMIT 10000`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []models.HostSnapshot
	for rows.Next() {
		var snap models.HostSnapshot
		var ts int64
		if err := rows.Scan(&snap.NodeID, &snap.HostIP, &ts, &snap.BytesSent, &snap.BytesReceived, &snap.PacketsSent, &snap.PacketsReceived); err != nil {
			return nil, err
		}
		snap.Timestamp = time.UnixMilli(ts)
		snapshots = append(snapshots, snap)
	}
	if snapshots == nil {
		snapshots = []models.HostSnapshot{}
	}
	return snapshots, nil
}

// QueryTopHosts returns the top N hosts by total traffic in a time range.
func (s *Store) QueryTopHosts(nodeID string, from, to time.Time, limit int) ([]TopHostRow, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	query := `SELECT host_ip, SUM(bytes_sent) as bs, SUM(bytes_received) as br,
		COUNT(*) as fc
		FROM host_snapshots WHERE timestamp >= ? AND timestamp <= ?`
	args := []interface{}{fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	query += ` GROUP BY host_ip ORDER BY (bs + br) DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []TopHostRow
	for rows.Next() {
		var row TopHostRow
		var bs, br float64
		if err := rows.Scan(&row.IP, &bs, &br, &row.FlowCount); err != nil {
			return nil, err
		}
		row.BytesSent = bs
		row.BytesReceived = br
		row.TotalBytes = bs + br
		hosts = append(hosts, row)
	}
	if hosts == nil {
		hosts = []TopHostRow{}
	}
	return hosts, nil
}

// QueryTopProtocols returns the top N protocols by total traffic in a time range.
func (s *Store) QueryTopProtocols(nodeID string, from, to time.Time, limit int) ([]TopProtocolRow, error) {
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	// Note: protocol data is stored within host_snapshots. We approximate by summing all traffic as
	// protocol data is not disaggregated in the stored history. For a real implementation, we'd need
	// a protocol-level history table. Instead, we approximate with traffic totals across nodes.
	query := `SELECT node_id, SUM(bytes_per_sec) as total_bytes, SUM(flows_count) as fc
		FROM traffic_snapshots WHERE timestamp >= ? AND timestamp <= ?`
	args := []interface{}{fromMs, toMs}

	if nodeID != "" {
		query += ` AND node_id = ?`
		args = append(args, nodeID)
	}
	query += ` GROUP BY node_id ORDER BY total_bytes DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var protocols []TopProtocolRow
	for rows.Next() {
		var row TopProtocolRow
		var tb float64
		if err := rows.Scan(&row.Name, &tb, &row.FlowCount); err != nil {
			return nil, err
		}
		row.TotalBytes = tb
		protocols = append(protocols, row)
	}
	if protocols == nil {
		protocols = []TopProtocolRow{}
	}
	return protocols, nil
}

// TopHostRow is a row from QueryTopHosts.
type TopHostRow struct {
	IP            string
	Hostname      string
	TotalBytes    float64
	BytesSent     float64
	BytesReceived float64
	FlowCount     int
}

// TopProtocolRow is a row from QueryTopProtocols.
type TopProtocolRow struct {
	Name       string
	TotalBytes float64
	FlowCount  int
}

// SyslogRecord is a row from QuerySyslog.
type SyslogRecord struct {
	ID        string
	Timestamp int64
	Facility  string
	Severity  string
	Hostname  string
	AppName   string
	Message   string
	Source    string
}

// InsertSyslog stores a syslog message.
func (s *Store) InsertSyslog(id string, ts int64, facility, severity, hostname, appName, message, source string) error {
	_, err := s.db.Exec(
		`INSERT INTO syslog (id, timestamp, facility, severity, hostname, app_name, message, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, ts, facility, severity, hostname, appName, message, source,
	)
	return err
}

// QuerySyslog returns paginated syslog messages.
func (s *Store) QuerySyslog(severity, source string, limit, offset int) ([]SyslogRecord, int, error) {
	where := ""
	args := make([]any, 0)
	if severity != "" {
		where = " WHERE severity = ?"
		args = append(args, severity)
	}
	if source != "" {
		if where == "" {
			where = " WHERE source = ?"
		} else {
			where += " AND source = ?"
		}
		args = append(args, source)
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM syslog" + where
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	queryArgs := append(args, limit, offset)
	rows, err := s.db.Query(
		`SELECT id, timestamp, facility, severity, hostname, app_name, message, source
		 FROM syslog`+where+` ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		queryArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []SyslogRecord
	for rows.Next() {
		var r SyslogRecord
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Facility, &r.Severity, &r.Hostname, &r.AppName, &r.Message, &r.Source); err != nil {
			return nil, 0, err
		}
		records = append(records, r)
	}
	return records, total, rows.Err()
}

// InsertMatrixSnapshot stores a traffic matrix cell for a given time.
func (s *Store) InsertMatrixSnapshot(source, dest string, bytes uint64, ts int64) error {
	_, err := s.db.Exec(
		`INSERT INTO traffic_matrix_snapshots (timestamp, source, dest, bytes) VALUES (?, ?, ?, ?)`,
		ts, source, dest, bytes,
	)
	return err
}

// QueryMatrixHistory returns matrix snapshots within a time range.
func (s *Store) QueryMatrixHistory(from, to int64, limit int) ([]TrafficMatrixCell, error) {
	rows, err := s.db.Query(
		`SELECT timestamp, source, dest, bytes FROM traffic_matrix_snapshots
		 WHERE timestamp BETWEEN ? AND ? ORDER BY timestamp DESC LIMIT ?`,
		from, to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []TrafficMatrixCell
	for rows.Next() {
		var c TrafficMatrixCell
		if err := rows.Scan(&c.Timestamp, &c.Source, &c.Dest, &c.Bytes); err != nil {
			return nil, err
		}
		cells = append(cells, c)
	}
	return cells, rows.Err()
}

// TrafficMatrixCell is a row from the traffic matrix history.
type TrafficMatrixCell struct {
	Timestamp int64
	Source    string
	Dest      string
	Bytes     uint64
}

// InsertVoipSession stores a VoIP session snapshot.
func (s *Store) InsertVoipSession(ssrc uint32, srcIP, dstIP string, srcPort, dstPort uint16, packets, bytes, lostPkts int64, jitterMS, mos float64, codec string, ts int64) error {
	_, err := s.db.Exec(
		`INSERT INTO voip_sessions (ssrc, src_ip, dst_ip, src_port, dst_port, packets, bytes, lost_packets, jitter_ms, mos, codec, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ssrc, srcIP, dstIP, srcPort, dstPort, packets, bytes, lostPkts, jitterMS, mos, codec, ts,
	)
	return err
}

// QueryVoipSessions returns VoIP sessions within a time range.
func (s *Store) QueryVoipSessions(since, until int64) ([]VoipSessionRecord, error) {
	rows, err := s.db.Query(
		`SELECT ssrc, src_ip, dst_ip, src_port, dst_port, packets, bytes, lost_packets, jitter_ms, mos, codec, timestamp
		 FROM voip_sessions WHERE timestamp >= ? AND timestamp <= ? ORDER BY timestamp DESC LIMIT 500`,
		since, until,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []VoipSessionRecord
	for rows.Next() {
		var r VoipSessionRecord
		if err := rows.Scan(&r.SSRC, &r.SrcIP, &r.DstIP, &r.SrcPort, &r.DstPort, &r.Packets, &r.Bytes, &r.LostPackets, &r.JitterMS, &r.MOS, &r.Codec, &r.Timestamp); err != nil {
			return nil, err
		}
		sessions = append(sessions, r)
	}
	return sessions, rows.Err()
}

// VoipSessionRecord represents a stored VoIP session.
type VoipSessionRecord struct {
	SSRC        uint32
	SrcIP       string
	DstIP       string
	SrcPort     uint16
	DstPort     uint16
	Packets     int64
	Bytes       int64
	LostPackets int64
	JitterMS    float64
	MOS         float64
	Codec       string
	Timestamp   int64
}

// InterceptRuleRecord represents an intercept rule stored in the database.
type InterceptRuleRecord struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Expression string `json:"expression"`
	Action     string `json:"action"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

// ListInterceptRules returns all intercept rules.
func (s *Store) ListInterceptRules() ([]InterceptRuleRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, name, expression, action, enabled, created_at, updated_at
		 FROM intercept_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []InterceptRuleRecord
	for rows.Next() {
		var r InterceptRuleRecord
		if err := rows.Scan(&r.ID, &r.Name, &r.Expression, &r.Action, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// CreateInterceptRule inserts a new intercept rule.
func (s *Store) CreateInterceptRule(r InterceptRuleRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO intercept_rules (id, name, expression, action, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Expression, r.Action, r.Enabled, r.CreatedAt, r.UpdatedAt)
	return err
}

// UpdateInterceptRule updates an existing intercept rule.
func (s *Store) UpdateInterceptRule(r InterceptRuleRecord) error {
	_, err := s.db.Exec(
		`UPDATE intercept_rules SET name=?, expression=?, action=?, enabled=?, updated_at=? WHERE id=?`,
		r.Name, r.Expression, r.Action, r.Enabled, r.UpdatedAt, r.ID)
	return err
}

// DeleteInterceptRule deletes an intercept rule by ID.
func (s *Store) DeleteInterceptRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM intercept_rules WHERE id=?`, id)
	return err
}

// GetInterceptRule returns a single intercept rule by ID.
func (s *Store) GetInterceptRule(id string) (*InterceptRuleRecord, error) {
	row := s.db.QueryRow(
		`SELECT id, name, expression, action, enabled, created_at, updated_at
		 FROM intercept_rules WHERE id=?`, id)
	var r InterceptRuleRecord
	err := row.Scan(&r.ID, &r.Name, &r.Expression, &r.Action, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateNodeToken inserts a new node auth token.
func (s *Store) CreateNodeToken(id, token, description string) error {
	_, err := s.db.Exec(
		`INSERT INTO node_tokens (id, token, description, enabled, created_at)
		 VALUES (?, ?, ?, 1, ?)`,
		id, token, description, time.Now().UnixMilli())
	return err
}

// NodeTokenRecord mirrors the node_tokens table row.
type NodeTokenRecord struct {
	ID          string
	Token       string
	Description string
	Enabled     bool
	CreatedAt   int64
	LastUsedAt  *int64
}

// ListNodeTokens returns all node tokens, ordered by creation time.
func (s *Store) ListNodeTokens() ([]NodeTokenRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, token, description, enabled, created_at, last_used_at
		 FROM node_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []NodeTokenRecord
	for rows.Next() {
		var t NodeTokenRecord
		var enabled int
		var lastUsed sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Token, &t.Description, &enabled, &t.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		t.Enabled = enabled == 1
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.Int64
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteNodeToken deletes a node token by ID.
func (s *Store) DeleteNodeToken(id string) error {
	_, err := s.db.Exec(`DELETE FROM node_tokens WHERE id=?`, id)
	return err
}

// ValidateNodeToken checks whether a token exists and is enabled.
// Updates last_used_at on success.
func (s *Store) ValidateNodeToken(token string) (bool, error) {
	var enabled int
	err := s.db.QueryRow(
		`SELECT enabled FROM node_tokens WHERE token=?`, token).Scan(&enabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	s.db.Exec(`UPDATE node_tokens SET last_used_at=? WHERE token=?`,
		time.Now().UnixMilli(), token)
	return enabled == 1, nil
}
