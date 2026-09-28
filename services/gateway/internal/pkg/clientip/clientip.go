// Package clientip finds the address a request really comes from, behind proxies.
// challenge and gateway each keep a copy of this package: keep the copies identical.
package clientip

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

// From returns the address to attribute a request to. X-Forwarded-For is only read when
// the connection itself comes from a trusted proxy, otherwise a client could hand itself
// a fresh identity per request. Among the hops it takes the rightmost untrusted one, the
// last address a trusted proxy observed. The zero value means the address is unknown.
func From(r *http.Request, trusted []netip.Prefix) netip.Addr {
	remote := remoteAddr(r)
	if !contains(trusted, remote) {
		return remote
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			continue
		}
		if !contains(trusted, addr) {
			return addr
		}
	}
	// Every hop is trusted as well: fall back to the first one it named.
	for _, hop := range hops {
		if addr, err := netip.ParseAddr(strings.TrimSpace(hop)); err == nil {
			return addr
		}
	}
	return remote
}

// Key is From as a map key, e.g. for rate limiting; unknown addresses share one key.
func Key(r *http.Request, trusted []netip.Prefix) string {
	return From(r, trusted).String()
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func contains(prefixes []netip.Prefix, addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
