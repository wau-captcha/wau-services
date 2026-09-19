package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted, err := ParsePrefixes([]string{"127.0.0.0/8", "::1/128"})
	if err != nil {
		t.Fatalf("ParsePrefixes: %v", err)
	}

	tests := map[string]struct {
		remoteAddr string
		forwarded  string
		want       string
	}{
		"untrusted caller keeps its address": {"203.0.113.7:1111", "1.2.3.4", "203.0.113.7"},
		"trusted proxy is believed":          {"127.0.0.1:1111", "1.2.3.4", "1.2.3.4"},
		"rightmost untrusted hop wins":       {"127.0.0.1:1111", "1.2.3.4, 127.0.0.1", "1.2.3.4"},
		"all hops trusted":                   {"127.0.0.1:1111", "127.0.0.2, 127.0.0.1", "127.0.0.2"},
		"malformed header":                   {"127.0.0.1:1111", "not-an-ip", "127.0.0.1"},
		"no header":                          {"127.0.0.1:1111", "", "127.0.0.1"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/v1/auth/google", nil)
			request.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				request.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if got := ClientIP(request, trusted); got != tt.want {
				t.Fatalf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePrefixesRejectsGarbage(t *testing.T) {
	if _, err := ParsePrefixes([]string{"nope"}); err == nil {
		t.Fatal("want error for an invalid CIDR")
	}
}
