package biz

import (
	"strings"

	"challenge/internal/pkg/requestinfo"
)

// domainAllowed reports whether a request from origin may use a key that lists domains, as tenant
// stores them: "shop.example.com", "*.example.com" for its subdomains only, "localhost" or
// "localhost:3000", "127.0.0.1". A key that lists none lets any website use it; a request without
// an origin only gets through those. The scheme does not matter, the port only for an entry
// that names one.
func domainAllowed(domains []string, origin string) bool {
	if len(domains) == 0 {
		return true
	}
	host := requestinfo.Host(origin)
	if host == "" {
		return false
	}
	port := requestinfo.Port(origin)
	for _, domain := range domains {
		if domainMatches(domain, host, port) {
			return true
		}
	}
	return false
}

func domainMatches(domain, host, port string) bool {
	if base, ok := strings.CutPrefix(domain, "*."); ok {
		return strings.HasSuffix(host, "."+base)
	}
	if name, wantPort, ok := strings.Cut(domain, ":"); ok {
		return host == name && port == wantPort
	}
	return host == domain
}
