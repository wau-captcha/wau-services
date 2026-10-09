package requestinfo

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrigin(t *testing.T) {
	tests := []struct {
		origin, referer, want string
	}{
		{"https://Shop.Example.com", "", "https://shop.example.com"},
		{"https://shop.example.com:443", "", "https://shop.example.com"},
		{"http://localhost:3000", "", "http://localhost:3000"},
		{"http://127.0.0.1:80", "", "http://127.0.0.1"},
		{"", "https://shop.example.com/signup?step=2", "https://shop.example.com"},
		{"null", "https://shop.example.com/a", "https://shop.example.com"},
		{"null", "", ""},
		{"file://", "", ""},
		{"chrome-extension://abcdef", "", ""},
		{"", "not a url", ""},
		{"https://[::1]:8443", "", "https://[::1]:8443"},
		{"https://" + strings.Repeat("a", 254), "", ""},
		{"https://shop.example.com:" + strings.Repeat("1", 6), "", ""},
		{"https://" + strings.Repeat("a", 253) + ":8443", "", "https://" + strings.Repeat("a", 253) + ":8443"},
	}
	for _, test := range tests {
		request := httptest.NewRequest("GET", "/challenge/new", nil)
		if test.origin != "" {
			request.Header.Set("Origin", test.origin)
		}
		if test.referer != "" {
			request.Header.Set("Referer", test.referer)
		}
		if got := Origin(request); got != test.want {
			t.Errorf("Origin(%q, %q) = %q, want %q", test.origin, test.referer, got, test.want)
		}
	}
	if Host("https://shop.example.com:8443") != "shop.example.com" || Port("https://shop.example.com:8443") != "8443" || Host("") != "" {
		t.Error("Host or Port")
	}
}
