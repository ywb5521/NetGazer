package config

import (
	"crypto/rand"
	"flag"
	"os"
	"encoding/json"
	"time"
)

// SNMPDeviceConfig defines a device for SNMP polling.
type SNMPDeviceConfig struct {
	Name      string `json:"name"`
	Target    string `json:"target"`
	Community string `json:"community"`
	Version   string `json:"version"`
	Port      uint16 `json:"port"`
	// SNMPv3 USM fields
	V3Username       string `json:"v3_username"`
	V3AuthProtocol   string `json:"v3_auth_protocol"`
	V3AuthPassphrase string `json:"v3_auth_passphrase"`
	V3PrivProtocol   string `json:"v3_priv_protocol"`
	V3PrivPassphrase string `json:"v3_priv_passphrase"`
	V3SecurityLevel  string `json:"v3_security_level"`
	// Custom OID polling
	CustomOIDs []CustomOIDConfig `json:"custom_oids"`
}

// CustomOIDConfig defines a custom SNMP OID to poll.
type CustomOIDConfig struct {
	OID  string `json:"oid"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type ServerConfig struct {
	GRPCPort    int
	HTTPPort    int
	DBPath      string
	Retention   time.Duration
	TLSCert     string
	TLSKey      string
	TLSCA       string
	SnapshotInterval time.Duration
	HistoryInterval  time.Duration
	FlowTimeout      time.Duration
	BandwidthThresholdBps float64
	WebhookURL   string
	JWTSecret    []byte
	BPFFilter    string
	NetFlowPort  int
	SFlowPort    int
	SNMPConfig   string
	SNMPDevices  []SNMPDeviceConfig
	DiscoverySubnets   string // comma-separated CIDR subnets (e.g. "192.168.1.0/24,10.0.0.0/16")
	SyslogPort         int
	TrapPort           int
	GeoIPCountryDB     string // path to MaxMind GeoLite2 Country mmdb file
	GeoIPASNDB         string // path to MaxMind GeoLite2 ASN mmdb file
	NodeAuth           bool   // require auth token for agent registration
	WebDir             string // path to frontend dist directory (enables static file serving)
}

func ParseServerFlags() *ServerConfig {
	cfg := &ServerConfig{}
	flag.IntVar(&cfg.GRPCPort, "grpc-port", 50051, "gRPC listen port")
	flag.IntVar(&cfg.HTTPPort, "http-port", 8080, "HTTP/WebSocket listen port")
	flag.StringVar(&cfg.DBPath, "db", "./netgazer.db", "SQLite database path")
	flag.DurationVar(&cfg.Retention, "retention", 24*time.Hour, "Data retention duration")
	flag.StringVar(&cfg.TLSCert, "tls-cert", "", "gRPC TLS certificate path")
	flag.StringVar(&cfg.TLSKey, "tls-key", "", "gRPC TLS key path")
	flag.StringVar(&cfg.TLSCA, "tls-ca", "", "gRPC mTLS CA certificate path (enables mutual TLS when set)")
	flag.Float64Var(&cfg.BandwidthThresholdBps, "bandwidth-threshold", 100_000_000, "Bandwidth alert threshold in bps (default 100 Mbps)")
	flag.IntVar(&cfg.NetFlowPort, "netflow-port", 2055, "NetFlow UDP listen port (0 to disable)")
	flag.IntVar(&cfg.SFlowPort, "sflow-port", 6343, "sFlow UDP listen port (0 to disable)")
	flag.StringVar(&cfg.SNMPConfig, "snmp-config", "", "Path to SNMP devices JSON config file")
	flag.StringVar(&cfg.DiscoverySubnets, "discovery-subnets", "", "Comma-separated CIDR subnets for network discovery (e.g. \"192.168.1.0/24,10.0.0.0/16\")")
	flag.IntVar(&cfg.SyslogPort, "syslog-port", 0, "Syslog UDP listen port (0 to disable)")
	flag.IntVar(&cfg.TrapPort, "trap-port", 0, "SNMP Trap UDP listen port (0 to disable)")
	flag.StringVar(&cfg.GeoIPCountryDB, "geoip-country-db", "", "Path to MaxMind GeoLite2 Country mmdb file")
	flag.StringVar(&cfg.GeoIPASNDB, "geoip-asn-db", "", "Path to MaxMind GeoLite2 ASN mmdb file")
	flag.BoolVar(&cfg.NodeAuth, "node-auth", false, "Require auth token for agent registration")
	flag.StringVar(&cfg.WebDir, "web-dir", "", "Path to frontend dist directory (enables built-in static file serving)")
	flag.StringVar(&cfg.WebhookURL, "webhook-url", "", "Webhook URL for alert notifications")
	if port := os.Getenv("NETGAZER_HTTP_PORT"); port != "" {
		// overridden after parse
	}
	flag.Parse()
	if port := os.Getenv("NETGAZER_HTTP_PORT"); port != "" {
		cfg.HTTPPort = parseInt(port)
	}
	cfg.SnapshotInterval = 1 * time.Second
	cfg.HistoryInterval = 10 * time.Second
	cfg.FlowTimeout = 120 * time.Second

	// Load SNMP device config from JSON file
	if cfg.SNMPConfig != "" {
		if data, err := os.ReadFile(cfg.SNMPConfig); err == nil {
			var devices []SNMPDeviceConfig
			if json.Unmarshal(data, &devices) == nil {
				cfg.SNMPDevices = devices
			}
		}
	}

	// Generate random JWT secret (persisted to DB, loaded on restart)
	cfg.JWTSecret = make([]byte, 32)
	if _, err := rand.Read(cfg.JWTSecret); err != nil {
		cfg.JWTSecret = []byte("netgazer-dev-insecure-secret-key!!")
	}
	return cfg
}
