package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gtopng/backend/internal/auth"
)

func NewRouter(s *Server) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5, "application/json", "text/csv", "application/octet-stream"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Unauthenticated routes
	r.Get("/api/auth/status", s.CheckSetup)
	r.Post("/api/auth/setup", s.Setup)
	r.Post("/api/auth/login", s.Login)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTAuth(s.cfg.JWTSecret))

		r.Get("/api/summary", s.GetSummary)
		r.Get("/api/nodes", s.GetNodes)
		r.Get("/api/hosts", s.GetHosts)
		r.Get("/api/hosts/{ip}", s.GetHost)
		r.Get("/api/hosts/{ip}/protocols", s.GetHostProtocols)
		r.Get("/api/hosts/{ip}/peers", s.GetHostPeers)
		r.Get("/api/hosts/{ip}/traffic", s.GetHostTrafficHistory)
		r.Get("/api/flows", s.GetFlows)
		r.Get("/api/protocols", s.GetProtocols)
		r.Get("/api/alerts", s.GetAlerts)
		r.Post("/api/alerts/{id}/ack", s.AcknowledgeAlert)
		r.Get("/api/traffic/history", s.GetTrafficHistory)
		r.Get("/api/traffic-matrix", s.GetTrafficMatrix)
		r.Get("/api/config", s.GetConfig)
		r.Put("/api/config", s.UpdateConfig)
		r.Get("/api/reports/summary", s.GetReportSummary)
		r.Get("/api/reports/top-talkers", s.GetReportTopTalkers)
		r.Get("/api/reports/top-protocols", s.GetReportTopProtocols)
		r.Get("/api/reports/alerts", s.GetReportAlerts)
		r.Get("/api/reports/trend", s.GetReportTrend)
		r.Get("/api/export/snapshots", s.ExportSnapshots)
		r.Get("/api/export/hosts", s.ExportHosts)
		r.Get("/api/export/alerts", s.ExportAlerts)
		r.Get("/api/notification-channels", s.ListChannels)
		r.Post("/api/notification-channels", s.CreateChannel)
		r.Put("/api/notification-channels/{id}", s.UpdateChannel)
		r.Delete("/api/notification-channels/{id}", s.DeleteChannel)
		r.Post("/api/notification-channels/{id}/test", s.TestChannel)
		r.Get("/api/lua-scripts", s.ListLuaScripts)
		r.Post("/api/lua-scripts", s.CreateLuaScript)
		r.Delete("/api/lua-scripts/{name}", s.DeleteLuaScript)
		r.Post("/api/lua-scripts/test", s.TestLuaScript)
		r.Get("/api/voip-sessions", s.GetVoipSessions)
	r.Get("/api/intercept/rules", s.ListInterceptRules)
	r.Post("/api/intercept/rules", s.CreateInterceptRule)
	r.Put("/api/intercept/rules/{id}", s.UpdateInterceptRule)
	r.Delete("/api/intercept/rules/{id}", s.DeleteInterceptRule)
	r.Post("/api/intercept/apply", s.ApplyInterceptRules)
	r.Get("/api/intercept/node-rules/{node}", s.GetInterceptNodeRules)
		r.Get("/api/syslog", s.GetSyslog)
		r.Get("/api/traps", s.GetTraps)
		r.Get("/api/traffic-matrix/history", s.GetMatrixHistory)
		r.Get("/api/geoip/status", s.GetGeoIPStatus)
		r.Post("/api/geoip/upload", s.UploadGeoIPDB)
		r.Post("/api/geoip/download", s.DownloadGeoIPDB)
		r.Get("/api/node-tokens", s.ListNodeTokens)
		r.Post("/api/node-tokens", s.CreateNodeToken)
		r.Delete("/api/node-tokens/{id}", s.DeleteNodeToken)
		r.Get("/api/geo/countries", s.GetCountryStats)
		r.Get("/api/geo/asns", s.GetASNStats)
		r.Get("/api/service-map", s.GetServiceMap)
		r.Get("/api/interfaces", s.GetAllInterfaces)
		r.Get("/api/host-pools", s.ListHostPools)
		r.Post("/api/host-pools", s.CreateHostPool)
		r.Put("/api/host-pools/{id}", s.UpdateHostPool)
		r.Delete("/api/host-pools/{id}", s.DeleteHostPool)
		r.Get("/api/host-pools/{id}/stats", s.GetHostPoolStats)
	})

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		s.Hub.HandleWebSocket(w, r)
	})

	return r
}
