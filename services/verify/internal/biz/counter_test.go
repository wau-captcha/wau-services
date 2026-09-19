package biz

import (
	"context"
	"net/http"
	"testing"
	"time"

	"verify/internal/conf"

	v1 "common/proto/verify/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/durationpb"
)

type fakeCounters struct {
	verified  map[string]int
	succeeded map[string]int
	days      []string
	fail      error
}

func (f *fakeCounters) CountVerification(_ context.Context, apiKeyID, day string) error {
	f.days = append(f.days, day)
	f.verified[apiKeyID]++
	return f.fail
}

func (f *fakeCounters) CountSuccess(_ context.Context, userID, day string) error {
	f.days = append(f.days, day)
	f.succeeded[userID]++
	return f.fail
}

func TestEveryCheckedAnswerFeedsTheScrapingCheck(t *testing.T) {
	uc, counters := newCountingUsecase(t, &Solution{APIKeyID: "a", UserID: "u1", ChallengeType: "image_distort"})
	if _, err := uc.VerifyChallenge(context.Background(), "c1", "wrong"); err != nil {
		t.Fatalf("VerifyChallenge: %v", err)
	}
	if counters.verified["a"] != 1 || counters.succeeded["u1"] != 0 {
		t.Fatalf("wrong answer: verified=%d quota=%d, want 1 and 0", counters.verified["a"], counters.succeeded["u1"])
	}
}

func TestCorrectAnswerCountsAgainstTheOwnersQuota(t *testing.T) {
	uc, counters := newCountingUsecase(t, &Solution{Success: true, APIKeyID: "a", UserID: "u1", ChallengeType: "image_distort"})
	if _, err := uc.VerifyChallenge(context.Background(), "c1", "abcde"); err != nil {
		t.Fatalf("VerifyChallenge: %v", err)
	}
	if counters.verified["a"] != 1 || counters.succeeded["u1"] != 1 {
		t.Fatalf("correct answer: verified=%d quota=%d, want 1 and 1", counters.verified["a"], counters.succeeded["u1"])
	}
}

func TestChallengeWithoutOwnerSkipsTheQuota(t *testing.T) {
	uc, counters := newCountingUsecase(t, &Solution{Success: true, APIKeyID: "a", ChallengeType: "image_distort"})
	if token, err := uc.VerifyChallenge(context.Background(), "c1", "abcde"); err != nil || token == nil {
		t.Fatalf("VerifyChallenge = %v, %v, want a token", token, err)
	}
	if len(counters.succeeded) != 0 {
		t.Fatalf("quota counted for an empty user id: %v", counters.succeeded)
	}
}

func TestCounterFailureStillIssuesTheToken(t *testing.T) {
	uc, counters := newCountingUsecase(t, &Solution{Success: true, APIKeyID: "a", UserID: "u1", ChallengeType: "image_distort"})
	counters.fail = errors.InternalServer("REDIS", "down")
	if token, err := uc.VerifyChallenge(context.Background(), "c1", "abcde"); err != nil || token == nil {
		t.Fatalf("VerifyChallenge = %v, %v, want a token despite the counter failure", token, err)
	}
}

func TestCountersUseTheQuotaTimeZone(t *testing.T) {
	uc, counters := newCountingUsecase(t, &Solution{Success: true, APIKeyID: "a", UserID: "u1", ChallengeType: "image_distort"})
	uc.now = func() time.Time { return time.Date(2026, 9, 18, 17, 30, 0, 0, time.UTC) }
	if _, err := uc.VerifyChallenge(context.Background(), "c1", "abcde"); err != nil {
		t.Fatalf("VerifyChallenge: %v", err)
	}
	for _, day := range counters.days {
		if day != "2026-09-19" {
			t.Fatalf("counter day = %s, want 2026-09-19", day)
		}
	}
}

func TestSiteVerifyRefusesBlockedKeys(t *testing.T) {
	uc := newTestUsecase(t, &Solution{Success: true, APIKeyID: "s", ChallengeType: "image_distort"})
	token, err := uc.VerifyChallenge(context.Background(), "c1", "abcde")
	if err != nil || token == nil {
		t.Fatalf("VerifyChallenge = %v, %v", token, err)
	}
	tests := map[string]v1.ErrorReason{
		"sk_suspended": v1.ErrorReason_API_KEY_SUSPENDED,
		"sk_banned":    v1.ErrorReason_ACCOUNT_BANNED,
	}
	for secret, reason := range tests {
		_, err := uc.SiteVerify(context.Background(), secret, token.Value)
		if errors.Reason(err) != reason.String() || errors.Code(err) != http.StatusForbidden {
			t.Errorf("SiteVerify(%s) err = %v, want %s", secret, err, reason)
		}
	}
}

func TestQuotaTimeZoneIsRequired(t *testing.T) {
	for _, zone := range []string{"", "Mars/Olympus"} {
		_, err := NewVerifyUsecase(fakeChallenges{}, fakeGrants{}, fakeSites{}, &fakeCounters{},
			&conf.Token{Secret: "secret", Ttl: durationpb.New(time.Minute)}, &conf.Quota{TimeZone: zone})
		if err == nil {
			t.Fatalf("time zone %q accepted", zone)
		}
	}
}
