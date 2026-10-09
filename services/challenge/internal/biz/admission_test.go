package biz

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

func TestScrapingJudge(t *testing.T) {
	rules := newFixture(nil).usecase.scraping
	tests := []struct {
		name   string
		counts DailyCounts
		want   scrapingVerdict
	}{
		{"quiet day", DailyCounts{Issued: 499}, scrapingNone},
		{"prefetching app under the flag floor", DailyCounts{Issued: 499, Answered: 10}, scrapingNone},
		{"under 10% answered", DailyCounts{Issued: 500, Answered: 49}, scrapingFlag},
		{"10% answered", DailyCounts{Issued: 500, Answered: 50, Solved: 40}, scrapingNone},
		{"under 10% solved", DailyCounts{Issued: 1000, Answered: 200, Solved: 19}, scrapingFlag},
		{"10% solved", DailyCounts{Issued: 1000, Answered: 200, Solved: 20}, scrapingNone},
		{"over half automated", DailyCounts{Issued: 1000, Answered: 200, Solved: 150, Automated: 101}, scrapingFlag},
		{"half automated", DailyCounts{Issued: 1000, Answered: 200, Solved: 150, Automated: 100}, scrapingNone},
		{"too few answers to judge", DailyCounts{Issued: 1000, Answered: 199, Automated: 199}, scrapingNone},
		{"thousands, under 5% answered", DailyCounts{Issued: 5000, Answered: 249}, scrapingSuspend},
		{"thousands, 5% answered", DailyCounts{Issued: 5000, Answered: 250, Solved: 200}, scrapingFlag},
		{"just under the suspension floor", DailyCounts{Issued: 4999}, scrapingFlag},
	}
	for _, test := range tests {
		if got := rules.judge(test.counts); got != test.want {
			t.Errorf("%s: judge(%+v) = %d, want %d", test.name, test.counts, got, test.want)
		}
	}
	if got := (scrapingRules{}).judge(DailyCounts{Issued: 1 << 20}); got != scrapingNone {
		t.Errorf("rules turned off: judge = %d", got)
	}
}

func scrapingSite() *Site {
	return &Site{APIKeyID: "key-1", UserID: "user-1", ChallengeTypes: []string{TypeImageDistort}, DailyQuota: 5000}
}

func TestAdmitChallengeFlagsOncePerDay(t *testing.T) {
	f := newFixture(scrapingSite())
	f.counters.counts = DailyCounts{Issued: 599, Answered: 10}
	site := scrapingSite()
	if err := f.usecase.admitChallenge(context.Background(), site); err != nil {
		t.Fatalf("a flagged key is throttled, not refused: %v", err)
	}
	if f.sites.flagged != 1 || !slices.Equal(f.sites.events, []string{"flagged"}) {
		t.Fatalf("flagged %d times, events %q", f.sites.flagged, f.sites.events)
	}
	// Once the key carries today's flag, it is not flagged again.
	site.FlaggedAt = f.now.Add(-time.Hour)
	if err := f.usecase.admitChallenge(context.Background(), site); err != nil || f.sites.flagged != 1 {
		t.Fatalf("flagged again: %v, %d", err, f.sites.flagged)
	}
	// Yesterday's flag does not count: 2026-10-07 in Ho Chi Minh City.
	site.FlaggedAt = time.Date(2026, 10, 7, 16, 59, 0, 0, time.UTC)
	if err := f.usecase.admitChallenge(context.Background(), site); err != nil || f.sites.flagged != 2 {
		t.Fatalf("yesterday's flag kept: %v, %d", err, f.sites.flagged)
	}
}

func TestAdmitChallengeSuspends(t *testing.T) {
	f := newFixture(scrapingSite())
	f.counters.counts = DailyCounts{Issued: 4999, Answered: 100}
	err := f.usecase.admitChallenge(context.Background(), scrapingSite())
	if !errors.Is(err, ErrApiKeySuspended) || !slices.Equal(f.sites.events, []string{"suspended"}) {
		t.Fatalf("got %v, events %q", err, f.sites.events)
	}
	// Already suspended: refused again, but logged once.
	err = f.usecase.admitChallenge(context.Background(), scrapingSite())
	if !errors.Is(err, ErrApiKeySuspended) || len(f.sites.events) != 1 {
		t.Fatalf("got %v, events %q", err, f.sites.events)
	}
}

func TestAdmitChallengeQuotaFirst(t *testing.T) {
	f := newFixture(scrapingSite())
	f.counters.used = 5000
	if err := f.usecase.admitChallenge(context.Background(), scrapingSite()); !errors.Is(err, ErrDailyQuotaExceeded) {
		t.Fatalf("got %v, want DAILY_QUOTA_EXCEEDED", err)
	}
	if f.counters.counts.Issued != 0 {
		t.Fatal("a challenge refused for quota was counted for the scraping check")
	}
}
