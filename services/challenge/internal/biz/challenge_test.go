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
		return s, nil
	}
	return nil, ErrSiteKeyInvalid
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
		"pk_free": {APIKeyID: "k1", ChallengeTypes: []string{TypeImageDistort}},
		"pk_none": {APIKeyID: "k2"},
	}, fakeChallenges{}, &conf.Challenge{Ttl: durationpb.New(time.Minute)})
	if err != nil {
		t.Fatalf("NewChallengeUsecase() error = %v", err)
	}

	if _, err := uc.New(ctx, "pk_unknown"); !errors.IsUnauthorized(err) {
		t.Fatalf("New(unknown) error = %v, want unauthorized", err)
	}
	if _, err := uc.New(ctx, "pk_none"); !errors.IsForbidden(err) {
		t.Fatalf("New(type not allowed) error = %v, want forbidden", err)
	}

	c, err := uc.New(ctx, "pk_free")
	if err != nil || c.APIKeyID != "k1" || len(c.Image) == 0 {
		t.Fatalf("New() = %+v, %v", c, err)
	}
	if _, ok, err := uc.Check(ctx, c.ID, strings.ToLower(c.Answer)); err != nil || !ok {
		t.Fatalf("Check(correct) = %v, %v, want true", ok, err)
	}
	if _, _, err := uc.Check(ctx, c.ID, c.Answer); !errors.IsNotFound(err) {
		t.Fatalf("Check(reused) error = %v, want not found", err)
	}

	c, _ = uc.New(ctx, "pk_free")
	if _, ok, _ := uc.Check(ctx, c.ID, "wrong!"); ok {
		t.Fatal("Check(wrong) = true, want false")
	}
	if _, _, err := uc.Check(ctx, c.ID, c.Answer); !errors.IsNotFound(err) {
		t.Fatalf("Check(after wrong) error = %v, want not found", err)
	}
}
