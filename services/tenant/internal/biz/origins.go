package biz

import (
	"net/netip"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// A key may name this many addresses, each with a description of this length.
	maxOrigins           = 50
	maxOriginDescription = 100
)

// AllowedOrigin is one address, or network, allowed to use a site key.
type AllowedOrigin struct {
	// Canonical network, e.g. "203.0.113.4/32", "203.0.113.0/24" or "2001:db8::/32".
	IPAddress string
	// What the address is, chosen by the user; may be empty.
	Description string
	CreateDate  time.Time
}

// NormalizeOrigins checks the addresses a key accepts and puts them in the form they are
// stored and shown in: a network, with a plain address covering only itself. An empty list
// is allowed and means every address may use the key.
func NormalizeOrigins(rawOrigins []AllowedOrigin) ([]AllowedOrigin, error) {
	if len(rawOrigins) > maxOrigins {
		return nil, InvalidField("ip_address", "at most 50 addresses")
	}
	origins := make([]AllowedOrigin, 0, len(rawOrigins))
	for _, entry := range rawOrigins {
		address, err := NormalizeAddress(entry.IPAddress)
		if err != nil {
			return nil, err
		}
		description := strings.TrimSpace(entry.Description)
		if utf8.RuneCountInString(description) > maxOriginDescription {
			return nil, InvalidField("description", "a description may be at most 100 characters")
		}
		if slices.ContainsFunc(origins, func(origin AllowedOrigin) bool { return origin.IPAddress == address }) {
			return nil, InvalidField("ip_address", "each address may be listed only once")
		}
		origins = append(origins, AllowedOrigin{IPAddress: address, Description: description})
	}
	return origins, nil
}

// NormalizeAddress reads an IPv4 or IPv6 address, with an optional prefix length, and
// returns its network: "203.0.113.4" becomes "203.0.113.4/32" and "203.0.113.4/24" the
// network it lies in, since only the network decides who gets through.
func NormalizeAddress(rawAddress string) (string, error) {
	value := strings.TrimSpace(rawAddress)
	invalid := InvalidField("ip_address", "an address is an IPv4 or IPv6 address with an optional prefix length, e.g. 203.0.113.4 or 203.0.113.0/24")
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), nil
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return "", invalid
	}
	address = address.Unmap()
	return netip.PrefixFrom(address, address.BitLen()).String(), nil
}

// addresses lists the networks of origins, for the activity log.
func addresses(origins []AllowedOrigin) []string {
	networks := make([]string, len(origins))
	for i, origin := range origins {
		networks[i] = origin.IPAddress
	}
	return networks
}
