package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	"challenge/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/durationpb"
)

type fakeSites map[string]*Site

func (f fakeSites) FindBySiteKey(_ context.Context, siteKey string) (*Site, error) {
	if s, ok := f[siteKey]; ok {
		copied := *s
		return &copied, nil
	}
	return nil, ErrSiteKeyInvalid
}

func (f fakeSites) SuspendForScraping(_ context.Context, apiKeyID string) error {
	for _, s := range f {
		if s.APIKeyID == apiKeyID {
			s.Suspended = true
		}
	}
	return nil
}

type fakeChallenges map[string]*Challenge

func (f fakeChallenges) Save(_ context.Context, c *Challenge) error {
	f[c.ID] = c
	return nil
}

func (f fakeChallenges) Take(_ context.Context, id string) (*Challenge, error) {
	c, ok := f[id]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	delete(f, id)
	return c, nil
}

func TestChallengeUsecase(t *testing.T) {
	ctx := context.Background()
	uc, err := NewChallengeUsecase(fakeSites{
		"pk_free":    {APIKeyID: "k1", UserID: "u1", DailyQuota: 1000, ChallengeTypes: []string{TypeImageDistort}},
		"pk_premium": {APIKeyID: "k2", UserID: "u2", DailyQuota: 1000, ChallengeTypes: []string{TypeImageDistort, TypeMotionNoise}},
		"pk_none":    {APIKeyID: "k3", UserID: "u3", DailyQuota: 1000},
	}, fakeChallenges{}, newFakeCounters(), &conf.Challenge{Ttl: durationpb.New(time.Minute)}, testQuota)
	if err != nil {
		t.Fatalf("NewChallengeUsecase() error = %v", err)
	}

	if _, err := uc.New(ctx, "pk_free", "slide_puzzle"); !errors.IsBadRequest(err) {
		t.Fatalf("New(unsupported type) error = %v, want bad request", err)
	}
	if _, err := uc.New(ctx, "pk_unknown", ""); !errors.IsUnauthorized(err) {
		t.Fatalf("New(unknown) error = %v, want unauthorized", err)
	}
	if _, err := uc.New(ctx, "pk_none", ""); !errors.IsForbidden(err) {
		t.Fatalf("New(no types) error = %v, want forbidden", err)
	}
	if _, err := uc.New(ctx, "pk_free", TypeMotionNoise); !errors.IsForbidden(err) {
		t.Fatalf("New(motion_noise on free) error = %v, want forbidden", err)
	}

	video, err := uc.New(ctx, "pk_premium", TypeMotionNoise)
	if err != nil {
		t.Fatalf("New(motion_noise) error = %v", err)
	}
	if video.Type != TypeMotionNoise || video.Media.MIMEType != "image/gif" {
		t.Fatalf("New(motion_noise) type = %q, mime = %q", video.Type, video.Media.MIMEType)
	}
	if strings.Trim(video.Answer, uppercaseChars) != "" {
		t.Fatalf("New(motion_noise) answer = %q, want uppercase letters only", video.Answer)
	}

	c, err := uc.New(ctx, "pk_free", "")
	if err != nil {
		t.Fatalf("New(default) error = %v", err)
	}
	if c.Type != TypeImageDistort || c.APIKeyID != "k1" || c.UserID != "u1" || len(c.Media.Data) == 0 {
		t.Fatalf("New(default) type = %q, api key = %q", c.Type, c.APIKeyID)
	}
	if _, ok, err := uc.Check(ctx, c.ID, strings.ToLower(c.Answer)); err != nil || !ok {
		t.Fatalf("Check(correct) = %v, %v, want true", ok, err)
	}
	if _, _, err := uc.Check(ctx, c.ID, c.Answer); !errors.IsNotFound(err) {
		t.Fatalf("Check(reused) error = %v, want not found", err)
	}

	c, _ = uc.New(ctx, "pk_free", "")
	if _, ok, _ := uc.Check(ctx, c.ID, "wrong!"); ok {
		t.Fatal("Check(wrong) = true, want false")
	}
	if _, _, err := uc.Check(ctx, c.ID, c.Answer); !errors.IsNotFound(err) {
		t.Fatalf("Check(after wrong) error = %v, want not found", err)
	}
}
