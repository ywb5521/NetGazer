package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/netgazer/backend/internal/storage"
)

func (s *Server) ListHostPools(w http.ResponseWriter, r *http.Request) {
	pools, err := s.store.ListHostPools()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if pools == nil {
		pools = []storage.HostPoolRecord{}
	}
	writeJSON(w, http.StatusOK, pools)
}

func (s *Server) CreateHostPool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		CIDRs       []string `json:"cidrs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	id := fmt.Sprintf("hp_%d", time.Now().UnixNano())
	if err := s.store.CreateHostPool(id, req.Name, req.Description, req.CIDRs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "ok"})
}

func (s *Server) UpdateHostPool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		CIDRs       []string `json:"cidrs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := s.store.UpdateHostPool(id, req.Name, req.Description, req.CIDRs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) DeleteHostPool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteHostPool(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) GetHostPoolStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pools, err := s.store.ListHostPools()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var pool *storage.HostPoolRecord
	for _, p := range pools {
		if p.ID == id {
			pool = &p
			break
		}
	}
	if pool == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "pool not found"})
		return
	}

	// Aggregate hosts matching the pool CIDRs
	type poolHost struct {
		IP        string `json:"ip"`
		Hostname  string `json:"hostname"`
		BytesIn   uint64 `json:"bytes_in"`
		BytesOut  uint64 `json:"bytes_out"`
		Country   string `json:"country"`
	}

	gs := s.agg.GlobalSnapshot()
	var hosts []poolHost
	var totalBytes uint64
	cidrNets := parseCIDRs(pool.CIDRs)

	for _, h := range gs.Hosts {
		if matchesAnyCIDR(h.IP, cidrNets) {
			hosts = append(hosts, poolHost{
				IP:       h.IP,
				Hostname: h.Hostname,
				BytesIn:  h.BytesReceived,
				BytesOut: h.BytesSent,
				Country:  h.Country,
			})
			totalBytes += h.BytesSent + h.BytesReceived
		}
	}

	if hosts == nil {
		hosts = []poolHost{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pool":        pool,
		"hosts":       hosts,
		"hosts_count": len(hosts),
		"total_bytes": totalBytes,
	})
}

func parseCIDRs(cidrs []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, c := range cidrs {
		_, nw, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		nets = append(nets, nw)
	}
	return nets
}

func matchesAnyCIDR(ipStr string, nets []*net.IPNet) bool {
	if len(nets) == 0 {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, nw := range nets {
		if nw.Contains(ip) {
			return true
		}
	}
	return false
}
