package biz

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/errors"
)

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		raw, want string
	}{
		{"shop.example.com", "shop.example.com"},
		{"  Shop.Example.COM  ", "shop.example.com"},
		{"shop.example.com.", "shop.example.com"},
		{"*.example.com", "*.example.com"},
		{"*.Example.com", "*.example.com"},
		{"bücher.de", "xn--bcher-kva.de"},
		{"*.bücher.de", "*.xn--bcher-kva.de"},
		{"xn--bcher-kva.de", "xn--bcher-kva.de"},
		{"cửahàng.vn", "xn--cahng-tqa2694c.vn"},
		{"localhost", "localhost"},
		{"LOCALHOST:3000", "localhost:3000"},
		{"127.0.0.1", "127.0.0.1"},
		{"a1.example.com", "a1.example.com"},
		{strings.Repeat("a", 63) + ".com", strings.Repeat("a", 63) + ".com"},
		// Refused.
		{"", ""},
		{"https://shop.example.com", ""},
		{"shop.example.com/signup", ""},
		{"shop.example.com?x=1", ""},
		{"user@shop.example.com", ""},
		{"shop example.com", ""},
		{"shop.example.com:443", ""},
		{"localhost:0", ""},
		{"localhost:65536", ""},
		{"localhost:30a0", ""},
		{"*.localhost", ""},
		{"*.com", ""},
		{"*example.com", ""},
		{"a.*.example.com", ""},
		{"*.*.example.com", ""},
		{"com", ""},
		{"intranet", ""},
		{"10.0.0.1", ""},
		{"*.127.0.0.1", ""},
		{"127.0.0.1:3000", ""},
		{"[::1]", ""},
		{"::1", ""},
		{"1.2.3.4.5", ""},
		{"shop..example.com", ""},
		{".example.com", ""},
		{"-shop.example.com", ""},
		{"shop_1.example.com", ""},
		{strings.Repeat("a", 64) + ".com", ""},
		{strings.Repeat("a.", 126) + "com", ""},
	}
	for _, test := range tests {
		got, err := NormalizeDomain(test.raw)
		switch {
		case test.want == "" && err == nil:
			t.Errorf("NormalizeDomain(%q) = %q, want an error", test.raw, got)
		case test.want != "" && (err != nil || got != test.want):
			t.Errorf("NormalizeDomain(%q) = %q, %v; want %q", test.raw, got, err, test.want)
		case err != nil && (!errors.Is(err, ErrInvalidArgument) || errors.FromError(err).Metadata["field"] != "allowed_domains"):
			t.Errorf("NormalizeDomain(%q): %v is not INVALID_ARGUMENT on allowed_domains", test.raw, err)
		}
	}
}

func TestNormalizeDomains(t *testing.T) {
	if domains, err := NormalizeDomains(nil); err != nil || len(domains) != 0 {
		t.Fatalf("empty list: %v, %v", domains, err)
	}
	if _, err := NormalizeDomains([]string{"Shop.example.com", "shop.example.com"}); err == nil {
		t.Fatal("the same domain twice was accepted")
	}
	fifty := make([]string, 50)
	for i := range fifty {
		fifty[i] = fmt.Sprintf("site%d.example.com", i)
	}
	if _, err := NormalizeDomains(fifty); err != nil {
		t.Fatalf("50 domains: %v", err)
	}
	if _, err := NormalizeDomains(append(fifty, "one-more.example.com")); err == nil {
		t.Fatal("51 domains were accepted")
	}
}
