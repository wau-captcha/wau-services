// Package auth checks the user access tokens wau-tenant issues.
package auth

import (
	"fmt"

	"billing/internal/biz"
	"billing/internal/conf"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// userIssuer is the "iss" of wau-tenant user tokens; its administrator tokens use another
	// issuer and are refused here.
	userIssuer   = "wau-tenant"
	minSecretLen = 32
)

// Tokens verifies HS256 user tokens with the secret wau-tenant signs them with.
type Tokens struct {
	secret []byte
}

func NewTokens(authConf *conf.Auth) (*Tokens, error) {
	if len(authConf.GetSecret()) < minSecretLen {
		return nil, fmt.Errorf("auth secret must be at least %d bytes", minSecretLen)
	}
	return &Tokens{secret: []byte(authConf.GetSecret())}, nil
}

// Parse returns the user behind a token. Every failure is the same error, so the response never
// says which part of the token was wrong.
func (t *Tokens) Parse(token string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(userIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return uuid.Nil, biz.ErrUnauthenticated
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, biz.ErrUnauthenticated
	}
	return id, nil
}
