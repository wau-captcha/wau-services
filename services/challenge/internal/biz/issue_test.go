package biz

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

func boundSite() *Site {
	site := scrapingSite()
	site.AllowedDomains = []string{"shop.example.com", "*.shop.example.com"}
	site.RequireBehavior = true
	return site
}

func TestNewStoresTheRequest(t *testing.T) {
	f := newFixture(boundSite())
	challenge, bound, err := f.usecase.New(context.Background(), NewRequest{
		SiteKey: "pk_shop", Client: visitor, Origin: "https://m.shop.example.com", UserAgent: chromeUA,
	})
	if err != nil || !bound {
		t.Fatalf("got %v, bound %v", err, bound)
	}
	stored := f.challenges.stored[challenge.ID]
	switch {
	case stored == nil:
		t.Fatal("not saved")
	case stored.SiteKey != "pk_shop" || stored.Origin != "https://m.shop.example.com" || stored.ClientIP != "203.0.113.4":
		t.Fatalf("request not stored: %+v", stored)
	case stored.UserAgentHash != hashUserAgent(chromeUA) || len(stored.UserAgentHash) != 64:
		t.Fatalf("user agent hash %q", stored.UserAgentHash)
	case !stored.RequireBehavior || stored.PowDifficulty != 8 || len(stored.PowSalt) != 16:
		t.Fatalf("require_behavior %v, pow %d bits, %d-byte salt", stored.RequireBehavior, stored.PowDifficulty, len(stored.PowSalt))
	case !validChallengeID(stored.ID):
		t.Fatalf("id %q", stored.ID)
	}
}

func TestNewDomainBinding(t *testing.T) {
	open := scrapingSite()
	tests := []struct {
		name   string
		site   *Site
		origin string
		bound  bool
		err    *errors.Error
	}{
		{"listed domain", boundSite(), "https://shop.example.com", true, nil},
		{"another website", boundSite(), "https://evil.test", true, ErrDomainNotAllowed},
		{"no website", boundSite(), "", true, ErrDomainNotAllowed},
		{"key without domains, any website", open, "https://evil.test", false, nil},
		{"key without domains, no website", open, "", false, nil},
	}
	for _, test := range tests {
		f := newFixture(test.site)
		_, bound, err := f.usecase.New(context.Background(), NewRequest{SiteKey: "pk_shop", Client: visitor, Origin: test.origin, UserAgent: chromeUA})
		if bound != test.bound || (test.err == nil && err != nil) || (test.err != nil && !errors.Is(err, test.err)) {
			t.Errorf("%s: got %v, bound %v; want %v, bound %v", test.name, err, bound, test.err, test.bound)
		}
	}
	// The IP list keeps its own reason.
	site := boundSite()
	site.AllowedOrigins = []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")}
	f := newFixture(site)
	if _, _, err := f.usecase.New(context.Background(), NewRequest{SiteKey: "pk_shop", Client: visitor, Origin: "https://shop.example.com"}); !errors.Is(err, ErrOriginNotAllowed) {
		t.Errorf("address outside the IP list: %v, want ORIGIN_NOT_ALLOWED", err)
	}
}

func TestNewThrottlesFlaggedKey(t *testing.T) {
	tests := []struct {
		name      string
		flaggedAt func(now time.Time) time.Time
		limit     int64
	}{
		{"never flagged", func(time.Time) time.Time { return time.Time{} }, 600},
		{"flagged today", func(now time.Time) time.Time { return now.Add(-time.Hour) }, 60},
		{"flagged yesterday", func(now time.Time) time.Time { return now.Add(-24 * time.Hour) }, 600},
	}
	for _, test := range tests {
		site := scrapingSite()
		f := newFixture(site)
		site.FlaggedAt = test.flaggedAt(f.now)
		if _, _, err := f.usecase.New(context.Background(), NewRequest{SiteKey: "pk_shop", Client: visitor}); err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		rules := f.limiter.rules
		if len(rules) != 2 || rules[0].Scope != "new_ip" || rules[1].Scope != "new_key" || rules[1].Limit != test.limit {
			t.Errorf("%s: rules %+v, want new_ip then new_key at %d", test.name, rules, test.limit)
		}
	}
}
