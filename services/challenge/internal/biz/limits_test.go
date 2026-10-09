package biz

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/errors"
)

func TestValidChallengeID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f", true},
		{"0F9C6B5E-1D2A-4C3B-8E7F-9A0B1C2D3E4F", false},
		{"0f9c6b5e1d2a4c3b8e7f9a0b1c2d3e4f", false},
		{"{0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f}", false},
		{"urn:uuid:0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f", false},
		{"0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4g", false},
		{"", false},
		{strings.Repeat("a", 8<<10), false},
	}
	for _, test := range tests {
		if got := validChallengeID(test.id); got != test.valid {
			t.Errorf("validChallengeID(%.40q) = %v, want %v", test.id, got, test.valid)
		}
	}
}

func TestCheckRefusesMalformedIDBeforeRedis(t *testing.T) {
	f := newFixture(nil)
	_, err := f.usecase.Check(context.Background(), Answer{ChallengeID: strings.Repeat("x", 8<<10), Text: "abcde"})
	if !errors.Is(err, ErrChallengeIDInvalid) || errors.Code(err) != http.StatusBadRequest || f.challenges.takes != 0 {
		t.Fatalf("got %v after %d takes, want CHALLENGE_ID_INVALID 400 before Redis", err, f.challenges.takes)
	}
}

func TestNewCountsTheAddressFirst(t *testing.T) {
	f := newFixture(nil)
	f.limiter.refuse["new_ip"] = true
	_, _, err := f.usecase.New(context.Background(), NewRequest{SiteKey: "pk_test", Client: netip.MustParseAddr("203.0.113.4")})
	if !errors.Is(err, ErrRateLimited) || errors.Code(err) != http.StatusTooManyRequests {
		t.Fatalf("got %v, want RATE_LIMITED 429", err)
	}
	if got := errors.FromError(err).Metadata["retry_after"]; got != "2" {
		t.Fatalf("retry_after = %q, want 2 (1.5 s rounded up)", got)
	}
	// The key was never looked up: fakeSites has none and would have said SITE_KEY_INVALID.
	if len(f.limiter.rules) != 1 || f.limiter.rules[0].Scope != "new_ip" {
		t.Fatalf("rules counted: %+v", f.limiter.rules)
	}
}
