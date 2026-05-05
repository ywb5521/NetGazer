package geoip

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// Engine provides IP-to-country and IP-to-ASN lookups using MaxMind mmdb files.
// Falls back to builtin prefix maps when no database is loaded.
type Engine struct {
	mu        sync.RWMutex
	countryDB *maxminddb.Reader
	asnDB     *maxminddb.Reader
	countryPath string
	asnPath     string
}

// Status holds information about loaded databases.
type Status struct {
	CountryDB   string `json:"country_db"`
	CountryInfo string `json:"country_info"`
	ASNDB       string `json:"asn_db"`
	ASNInfo     string `json:"asn_info"`
	Ready       bool   `json:"ready"`
}

type countryRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

type asnRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

var defaultEngine = &Engine{}

// LookupISOCode returns the two-letter ISO country code for the given IP.
// Uses mmdb if loaded, otherwise falls back to builtin mapping.
func (e *Engine) LookupISOCode(ip string) string {
	e.mu.RLock()
	db := e.countryDB
	e.mu.RUnlock()

	if db != nil {
		var rec countryRecord
		ipAddr := net.ParseIP(ip)
		if ipAddr != nil {
			err := db.Lookup(ipAddr, &rec)
			if err == nil && rec.Country.ISOCode != "" {
				return rec.Country.ISOCode
			}
		}
	}
	return builtinCountryCode(ip)
}

func builtinCountryCode(ip string) string {
	for _, entry := range prefixMapWithCode {
		if strings.HasPrefix(ip, entry.prefix) {
			return entry.code
		}
	}
	return ""
}

var prefixMapWithCode = []struct {
	prefix string
	code   string
}{
	{"10.", "XX"},
	{"172.16.", "XX"},
	{"172.17.", "XX"},
	{"172.18.", "XX"},
	{"172.19.", "XX"},
	{"172.20.", "XX"},
	{"172.21.", "XX"},
	{"172.22.", "XX"},
	{"172.23.", "XX"},
	{"172.24.", "XX"},
	{"172.25.", "XX"},
	{"172.26.", "XX"},
	{"172.27.", "XX"},
	{"172.28.", "XX"},
	{"172.29.", "XX"},
	{"172.30.", "XX"},
	{"172.31.", "XX"},
	{"192.168.", "XX"},
	{"127.", "XX"},
	{"169.254.", "XX"},
	{"0.", "XX"},
	{"224.", "XX"},
	{"239.", "XX"},
	{"255.255.255.255", "XX"},
	{"::1", "XX"},
	{"fe80:", "XX"},
	{"ff00:", "XX"},
}

// Default returns the default global GeoIP engine.
func Default() *Engine { return defaultEngine }

// LoadCountry loads a MaxMind Country mmdb file.
func (e *Engine) LoadCountry(path string) error {
	db, err := maxminddb.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open country db %s: %w", path, err)
	}
	e.mu.Lock()
	if e.countryDB != nil {
		e.countryDB.Close()
	}
	e.countryDB = db
	e.countryPath = path
	e.mu.Unlock()
	log.Printf("[geoip] country database loaded: %s", path)
	return nil
}

// LoadASN loads a MaxMind ASN mmdb file.
func (e *Engine) LoadASN(path string) error {
	db, err := maxminddb.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open ASN db %s: %w", path, err)
	}
	e.mu.Lock()
	if e.asnDB != nil {
		e.asnDB.Close()
	}
	e.asnDB = db
	e.asnPath = path
	e.mu.Unlock()
	log.Printf("[geoip] ASN database loaded: %s", path)
	return nil
}

// UnloadCountry closes the country database.
func (e *Engine) UnloadCountry() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.countryDB != nil {
		e.countryDB.Close()
		e.countryDB = nil
	}
	e.countryPath = ""
}

// UnloadASN closes the ASN database.
func (e *Engine) UnloadASN() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.asnDB != nil {
		e.asnDB.Close()
		e.asnDB = nil
	}
	e.asnPath = ""
}

// Close closes all databases.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.countryDB != nil {
		e.countryDB.Close()
		e.countryDB = nil
	}
	if e.asnDB != nil {
		e.asnDB.Close()
		e.asnDB = nil
	}
}

// Status returns the current engine status.
func (e *Engine) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := Status{}
	if e.countryDB != nil {
		s.CountryDB = e.countryPath
		s.CountryInfo = dbInfo(e.countryDB, "country")
	}
	if e.asnDB != nil {
		s.ASNDB = e.asnPath
		s.ASNInfo = dbInfo(e.asnDB, "ASN")
	}
	s.Ready = e.countryDB != nil || e.asnDB != nil
	return s
}

func dbInfo(db *maxminddb.Reader, kind string) string {
	md := db.Metadata
	return fmt.Sprintf("%s edition: %s, build: %s, languages: %v, node count: %d",
		kind, md.DatabaseType, md.BuildEpoch, md.Languages, int64(md.NodeCount))
}

// LookupCountry returns a human-readable country string for the given IP.
// Uses mmdb if loaded, otherwise falls back to the builtin prefix map.
func (e *Engine) LookupCountry(ip string) string {
	e.mu.RLock()
	db := e.countryDB
	e.mu.RUnlock()

	if db != nil {
		var rec countryRecord
		ipAddr := net.ParseIP(ip)
		if ipAddr != nil {
			err := db.Lookup(ipAddr, &rec)
			if err == nil && rec.Country.ISOCode != "" {
				if name, ok := rec.Country.Names["en"]; ok && name != "" {
					return name
				}
				// Some mmdb files use zh-CN or other languages
				for _, n := range rec.Country.Names {
					if n != "" {
						return n
					}
				}
				return rec.Country.ISOCode
			}
		}
	}
	return builtinCountryLookup(ip)
}

// LookupASNString returns "AS<num> (<org>)" for the given IP.
func (e *Engine) LookupASNString(ip string) string {
	e.mu.RLock()
	db := e.asnDB
	e.mu.RUnlock()

	if db != nil {
		var rec asnRecord
		ipAddr := net.ParseIP(ip)
		if ipAddr != nil {
			err := db.Lookup(ipAddr, &rec)
			if err == nil && rec.AutonomousSystemNumber > 0 {
				return fmt.Sprintf("AS%d (%s)", rec.AutonomousSystemNumber, rec.AutonomousSystemOrganization)
			}
		}
	}
	return LookupASNString(ip)
}

// LookupASN returns structured ASN info for the given IP.
func (e *Engine) LookupASN(ip string) *ASNInfo {
	e.mu.RLock()
	db := e.asnDB
	e.mu.RUnlock()

	if db != nil {
		var rec asnRecord
		ipAddr := net.ParseIP(ip)
		if ipAddr != nil {
			err := db.Lookup(ipAddr, &rec)
			if err == nil && rec.AutonomousSystemNumber > 0 {
				return &ASNInfo{
					ASNumber: uint32(rec.AutonomousSystemNumber),
					ASOrg:    rec.AutonomousSystemOrganization,
				}
			}
		}
	}
	return LookupASN(ip)
}

const GeoipDir = "/var/lib/netgazer/geoip"

// EnsureDir creates the GeoIP data directory if it doesn't exist.
func EnsureDir() error {
	if err := os.MkdirAll(GeoipDir, 0755); err != nil {
		return fmt.Errorf("failed to create geoip dir %s: %w", GeoipDir, err)
	}
	return nil
}
