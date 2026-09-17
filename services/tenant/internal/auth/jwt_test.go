package auth

import (
	"context"
	"testing"
	"time"

	"tenant/internal/biz"
	"tenant/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/durationpb"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func testJWT(t *testing.T) *JWT {
	t.Helper()
	j, err := NewJWT(&conf.Auth{Secret: testSecret, Ttl: durationpb.New(time.Hour)})
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	return j
}

func TestNewJWTRejectsWeakConfig(t *testing.T) {
	tests := map[string]*conf.Auth{
		"short secret": {Secret: "short", Ttl: durationpb.New(time.Hour)},
		"no secret":    {Ttl: durationpb.New(time.Hour)},
		"zero ttl":     {Secret: testSecret, Ttl: durationpb.New(0)},
		"no ttl":       {Secret: testSecret},
	}
	for name, c := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewJWT(c); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestIssueParseRoundTrip(t *testing.T) {
	j := testJWT(t)
	id := uuid.New()

	token, ttl, err := j.Issue(id)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if ttl != time.Hour {
		t.Fatalf("ttl = %v, want 1h", ttl)
	}

	got, err := j.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got != id {
		t.Fatalf("id = %v, want %v", got, id)
	}
}

func TestParseRejects(t *testing.T) {
	j := testJWT(t)
	id := uuid.New()

	other, err := NewJWT(&conf.Auth{Secret: "fedcba9876543210fedcba9876543210", Ttl: durationpb.New(time.Hour)})
	if err != nil {
		t.Fatalf("NewJWT: %v", err)
	}
	otherToken, _, _ := other.Issue(id)

	expired := testJWT(t)
	expired.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expiredToken, _, _ := expired.Issue(id)

	noneToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject:   id.String(),
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}

	wrongIssuer, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   id.String(),
		Issuer:    "somebody-else",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign wrong issuer: %v", err)
	}

	badSubject, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "not-a-uuid",
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign bad subject: %v", err)
	}

	noExpiry, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: id.String(),
		Issuer:  issuer,
	}).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign without expiry: %v", err)
	}

	tests := map[string]string{
		"empty":            "",
		"garbage":          "not.a.token",
		"other secret":     otherToken,
		"expired":          expiredToken,
		"alg none":         noneToken,
		"wrong issuer":     wrongIssuer,
		"subject not uuid": badSubject,
		"no expiry":        noExpiry,
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := j.Parse(token); !errors.Is(err, biz.ErrUnauthenticated) {
				t.Fatalf("err = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

// fakeTransport carries an Authorization header the middleware can read.
type fakeTransport struct {
	transport.Transporter
	header transport.Header
}

func (t *fakeTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (t *fakeTransport) Endpoint() string                { return "" }
func (t *fakeTransport) Operation() string               { return "/tenant.v1.AuthService/GetMe" }
func (t *fakeTransport) RequestHeader() transport.Header { return t.header }
func (t *fakeTransport) ReplyHeader() transport.Header   { return t.header }

type headerMap map[string][]string

func (h headerMap) Get(key string) string      { return h[key][0] }
func (h headerMap) Set(key, value string)      { h[key] = []string{value} }
func (h headerMap) Add(key, value string)      { h[key] = append(h[key], value) }
func (h headerMap) Values(key string) []string { return h[key] }
func (h headerMap) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}

func contextWithAuthorization(value string) context.Context {
	header := headerMap{}
	if value != "" {
		header.Set("Authorization", value)
	} else {
		header.Set("Authorization", "")
	}
	return transport.NewServerContext(context.Background(), &fakeTransport{header: header})
}

func TestMiddleware(t *testing.T) {
	j := testJWT(t)
	id := uuid.New()
	token, _, _ := j.Issue(id)

	t.Run("valid bearer", func(t *testing.T) {
		var seen uuid.UUID
		handler := Middleware(j)(func(ctx context.Context, _ any) (any, error) {
			seen, _ = UserID(ctx)
			return nil, nil
		})
		if _, err := handler(contextWithAuthorization("Bearer "+token), nil); err != nil {
			t.Fatalf("handler: %v", err)
		}
		if seen != id {
			t.Fatalf("user id = %v, want %v", seen, id)
		}
	})

	rejected := map[string]string{
		"missing":      "",
		"wrong scheme": "Basic " + token,
		"bad token":    "Bearer not-a-token",
	}
	for name, value := range rejected {
		t.Run(name, func(t *testing.T) {
			called := false
			handler := Middleware(j)(func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			_, err := handler(contextWithAuthorization(value), nil)
			if !errors.Is(err, biz.ErrUnauthenticated) {
				t.Fatalf("err = %v, want ErrUnauthenticated", err)
			}
			if called {
				t.Fatal("handler ran for a rejected request")
			}
		})
	}
}
