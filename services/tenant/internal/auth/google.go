package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"tenant/internal/biz"
	"tenant/internal/conf"

	"github.com/golang-jwt/jwt/v5"
)

// Tolerated clock difference between Google and this server.
const tokenLeeway = 30 * time.Second

var defaultGoogleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

// Google verifies ID tokens that Google Identity Services issued for this app.
type Google struct {
	clientID string
	issuers  map[string]bool
	keys     *keySet
	now      func() time.Time
}

func NewGoogle(googleConf *conf.Google) (*Google, error) {
	if googleConf.GetClientId() == "" {
		return nil, errors.New("google client id is required (KRATOS_GOOGLE_CLIENT_ID)")
	}
	if googleConf.GetJwksUrl() == "" {
		return nil, errors.New("google jwks url is required (KRATOS_GOOGLE_JWKS_URL)")
	}
	issuers := googleConf.GetIssuers()
	if len(issuers) == 0 {
		issuers = defaultGoogleIssuers
	}
	accepted := make(map[string]bool, len(issuers))
	for _, issuer := range issuers {
		accepted[issuer] = true
	}
	return &Google{
		clientID: googleConf.GetClientId(),
		issuers:  accepted,
		keys:     newKeySet(googleConf.GetJwksUrl()),
		now:      time.Now,
	}, nil
}

type googleClaims struct {
	jwt.RegisteredClaims
	Email         string   `json:"email"`
	EmailVerified flexBool `json:"email_verified"`
	Name          string   `json:"name"`
	HostedDomain  string   `json:"hd"`
	Nonce         string   `json:"nonce"`
}

// flexBool accepts JSON booleans and the "true"/"false" strings older Google tokens used.
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "true", `"true"`:
		*b = true
	case "false", `"false"`, "null":
		*b = false
	default:
		return fmt.Errorf("invalid boolean %s", data)
	}
	return nil
}

// Verify follows Google's checklist: RS256 signature from a published key, audience,
// issuer, expiry and issue time, plus the nonce this browser was given.
func (g *Google) Verify(ctx context.Context, idToken, nonce string) (*biz.GoogleIdentity, error) {
	if idToken == "" || nonce == "" {
		return nil, biz.ErrGoogleTokenInvalid
	}
	var claims googleClaims
	_, err := jwt.ParseWithClaims(idToken, &claims,
		func(token *jwt.Token) (any, error) {
			keyID, _ := token.Header["kid"].(string)
			if keyID == "" {
				return nil, errors.New("token has no key id")
			}
			return g.keys.publicKey(ctx, keyID)
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithAudience(g.clientID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(tokenLeeway),
		jwt.WithTimeFunc(g.now),
	)
	if err != nil {
		return nil, biz.ErrGoogleTokenInvalid.WithCause(err)
	}
	if !g.issuers[claims.Issuer] || claims.Subject == "" ||
		subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonce)) != 1 {
		return nil, biz.ErrGoogleTokenInvalid
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	verified := bool(claims.EmailVerified) && email != ""
	return &biz.GoogleIdentity{
		Subject:       claims.Subject,
		Email:         email,
		EmailVerified: verified,
		// Google vouches for an address only on Gmail and Workspace (hd) accounts.
		EmailAuthoritative: verified && (strings.HasSuffix(email, "@gmail.com") || claims.HostedDomain != ""),
		Name:               strings.TrimSpace(claims.Name),
	}, nil
}
