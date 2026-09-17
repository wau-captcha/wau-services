// Package auth issues and checks the access tokens of the tenant API.
package auth

import (
	"fmt"
	"time"

	"tenant/internal/biz"
	"tenant/internal/conf"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	issuer       = "wau-tenant"
	minSecretLen = 32
)

// JWT signs and verifies HS256 access tokens.
type JWT struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewJWT fails fast on a weak configuration: a short signing key or a
// non-positive lifetime would make every issued token unsafe.
func NewJWT(c *conf.Auth) (*JWT, error) {
	if len(c.GetSecret()) < minSecretLen {
		return nil, fmt.Errorf("auth secret must be at least %d bytes", minSecretLen)
	}
	if c.GetTtl().AsDuration() <= 0 {
		return nil, fmt.Errorf("auth ttl must be positive")
	}
	return &JWT{secret: []byte(c.GetSecret()), ttl: c.GetTtl().AsDuration(), now: time.Now}, nil
}

// Issue returns a token for userID and how long it stays valid.
func (j *JWT) Issue(userID uuid.UUID) (string, time.Duration, error) {
	now := j.now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
	}).SignedString(j.secret)
	if err != nil {
		return "", 0, err
	}
	return token, j.ttl, nil
}

// Parse returns the user behind a token. Every failure maps to the same error so
// the response never says which part of the token was wrong.
func (j *JWT) Parse(token string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(j.now),
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
