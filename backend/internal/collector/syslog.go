package collector

import (
	"context"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// SyslogMessage represents a parsed syslog message.
type SyslogMessage struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Facility  string    `json:"facility"`
	Severity  string    `json:"severity"`
	Hostname  string    `json:"hostname"`
	AppName   string    `json:"app_name"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Raw       string    `json:"raw"`
}

// SyslogCallback is called with each parsed syslog message.
type SyslogCallback func(msg SyslogMessage)

// SyslogCollector listens for syslog messages via UDP.
type SyslogCollector struct {
	port     int
	callback SyslogCallback
	conn     *net.UDPConn
}

// NewSyslogCollector creates a new syslog collector.
func NewSyslogCollector(port int, cb SyslogCallback) *SyslogCollector {
	return &SyslogCollector{port: port, callback: cb}
}

// Start begins listening for syslog messages.
func (s *SyslogCollector) Start(ctx context.Context) error {
	addr := &net.UDPAddr{Port: s.port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	s.conn = conn
	log.Printf("[syslog] listening on UDP :%d", s.port)

	buf := make([]byte, 65535)
	go func() {
		defer conn.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				continue
			}
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if !strings.Contains(err.Error(), "use of closed") {
					log.Printf("[syslog] read error: %v", err)
				}
				continue
			}
			msg := parseSyslog(string(buf[:n]), remote.IP.String())
			if s.callback != nil {
				s.callback(msg)
			}
		}
	}()
	return nil
}

var severityNames = []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}
var facilityNames = []string{"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
	"uucp", "cron", "authpriv", "ftp", "ntp", "audit", "alert", "clock",
	"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7"}

func parseSyslog(raw, source string) SyslogMessage {
	msg := SyslogMessage{
		Timestamp: time.Now(),
		Severity:  "info",
		Facility:  "daemon",
		Source:    source,
		Raw:       raw,
		ID:        formatSyslogID(source, time.Now()),
	}

	// RFC 3164: <PRI>TIMESTAMP HOSTNAME MSG
	// RFC 5424: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
	if strings.HasPrefix(raw, "<") {
		end := strings.IndexByte(raw, '>')
		if end > 0 && end < 10 {
			pri := 0
			for _, c := range raw[1:end] {
				if c >= '0' && c <= '9' {
					pri = pri*10 + int(c-'0')
				}
			}
			fac := pri / 8
			sev := pri % 8
			if sev >= 0 && sev < len(severityNames) {
				msg.Severity = severityNames[sev]
			}
			if fac >= 0 && fac < len(facilityNames) {
				msg.Facility = facilityNames[fac]
			}
			rest := strings.TrimSpace(raw[end+1:])

			// Try to parse structured RFC 5424 format (version number present)
			if len(rest) > 2 && rest[0] == '1' && rest[1] == ' ' {
				parts := strings.SplitN(rest[2:], " ", 5)
				if len(parts) >= 1 {
					msg.Hostname = parts[0]
				}
				if len(parts) >= 2 {
					msg.AppName = parts[1]
				}
				if len(parts) >= 5 {
					msg.Message = parts[4]
				}
			} else {
				// RFC 3164 format
				parts := strings.SplitN(rest, " ", 3)
				if len(parts) >= 1 {
					msg.Hostname = parts[0]
				}
				if len(parts) >= 3 {
					msg.Message = parts[2]
				} else {
					msg.Message = rest
				}
			}
		}
	}

	if msg.Message == "" {
		msg.Message = raw
	}

	return msg
}

var syslogMu sync.Mutex
var syslogSeq int64

func formatSyslogID(source string, ts time.Time) string {
	syslogMu.Lock()
	syslogSeq++
	n := syslogSeq
	syslogMu.Unlock()
	return "syslog-" + source + "-" + ts.Format("20060102-150405") + "-" + itoa(int(n))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte(n%10) + '0'}, buf...)
		n /= 10
	}
	return string(buf)
}
