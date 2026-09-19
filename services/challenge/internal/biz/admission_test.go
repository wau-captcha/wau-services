package biz

import (
	"context"
	"net/http"
	"testing"
	"time"

	"challenge/internal/conf"

	v1 "common/proto/challenge/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/durationpb"
)

var testQuota = &conf.Quota{TimeZone: "Asia/Ho_Chi_Minh"}

type fakeCounters struct {
	used     map[string]int64
	issued   map[string]int64
	verified map[string]int64
	days     []string
}

func newFakeCounters() *fakeCounters {
	return &fakeCounters{used: map[string]int64{}, issued: map[string]int64{}, verified: map[string]int64{}}
}

func (f *fakeCounters) QuotaUsed(_ context.Context, userID, day string) (int64, error) {
	f.days = append(f.days, day)
	return f.used[userID], nil
}

func (f *fakeCounters) CountChallenge(_ context.Context, apiKeyID, day string) (int64, int64, error) {
	f.days = append(f.days, day)
	f.issued[apiKeyID]++
	return f.issued[apiKeyID], f.verified[apiKeyID], nil
}

func newAdmissionFixture(t *testing.T, site Site) (*ChallengeUsecase, fakeSites, *fakeCounters) {
	t.Helper()
	sites := fakeSites{"pk": &site}
	counters := newFakeCounters()
	uc, err := NewChallengeUsecase(sites, fakeChallenges{}, counters,
		&conf.Challenge{Ttl: durationpb.New(time.Minute)}, testQuota)
	if err != nil {
		t.Fatalf("NewChallengeUsecase: %v", err)
	}
	return uc, sites, counters
}

func openSite() Site {
	return Site{APIKeyID: "k1", UserID: "u1", DailyQuota: 1000, ChallengeTypes: []string{TypeImageDistort}}
}

func wantReason(t *testing.T, err error, reason v1.ErrorReason, code int) {
	t.Helper()
	if errors.Reason(err) != reason.String() || errors.Code(err) != code {
		t.Fatalf("err = %v, want %s (%d)", err, reason, code)
	}
}

func TestNewRejectsBlockedKeysWithoutCountingThem(t *testing.T) {
	tests := map[string]struct {
		change func(*Site, *fakeCounters)
		reason v1.ErrorReason
		code   int
	}{
		"banned owner": {func(s *Site, _ *fakeCounters) { s.OwnerBanned, s.Suspended = true, true },
			v1.ErrorReason_ACCOUNT_BANNED, http.StatusForbidden},
		"suspended key": {func(s *Site, _ *fakeCounters) { s.Suspended = true },
			v1.ErrorReason_API_KEY_SUSPENDED, http.StatusForbidden},
		"quota used up": {func(_ *Site, c *fakeCounters) { c.used["u1"] = 1000 },
			v1.ErrorReason_DAILY_QUOTA_EXCEEDED, http.StatusTooManyRequests},
		"quota overshot": {func(_ *Site, c *fakeCounters) { c.used["u1"] = 1003 },
			v1.ErrorReason_DAILY_QUOTA_EXCEEDED, http.StatusTooManyRequests},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			site := openSite()
			uc, sites, counters := newAdmissionFixture(t, site)
			tt.change(sites["pk"], counters)

			_, err := uc.New(context.Background(), "pk", "")
			wantReason(t, err, tt.reason, tt.code)
			if counters.issued["k1"] != 0 {
				t.Fatalf("a refused request was counted as an issued challenge")
			}
		})
	}
}

func TestNewAllowsTheLastVerificationOfTheQuota(t *testing.T) {
	uc, _, counters := newAdmissionFixture(t, openSite())
	counters.used["u1"] = 999
	if _, err := uc.New(context.Background(), "pk", ""); err != nil {
		t.Fatalf("New with one verification left: %v", err)
	}
}

func TestNewSuspendsScrapingKeys(t *testing.T) {
	tests := map[string]struct {
		issuedBefore, verified int64
		suspended              bool
	}{
		"199th challenge is never judged":     {198, 0, false},
		"200th with 59 answers is scraping":   {199, 59, true},
		"200th with 60 answers is 30 percent": {199, 60, false},
		"1000th with 299 answers":             {999, 299, true},
		"1000th with 300 answers":             {999, 300, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			uc, sites, counters := newAdmissionFixture(t, openSite())
			counters.issued["k1"] = tt.issuedBefore
			counters.verified["k1"] = tt.verified

			_, err := uc.New(context.Background(), "pk", "")
			if tt.suspended {
				wantReason(t, err, v1.ErrorReason_API_KEY_SUSPENDED, http.StatusForbidden)
			} else if err != nil {
				t.Fatalf("New: %v", err)
			}
			if sites["pk"].Suspended != tt.suspended {
				t.Fatalf("suspended = %v, want %v", sites["pk"].Suspended, tt.suspended)
			}
		})
	}
}

func TestSuspendedKeyStaysBlocked(t *testing.T) {
	uc, _, counters := newAdmissionFixture(t, openSite())
	counters.issued["k1"] = 199
	if _, err := uc.New(context.Background(), "pk", ""); err == nil {
		t.Fatal("scraping key was not suspended")
	}
	_, err := uc.New(context.Background(), "pk", "")
	wantReason(t, err, v1.ErrorReason_API_KEY_SUSPENDED, http.StatusForbidden)
	if counters.issued["k1"] != 200 {
		t.Fatalf("issued = %d, want 200: requests after the suspension must not be counted", counters.issued["k1"])
	}
}

func TestCountersUseTheQuotaTimeZone(t *testing.T) {
	uc, _, counters := newAdmissionFixture(t, openSite())
	// 17:30 UTC on the 18th is already the 19th in Ho Chi Minh City.
	uc.now = func() time.Time { return time.Date(2026, 9, 18, 17, 30, 0, 0, time.UTC) }
	if _, err := uc.New(context.Background(), "pk", ""); err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, day := range counters.days {
		if day != "2026-09-19" {
			t.Fatalf("counter day = %s, want 2026-09-19", day)
		}
	}
}

func TestQuotaTimeZoneIsRequired(t *testing.T) {
	for _, zone := range []string{"", "Mars/Olympus"} {
		_, err := NewChallengeUsecase(fakeSites{}, fakeChallenges{}, newFakeCounters(),
			&conf.Challenge{Ttl: durationpb.New(time.Minute)}, &conf.Quota{TimeZone: zone})
		if err == nil {
			t.Fatalf("time zone %q accepted", zone)
		}
	}
}
