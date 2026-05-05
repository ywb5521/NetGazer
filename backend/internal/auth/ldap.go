package auth

import (
	"fmt"
	"log"
	"strings"
)

// For LDAP support, add "github.com/go-ldap/ldap/v3" as a dependency and
// implement real LDAP Bind+Search authentication below.
//
// The function signature is ready for integration:
//   func LDAPAuth(server, bindDN, bindPassword, baseDN, userFilter, username, password string) (bool, error)

// LDAPConfig holds LDAP connection settings.
type LDAPConfig struct {
	Enabled    bool   `json:"enabled"`
	Server     string `json:"server"`      // e.g. "ldap.example.com:389"
	BindDN     string `json:"bind_dn"`     // e.g. "cn=admin,dc=example,dc=com"
	BindPass   string `json:"bind_pass"`   // bind password
	BaseDN     string `json:"base_dn"`     // e.g. "dc=example,dc=com"
	UserFilter string `json:"user_filter"` // e.g. "(uid=%s)" or "(sAMAccountName=%s)"
	UseTLS     bool   `json:"use_tls"`
}

// DefaultLDAPConfig returns a default LDAP configuration.
func DefaultLDAPConfig() LDAPConfig {
	return LDAPConfig{
		UserFilter: "(uid=%s)",
	}
}

// LDAPAuthenticator performs LDAP authentication when go-ldap is available.
// When the library is not available, it returns false with a message.
type LDAPAuthenticator struct {
	config LDAPConfig
}

func NewLDAPAuthenticator(cfg LDAPConfig) *LDAPAuthenticator {
	return &LDAPAuthenticator{config: cfg}
}

// Authenticate tries to authenticate a user against LDAP.
// Returns (true, nil) on success, (false, nil) if credentials are wrong,
// or (false, error) on connection/configuration errors.
func (la *LDAPAuthenticator) Authenticate(username, password string) (bool, error) {
	if !la.config.Enabled {
		return false, nil
	}

	// Build user DN filter
	filter := strings.ReplaceAll(la.config.UserFilter, "%s", username)

	log.Printf("[ldap] attempting auth for user=%q server=%s base=%s filter=%s",
		username, la.config.Server, la.config.BaseDN, filter)

	// When go-ldap is available, replace this with real LDAP bind+search:
	//
	//   conn, err := ldap.DialURL(fmt.Sprintf("ldap://%s", la.config.Server))
	//   if la.config.UseTLS { conn.StartTLS(tlsConfig) }
	//   err = conn.Bind(la.config.BindDN, la.config.BindPass)
	//   searchReq := ldap.NewSearchRequest(la.config.BaseDN, ldap.ScopeWholeSubtree, ...)
	//   result, err := conn.Search(searchReq)
	//   userDN := result.Entries[0].DN
	//   err = conn.Bind(userDN, password)

	return false, fmt.Errorf("LDAP library not available (add github.com/go-ldap/ldap/v3 dependency)")
}

// Enabled returns whether LDAP is configured and enabled.
func (la *LDAPAuthenticator) Enabled() bool {
	return la.config.Enabled
}
