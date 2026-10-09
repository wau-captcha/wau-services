// Package requestinfo reads what a browser says about the page behind a request. challenge and
// verify each keep a copy of this package: keep the copies identical, so both services read the
// same origin out of the same request.
package requestinfo

import (
	"net/http"
	"net/url"
	"strings"
)

// maxHostLength is the longest DNS name; a longer host cannot be a website's, nor a port of more
// than 5 digits.
const maxHostLength = 253

// Origin is the website a request comes from, as scheme://host[:port], lowercase and without a
// default port: the Origin header, or else the origin of Referer. Empty when neither names one,
// e.g. a call from a server, or the "null" origin of a sandboxed frame.
func Origin(r *http.Request) string {
	if origin := Normalize(r.Header.Get("Origin")); origin != "" {
		return origin
	}
	return Normalize(r.Header.Get("Referer"))
}

// Normalize reduces an origin or a URL to its origin, or returns "" when it has none.
func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if len(host) > maxHostLength {
		return ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	port := parsed.Port()
	if len(port) > 5 {
		return ""
	}
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	return parsed.Scheme + "://" + host
}

// Host is the host name of an origin Normalize returned, without its port; "" for "".
func Host(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// Port is the port an origin Normalize returned names, "" for the default one.
func Port(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	return parsed.Port()
}
