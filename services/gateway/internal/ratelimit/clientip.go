// Package ratelimit caps how often one client may hit sensitive endpoints.
package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ParsePrefixes reads the CIDR blocks whose forwarded headers may be believed.
func ParsePrefixes(cidrs []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(cidrs))
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q: %w", cidr, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

// ClientIP returns the address to count requests against. X-Forwarded-For is
// only read when the connection itself comes from a trusted proxy, otherwise a
// client could hand itself a fresh identity per request. Among the hops it takes
// the rightmost untrusted one, the last address a trusted proxy observed.
func ClientIP(r *http.Request, trusted []netip.Prefix) string {
	remote := remoteAddr(r)
	if !isTrusted(remote, trusted) {
		return remote
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			continue
		}
		if !isTrusted(addr.String(), trusted) {
			return addr.String()
		}
	}
	// Every hop is trusted as well: fall back to the first one it named.
	for _, hop := range hops {
		if addr, err := netip.ParseAddr(strings.TrimSpace(hop)); err == nil {
			return addr.String()
		}
	}
	return remote
}

func remoteAddr(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	return addr.String()
}

func isTrusted(ip string, trusted []netip.Prefix) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
