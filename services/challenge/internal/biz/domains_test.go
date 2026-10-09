package biz

import "testing"

func TestDomainAllowed(t *testing.T) {
	shop := []string{"shop.example.com", "*.example.org", "localhost:3000", "127.0.0.1"}
	tests := []struct {
		domains []string
		origin  string
		allowed bool
	}{
		{nil, "", true},
		{nil, "https://anything.test", true},
		{shop, "https://shop.example.com", true},
		{shop, "http://shop.example.com", true},
		{shop, "https://shop.example.com:8443", true},
		{shop, "https://www.shop.example.com", false},
		{shop, "https://example.com", false},
		{shop, "https://a.example.org", true},
		{shop, "https://a.b.example.org", true},
		{shop, "https://example.org", false},
		{shop, "https://badexample.org", false},
		{shop, "https://example.org.evil.test", false},
		{shop, "http://localhost:3000", true},
		{shop, "http://localhost:3001", false},
		{shop, "http://localhost", false},
		{shop, "http://127.0.0.1:8080", true},
		{shop, "", false},
	}
	for _, test := range tests {
		if got := domainAllowed(test.domains, test.origin); got != test.allowed {
			t.Errorf("domainAllowed(%q, %q) = %v, want %v", test.domains, test.origin, got, test.allowed)
		}
	}
}
