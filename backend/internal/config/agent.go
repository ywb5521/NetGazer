package config

import (
	"flag"
	"os"
	"strings"
)

type AgentConfig struct {
	ServerAddr  string
	Interface   string   // deprecated, kept for backward compat
	Interfaces  []string // parsed from --interfaces or fallback to --interface
	NodeID      string
	BPFFilter   string
	Tags        string
	TLSCert     string
	TLSKey      string
	TLSCA       string
	ProtoEngine string // "ndpi", "opengfw", "both"
	Intercept   bool
	InterceptLocal bool
	InterceptRST   bool
	AuthToken    string // shared secret for node authentication
}

func ParseAgentFlags() *AgentConfig {
	cfg := &AgentConfig{}
	hostname, _ := os.Hostname()
	flag.StringVar(&cfg.ServerAddr, "server-addr", "localhost:50051", "gtopng-server gRPC address")
	flag.StringVar(&cfg.Interface, "interface", "", "Network interface to capture (deprecated: use --interfaces)")
	var ifaces string
	flag.StringVar(&ifaces, "interfaces", "eth0", "Network interfaces to capture (comma-separated)")
	flag.StringVar(&cfg.NodeID, "node-id", hostname, "Agent node unique identifier")
	flag.StringVar(&cfg.BPFFilter, "bpf-filter", "", "BPF capture filter")
	flag.StringVar(&cfg.Tags, "tags", "", "Node tags (comma-separated)")
	flag.StringVar(&cfg.TLSCert, "tls-cert", "", "mTLS certificate path")
	flag.StringVar(&cfg.TLSKey, "tls-key", "", "mTLS key path")
	flag.StringVar(&cfg.TLSCA, "tls-ca", "", "mTLS CA certificate path (enables mutual TLS when set)")
	flag.StringVar(&cfg.ProtoEngine, "proto-engine", "ndpi", "Protocol detection engine: ndpi, opengfw, both")
	flag.BoolVar(&cfg.Intercept, "intercept", false, "Enable traffic interception (requires root)")
	flag.BoolVar(&cfg.InterceptLocal, "intercept-local", false, "Local mode for interception (INPUT/OUTPUT chains)")
	flag.BoolVar(&cfg.InterceptRST, "intercept-rst", false, "Send TCP RST for blocked connections")
	flag.StringVar(&cfg.AuthToken, "auth-token", "", "Shared secret token for node authentication to server")
	flag.Parse()

	// Build Interfaces list: prefer --interfaces, fallback to --interface
	if ifaces != "" {
		for _, s := range strings.Split(ifaces, ",") {
			if t := strings.TrimSpace(s); t != "" {
				cfg.Interfaces = append(cfg.Interfaces, t)
			}
		}
	}
	if len(cfg.Interfaces) == 0 && cfg.Interface != "" {
		cfg.Interfaces = []string{cfg.Interface}
	}
	if len(cfg.Interfaces) == 0 {
		cfg.Interfaces = []string{"eth0"}
	}

	return cfg
}
