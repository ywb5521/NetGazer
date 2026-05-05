package api

import (
	"net/http"
)

func (s *Server) GetCountryStats(w http.ResponseWriter, r *http.Request) {
	stats := s.agg.CountryStats()
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) GetASNStats(w http.ResponseWriter, r *http.Request) {
	stats := s.agg.ASStats()
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) GetServiceMap(w http.ResponseWriter, r *http.Request) {
	services, edges := s.agg.ServiceMap()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"services": services,
		"edges":    edges,
	})
}

func (s *Server) GetAllInterfaces(w http.ResponseWriter, r *http.Request) {
	nodes := s.agg.GetNodeStates()
	type ifaceSummary struct {
		NodeID       string  `json:"node_id"`
		Name         string  `json:"name"`
		BytesPerSec  float64 `json:"bytes_per_sec"`
		PktsPerSec   float64 `json:"packets_per_sec"`
		HostsCount   int     `json:"hosts_count"`
		FlowsCount   int     `json:"flows_count"`
	}
	result := make([]ifaceSummary, 0)
	for _, n := range nodes {
		for _, ii := range n.InterfaceInfo {
			result = append(result, ifaceSummary{
				NodeID:      n.NodeID,
				Name:        ii.Name,
				BytesPerSec: ii.BytesPerSec,
				PktsPerSec:  ii.PacketsPerSec,
				HostsCount:  ii.HostsCount,
				FlowsCount:  ii.FlowsCount,
			})
		}
	}
	writeJSON(w, http.StatusOK, result)
}
