package biz

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
)

const testChallengeID = "0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f"

var (
	visitor = netip.MustParseAddr("203.0.113.4")
	powSalt = []byte("0123456789abcdef")
)

// issued is a challenge as New stores it for a visitor of shop.example.com on Chrome.
func issued(now time.Time) *Challenge {
	return &Challenge{
		ID: testChallengeID, Type: TypeImageDistort, APIKeyID: "key-1", SiteKey: "pk_shop", UserID: "user-1", Answer: "ABCDE",
		TTL: 2 * time.Minute, IssuedAt: now.Add(-10 * time.Second), Origin: "https://shop.example.com",
		ClientIP: visitor.String(), UserAgentHash: hashUserAgent(chromeUA), PowSalt: powSalt, PowDifficulty: 8,
	}
}

// answered is the right answer to issued from the same visitor, as SDK 1.3.0 sends it.
func answered() Answer {
	return Answer{
		ChallengeID: testChallengeID, Text: "abcde", Behavior: humanBehavior(), SiteKey: "pk_shop",
		Origin: "https://shop.example.com", Client: visitor, UserAgent: chromeUA, SecFetchSite: "cross-site",
		AcceptLanguage: "vi-VN,vi;q=0.9", PowNonce: solvePow(powSalt, testChallengeID, 8),
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name             string
		challenge        func(*Challenge)
		answer           func(*Answer)
		right            bool
		verdict          string
		reasons          []string
		behaviorRequired bool
	}{
		{"right", nil, nil, true, VerdictHuman, []string{}, false},
		{"wrong text", nil, func(a *Answer) { a.Text = "abcdf" }, false, VerdictHuman, []string{}, false},
		{"old client without a site key", nil, func(a *Answer) { a.SiteKey = "" }, true, VerdictHuman, []string{}, false},
		{"another site key", nil, func(a *Answer) { a.SiteKey = "pk_other" }, false, VerdictHuman, []string{"site_key_mismatch"}, false},
		{"another website", nil, func(a *Answer) { a.Origin = "https://evil.test" }, false, VerdictHuman, []string{"origin_changed"}, false},
		{"no website", nil, func(a *Answer) { a.Origin = "" }, false, VerdictHuman, []string{"origin_changed"}, false},
		{"old verify that passes no request", nil, func(a *Answer) { a.Client, a.Origin, a.UserAgent = netip.Addr{}, "", "" },
			true, VerdictHuman, []string{}, false},
		{"no proof of work", nil, func(a *Answer) { a.PowNonce = "" }, true, VerdictSuspicious, []string{"no_pow"}, false},
		{"wrong proof of work", nil, func(a *Answer) { a.PowNonce = failingPow(powSalt, testChallengeID, 8) },
			false, VerdictAutomated, []string{"pow_invalid"}, false},
		{"challenge without proof of work", func(c *Challenge) { c.PowSalt, c.PowDifficulty = nil, 0 },
			func(a *Answer) { a.PowNonce = "" }, true, VerdictHuman, []string{}, false},
		{"automation client", nil, func(a *Answer) { a.UserAgent = "python-requests/2.32.3" }, false, VerdictAutomated, []string{"automation_ua"}, false},
		{"another address", nil, func(a *Answer) { a.Client = netip.MustParseAddr("198.51.100.7") }, true, VerdictHuman, []string{"ip_changed"}, false},
		{"no behaviour", nil, func(a *Answer) { a.Behavior = nil }, true, VerdictNotMeasured, nil, false},
		{"no behaviour for a key that requires it", func(c *Challenge) { c.RequireBehavior = true },
			func(a *Answer) { a.Behavior = nil }, false, VerdictNotMeasured, nil, true},
		{"behaviour for a key that requires it", func(c *Challenge) { c.RequireBehavior = true }, nil, true, VerdictHuman, []string{}, false},
	}
	for _, test := range tests {
		f := newFixture(nil)
		challenge := issued(f.now)
		if test.challenge != nil {
			test.challenge(challenge)
		}
		f.challenges.stored[challenge.ID] = challenge
		answer := answered()
		if test.answer != nil {
			test.answer(&answer)
		}
		outcome, err := f.usecase.Check(context.Background(), answer)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		got := outcome.Behavior
		if outcome.Right != test.right || got.Verdict != test.verdict || !slices.Equal(got.Reasons, test.reasons) ||
			outcome.BehaviorRequired != test.behaviorRequired {
			t.Errorf("%s: got right %v, %s %q, behaviour required %v; want %v, %s %q, %v", test.name,
				outcome.Right, got.Verdict, got.Reasons, outcome.BehaviorRequired, test.right, test.verdict, test.reasons, test.behaviorRequired)
		}
		if len(f.challenges.stored) != 0 {
			t.Errorf("%s: the challenge was not consumed", test.name)
		}
	}
}

func TestCheckConsumesTheChallenge(t *testing.T) {
	f := newFixture(nil)
	f.challenges.stored[testChallengeID] = issued(f.now)
	answer := answered()
	answer.Text = "wrong"
	if _, err := f.usecase.Check(context.Background(), answer); err != nil {
		t.Fatal(err)
	}
	if _, err := f.usecase.Check(context.Background(), answered()); !errors.Is(err, ErrChallengeNotFound) {
		t.Fatalf("second answer: %v, want CHALLENGE_NOT_FOUND", err)
	}
}

func TestAddressHistory(t *testing.T) {
	f := newFixture(scrapingSite())
	f.now = time.Date(2026, 10, 8, 3, 4, 30, 0, time.UTC)
	addr := visitor.String()

	// Wrong answers and automated verdicts are counted; a right answer refused for want of
	// behaviour is not a wrong one.
	for _, mutate := range []func(*Answer){
		func(a *Answer) { a.Text = "wrong" },
		func(a *Answer) { a.UserAgent = "curl/8.9.1" },
	} {
		f.challenges.stored[testChallengeID] = issued(f.now)
		answer := answered()
		mutate(&answer)
		if _, err := f.usecase.Check(context.Background(), answer); err != nil {
			t.Fatal(err)
		}
	}
	required := issued(f.now)
	required.RequireBehavior = true
	f.challenges.stored[testChallengeID] = required
	answer := answered()
	answer.Behavior = nil
	if _, err := f.usecase.Check(context.Background(), answer); err != nil {
		t.Fatal(err)
	}
	if f.history.wrong[addr] != 2 || f.history.automated[addr] != 1 {
		t.Fatalf("history: %d wrong, %d automated; want 2 and 1", f.history.wrong[addr], f.history.automated[addr])
	}

	// Past 5 automated verdicts in the hour, later answers carry ip_bad_history.
	f.history.automated[addr] = 6
	f.challenges.stored[testChallengeID] = issued(f.now)
	outcome, err := f.usecase.Check(context.Background(), answered())
	if err != nil || !slices.Contains(outcome.Behavior.Reasons, "ip_bad_history") {
		t.Fatalf("got %+v, %v; want ip_bad_history", outcome, err)
	}

	// 20 wrong answers in the bucket are allowed, the 21st blocks the address until 03:10.
	f.history.wrong[addr] = 20
	f.challenges.stored[testChallengeID] = issued(f.now)
	if _, err := f.usecase.Check(context.Background(), answered()); err != nil {
		t.Fatalf("20 wrong answers: %v", err)
	}
	f.history.wrong[addr] = 21
	f.challenges.stored[testChallengeID] = issued(f.now)
	for _, call := range []func() error{
		func() error { _, err := f.usecase.Check(context.Background(), answered()); return err },
		func() error {
			_, _, err := f.usecase.New(context.Background(), NewRequest{SiteKey: "pk_shop", Client: visitor})
			return err
		},
	} {
		err := call()
		if errors.Code(err) != http.StatusTooManyRequests || errors.FromError(err).Metadata["retry_after"] != "330" {
			t.Fatalf("got %v, want RATE_LIMITED for the 330 s left of the bucket", err)
		}
	}
	if len(f.challenges.stored) != 1 {
		t.Fatal("a blocked address consumed the challenge")
	}
	// Another address is not affected.
	other := answered()
	other.Client = netip.MustParseAddr("198.51.100.7")
	if _, err := f.usecase.Check(context.Background(), other); err != nil {
		t.Fatalf("another address: %v", err)
	}
}
