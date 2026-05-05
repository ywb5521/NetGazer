package storage

import (
	"encoding/json"
	"time"
)

// HostPoolRecord represents a row in the host_pools table.
type HostPoolRecord struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	CIDRs       []string `json:"cidrs"`
	CreatedAt   int64    `json:"created_at"`
}

// CreateHostPool inserts a new host pool.
func (s *Store) CreateHostPool(id, name, description string, cidrs []string) error {
	cidrJSON, err := json.Marshal(cidrs)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO host_pools (id, name, description, cidrs, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		id, name, description, string(cidrJSON), time.Now().UnixMilli())
	return err
}

// ListHostPools returns all host pools ordered by creation time.
func (s *Store) ListHostPools() ([]HostPoolRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, cidrs, created_at
		 FROM host_pools ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pools []HostPoolRecord
	for rows.Next() {
		var r HostPoolRecord
		var cidrJSON string
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &cidrJSON, &r.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(cidrJSON), &r.CIDRs)
		if r.CIDRs == nil {
			r.CIDRs = []string{}
		}
		pools = append(pools, r)
	}
	return pools, rows.Err()
}

// UpdateHostPool updates an existing host pool.
func (s *Store) UpdateHostPool(id, name, description string, cidrs []string) error {
	cidrJSON, err := json.Marshal(cidrs)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE host_pools SET name=?, description=?, cidrs=? WHERE id=?`,
		name, description, string(cidrJSON), id)
	return err
}

// DeleteHostPool deletes a host pool by ID.
func (s *Store) DeleteHostPool(id string) error {
	_, err := s.db.Exec(`DELETE FROM host_pools WHERE id=?`, id)
	return err
}
