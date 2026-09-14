package biz

import (
	"context"
	"testing"
	"time"

	"verify/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/durationpb"
)

type fakeChallenges struct{ sol *Solution }

func (f fakeChallenges) CheckAnswer(context.Context, string, string) (*Solution, error) {
	return f.sol, nil
}

type fakeGrants map[string]*Grant

func (f fakeGrants) Save(_ context.Context, id string, g *Grant, _ time.Duration) error {
	f[id] = g
	return nil
}

func (f fakeGrants) Take(_ context.Context, id string) (*Grant, error) {
	g := f[id]
	delete(f, id)
	return g, nil
}

// fakeSites maps secret keys to API key ids.
type fakeSites map[string]string

func (f fakeSites) FindAPIKeyID(_ context.Context, secretKey string) (string, error) {
	if id, ok := f[secretKey]; ok {
		return id, nil
	}
	return "", ErrSecretKeyInvalid
}

func (fakeSites) RecordUsage(context.Context, string, string, bool) error { return nil }

func newTestUsecase(t *testing.T, sol *Solution) *VerifyUsecase {
	t.Helper()
	uc, err := NewVerifyUsecase(fakeChallenges{sol}, fakeGrants{}, fakeSites{"sk_a": "a", "sk_b": "b"},
		&conf.Token{Secret: "secret", Ttl: durationpb.New(time.Minute)})
	if err != nil {
		t.Fatalf("NewVerifyUsecase() error = %v", err)
	}
	return uc
}

func TestVerifyUsecase(t *testing.T) {
	ctx := context.Background()
	uc := newTestUsecase(t, &Solution{Success: true, APIKeyID: "a", ChallengeType: "image_distort"})

	token, err := uc.VerifyChallenge(ctx, "c1", "abcde")
	if err != nil || token == nil {
		t.Fatalf("VerifyChallenge() = %v, %v, want token", token, err)
	}
	if _, err := uc.SiteVerify(ctx, "sk_unknown", token.Value); !errors.IsUnauthorized(err) {
		t.Fatalf("SiteVerify(unknown secret) error = %v, want unauthorized", err)
	}
	if g, err := uc.SiteVerify(ctx, "sk_a", token.Value); err != nil || g == nil || g.ChallengeType != "image_distort" {
		t.Fatalf("SiteVerify() = %+v, %v, want grant", g, err)
	}
	if g, _ := uc.SiteVerify(ctx, "sk_a", token.Value); g != nil {
		t.Fatal("SiteVerify(reused) returned a grant")
	}

	other, _ := uc.VerifyChallenge(ctx, "c2", "abcde")
	if g, _ := uc.SiteVerify(ctx, "sk_b", other.Value); g != nil {
		t.Fatal("SiteVerify(other site) returned a grant")
	}
}

func TestVerifyChallengeWrongAnswer(t *testing.T) {
	uc := newTestUsecase(t, &Solution{APIKeyID: "a", ChallengeType: "image_distort"})
	if token, err := uc.VerifyChallenge(context.Background(), "c1", "wrong"); err != nil || token != nil {
		t.Fatalf("VerifyChallenge(wrong) = %v, %v, want nil", token, err)
	}
}

func TestToken(t *testing.T) {
	secret, now := []byte("secret"), time.Now()
	id, token := newToken(secret, now.Add(time.Minute))
	if got, ok := parseToken(secret, token, now); !ok || got != id {
		t.Fatalf("parseToken() = %q, %v, want %q", got, ok, id)
	}
	for name, tc := range map[string]struct {
		secret []byte
		token  string
		now    time.Time
	}{
		"expired":      {secret, token, now.Add(time.Minute)},
		"wrong secret": {[]byte("other"), token, now},
		"tampered":     {secret, "x" + token, now},
		"malformed":    {secret, "garbage", now},
	} {
		if _, ok := parseToken(tc.secret, tc.token, tc.now); ok {
			t.Errorf("parseToken(%s) = ok, want rejected", name)
		}
	}
}
