package biz

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/idna"
)

const (
	// A key may name this many websites.
	maxDomains = 50
	// The longest DNS name.
	maxDomainLength = 253
)

// AllowedDomain is one website allowed to use a site key.
type AllowedDomain struct {
	// As NormalizeDomain returns it, e.g. "shop.example.com" or "*.example.com".
	Domain     string
	CreateDate time.Time
}

// NormalizeDomains checks the websites a key accepts and puts them in the form the challenge
// service compares origins with. An empty list is allowed and means any website may use the key.
func NormalizeDomains(rawDomains []string) ([]string, error) {
	if len(rawDomains) > maxDomains {
		return nil, InvalidField("allowed_domains", "at most 50 domains")
	}
	domains := make([]string, 0, len(rawDomains))
	for _, raw := range rawDomains {
		domain, err := NormalizeDomain(raw)
		if err != nil {
			return nil, err
		}
		if slices.Contains(domains, domain) {
			return nil, InvalidField("allowed_domains", "each domain may be listed only once")
		}
		domains = append(domains, domain)
	}
	return domains, nil
}

// NormalizeDomain reads a website as a host name: lowercase, international names in punycode,
// "*." in front for its subdomains (not the domain itself), a port for localhost only. Schemes,
// paths and IP addresses other than 127.0.0.1 are refused: an origin is matched by its host.
func NormalizeDomain(raw string) (string, error) {
	invalid := InvalidField("allowed_domains",
		"a domain is a host name such as shop.example.com or *.example.com, without scheme or path; localhost may name a port")
	value := strings.TrimSpace(raw)
	if value == "" || strings.ContainsAny(value, "/\\?#@ \t") {
		return "", invalid
	}
	wildcard := false
	if rest, ok := strings.CutPrefix(value, "*."); ok {
		wildcard, value = true, rest
	}
	host, port, hasPort := strings.Cut(value, ":")
	host = strings.TrimSuffix(host, ".")
	if strings.Contains(host, "*") {
		return "", invalid
	}
	if hasPort {
		if wildcard || !strings.EqualFold(host, "localhost") || !validPort(port) {
			return "", invalid
		}
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if wildcard || address != netip.AddrFrom4([4]byte{127, 0, 0, 1}) {
			return "", InvalidField("allowed_domains", "IP addresses are not domains, except 127.0.0.1; limit addresses in the IP list")
		}
		return host, nil
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil || len(ascii) > maxDomainLength || !validLabels(ascii) {
		return "", invalid
	}
	// A name of one label is a top-level domain or a machine on a private network.
	if (ascii != "localhost" || wildcard) && !strings.Contains(ascii, ".") {
		return "", invalid
	}
	switch {
	case wildcard:
		return "*." + ascii, nil
	case hasPort:
		return ascii + ":" + port, nil
	}
	return ascii, nil
}

// validLabels reports whether every label of a host name has 1 to 63 characters and the last one
// is not a number, which browsers would read as an IPv4 address.
func validLabels(host string) bool {
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
	}
	return strings.Trim(labels[len(labels)-1], "0123456789") != ""
}

func validPort(port string) bool {
	if port == "" || len(port) > 5 || strings.Trim(port, "0123456789") != "" {
		return false
	}
	number, err := strconv.Atoi(port)
	return err == nil && number >= 1 && number <= 65535
}
