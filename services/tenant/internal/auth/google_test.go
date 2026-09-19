package auth

import (
	"context"
	"testing"
	"time"

	"tenant/internal/biz"
	"tenant/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/golang-jwt/jwt/v5"
)

func TestNewGoogleRequiresClientID(t *testing.T) {
	if _, err := NewGoogle(&conf.Google{}); err == nil {
		t.Fatal("want error without a client id")
	}
}

func TestNewGoogleRequiresJWKSURL(t *testing.T) {
	if _, err := NewGoogle(&conf.Google{ClientId: testClientID}); err == nil {
		t.Fatal("want error without a JWKS URL")
	}
}

func TestVerifyAcceptsGoogleToken(t *testing.T) {
	f := newFakeGoogle(t)
	key := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)

	identity, err := g.Verify(context.Background(), signRS256(t, "k1", key, tokenClaims(clock.now)), testNonce)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	want := biz.GoogleIdentity{
		Subject:            "110169484474386276334",
		Email:              "ann.lee@gmail.com",
		EmailVerified:      true,
		EmailAuthoritative: true,
		Name:               "Ann Lee",
	}
	if *identity != want {
		t.Fatalf("identity = %+v, want %+v", *identity, want)
	}
}

func TestVerifyEmailFlags(t *testing.T) {
	tests := map[string]struct {
		email         string
		verified      any
		hostedDomain  string
		wantVerified  bool
		wantAuthority bool
	}{
		"gmail":                   {"a@gmail.com", true, "", true, true},
		"workspace":               {"a@acme.io", true, "acme.io", true, true},
		"other provider":          {"a@yahoo.com", true, "", true, false},
		"unverified gmail":        {"a@gmail.com", false, "", false, false},
		"verified as string":      {"a@gmail.com", "true", "", true, true},
		"unverified as string":    {"a@gmail.com", "false", "", false, false},
		"missing email":           {"", true, "", false, false},
		"unverified on workspace": {"a@acme.io", false, "acme.io", false, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			f := newFakeGoogle(t)
			key := f.publish(t, "k1")
			g, clock := newTestVerifier(t, f)
			claims := tokenClaims(clock.now)
			claims["email"] = tt.email
			claims["email_verified"] = tt.verified
			if tt.hostedDomain != "" {
				claims["hd"] = tt.hostedDomain
			}

			identity, err := g.Verify(context.Background(), signRS256(t, "k1", key, claims), testNonce)
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if identity.EmailVerified != tt.wantVerified || identity.EmailAuthoritative != tt.wantAuthority {
				t.Fatalf("verified=%v authoritative=%v, want %v/%v",
					identity.EmailVerified, identity.EmailAuthoritative, tt.wantVerified, tt.wantAuthority)
			}
		})
	}
}

func TestVerifyRejects(t *testing.T) {
	f := newFakeGoogle(t)
	key := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	now := clock.now
	valid := func(change func(jwt.MapClaims)) string {
		claims := tokenClaims(now)
		change(claims)
		return signRS256(t, "k1", key, claims)
	}

	hs256 := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims(now))
	hs256.Header["kid"] = "k1"
	hmacToken, _ := hs256.SignedString([]byte("secret"))
	none := jwt.NewWithClaims(jwt.SigningMethodNone, tokenClaims(now))
	none.Header["kid"] = "k1"
	noneToken, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	noKid := jwt.NewWithClaims(jwt.SigningMethodRS256, tokenClaims(now))
	noKidToken, _ := noKid.SignedString(key)

	tests := map[string]struct{ token, nonce string }{
		"other audience":     {valid(func(c jwt.MapClaims) { c["aud"] = "another-app" }), testNonce},
		"no audience":        {valid(func(c jwt.MapClaims) { delete(c, "aud") }), testNonce},
		"other issuer":       {valid(func(c jwt.MapClaims) { c["iss"] = "https://evil.example" }), testNonce},
		"expired":            {valid(func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Minute).Unix() }), testNonce},
		"no expiry":          {valid(func(c jwt.MapClaims) { delete(c, "exp") }), testNonce},
		"issued in future":   {valid(func(c jwt.MapClaims) { c["iat"] = now.Add(5 * time.Minute).Unix() }), testNonce},
		"no subject":         {valid(func(c jwt.MapClaims) { c["sub"] = "" }), testNonce},
		"other nonce":        {valid(func(c jwt.MapClaims) { c["nonce"] = "someone-else" }), testNonce},
		"no nonce in token":  {valid(func(c jwt.MapClaims) { delete(c, "nonce") }), testNonce},
		"no expected nonce":  {valid(func(jwt.MapClaims) {}), ""},
		"foreign signature":  {signRS256(t, "k1", newRSAKey(t), tokenClaims(now)), testNonce},
		"hmac with key id":   {hmacToken, testNonce},
		"alg none":           {noneToken, testNonce},
		"no key id":          {noKidToken, testNonce},
		"garbage":            {"not.a.token", testNonce},
		"empty":              {"", testNonce},
		"bad email_verified": {valid(func(c jwt.MapClaims) { c["email_verified"] = "maybe" }), testNonce},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := g.Verify(context.Background(), tt.token, tt.nonce); !errors.Is(err, biz.ErrGoogleTokenInvalid) {
				t.Fatalf("err = %v, want ErrGoogleTokenInvalid", err)
			}
		})
	}
}

func TestVerifyAllowsSmallClockSkew(t *testing.T) {
	f := newFakeGoogle(t)
	key := f.publish(t, "k1")
	g, clock := newTestVerifier(t, f)
	claims := tokenClaims(clock.now)
	claims["iat"] = clock.now.Add(20 * time.Second).Unix()

	if _, err := g.Verify(context.Background(), signRS256(t, "k1", key, claims), testNonce); err != nil {
		t.Fatalf("Verify with 20s skew: %v", err)
	}
}
