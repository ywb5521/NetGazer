package geoip

import "strings"

var prefixMap = []struct {
	prefix  string
	country string
}{
	{"10.", "Local (Private)"},
	{"172.16.", "Local (Private)"},
	{"172.17.", "Local (Private)"},
	{"172.18.", "Local (Private)"},
	{"172.19.", "Local (Private)"},
	{"172.20.", "Local (Private)"},
	{"172.21.", "Local (Private)"},
	{"172.22.", "Local (Private)"},
	{"172.23.", "Local (Private)"},
	{"172.24.", "Local (Private)"},
	{"172.25.", "Local (Private)"},
	{"172.26.", "Local (Private)"},
	{"172.27.", "Local (Private)"},
	{"172.28.", "Local (Private)"},
	{"172.29.", "Local (Private)"},
	{"172.30.", "Local (Private)"},
	{"172.31.", "Local (Private)"},
	{"192.168.", "Local (Private)"},
	{"127.", "Localhost"},
	{"169.254.", "Link-Local"},
	{"0.", "Reserved"},
	{"224.", "Multicast"},
	{"239.", "Multicast"},
	{"255.255.255.255", "Broadcast"},
	{"::1", "Localhost"},
	{"fe80:", "Link-Local"},
	{"ff00:", "Multicast"},
	{"ff01:", "Multicast"},
	{"ff02:", "Multicast"},
	{"ff03:", "Multicast"},
	{"ff04:", "Multicast"},
	{"ff05:", "Multicast"},
	{"ff06:", "Multicast"},
	{"ff07:", "Multicast"},
	{"ff08:", "Multicast"},
	{"ff09:", "Multicast"},
	{"ff0a:", "Multicast"},
	{"ff0b:", "Multicast"},
	{"ff0c:", "Multicast"},
	{"ff0d:", "Multicast"},
	{"ff0e:", "Multicast"},
	{"ff0f:", "Multicast"},
	{"1.1.1.", "Cloudflare DNS"},
	{"8.8.8.", "Google DNS"},
	{"8.8.4.", "Google DNS"},
}

func builtinCountryLookup(ip string) string {
	for _, entry := range prefixMap {
		if strings.HasPrefix(ip, entry.prefix) {
			return entry.country
		}
	}
	return ""
}

// Lookup returns the country for an IP using the default engine.
func Lookup(ip string) string {
	return defaultEngine.LookupCountry(ip)
}
