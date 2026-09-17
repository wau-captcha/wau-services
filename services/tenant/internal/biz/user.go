package biz

import (
	"context"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User is a dashboard account with the tier it belongs to.
type User struct {
	ID         uuid.UUID
	Username   string
	Email      string
	TierID     string
	TierName   string
	CreateDate time.Time
}

// Session is an issued access token together with its owner.
type Session struct {
	AccessToken string
	ExpiresIn   time.Duration
	User        *User
}

// UserRepo stores accounts.
type UserRepo interface {
	Create(ctx context.Context, username, email, passwordHash string) (uuid.UUID, error)
	// FindCredentials looks the account up by email when the identifier holds "@",
	// by username otherwise.
	FindCredentials(ctx context.Context, identifier string) (uuid.UUID, string, error)
	Profile(ctx context.Context, id uuid.UUID) (*User, error)
}

// PasswordHasher hashes passwords and checks them.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// TokenIssuer mints access tokens.
type TokenIssuer interface {
	Issue(userID uuid.UUID) (string, time.Duration, error)
}

// AuthUsecase registers users and signs them in.
type AuthUsecase struct {
	users  UserRepo
	hasher PasswordHasher
	tokens TokenIssuer
}

func NewAuthUsecase(users UserRepo, hasher PasswordHasher, tokens TokenIssuer) *AuthUsecase {
	return &AuthUsecase{users: users, hasher: hasher, tokens: tokens}
}

const (
	minPasswordLen = 8
	// bcrypt reads at most 72 bytes.
	maxPasswordLen = 72
	maxEmailLen    = 254
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

// dummyHash is compared against when an identifier is unknown, so a failed login
// costs the same work either way and does not reveal which accounts exist.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// Register creates an account and signs it in.
func (uc *AuthUsecase) Register(ctx context.Context, username, email, password string) (*Session, error) {
	username, err := normalizeUsername(username)
	if err != nil {
		return nil, err
	}
	email, err = normalizeEmail(email)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	id, err := uc.users.Create(ctx, username, email, hash)
	if err != nil {
		return nil, err
	}
	return uc.session(ctx, id)
}

// Login accepts a username or an email.
func (uc *AuthUsecase) Login(ctx context.Context, identifier, password string) (*Session, error) {
	identifier = strings.ToLower(strings.TrimSpace(identifier))
	if identifier == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	id, hash, err := uc.users.FindCredentials(ctx, identifier)
	if err != nil {
		uc.hasher.Compare(dummyHash, password)
		return nil, ErrInvalidCredentials
	}
	if !uc.hasher.Compare(hash, password) {
		return nil, ErrInvalidCredentials
	}
	return uc.session(ctx, id)
}

// Me returns the signed-in account.
func (uc *AuthUsecase) Me(ctx context.Context, userID uuid.UUID) (*User, error) {
	return uc.users.Profile(ctx, userID)
}

func (uc *AuthUsecase) session(ctx context.Context, id uuid.UUID) (*Session, error) {
	user, err := uc.users.Profile(ctx, id)
	if err != nil {
		return nil, err
	}
	token, ttl, err := uc.tokens.Issue(id)
	if err != nil {
		return nil, err
	}
	return &Session{AccessToken: token, ExpiresIn: ttl, User: user}, nil
}

func normalizeUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return "", InvalidField("username", "username must be 3-32 characters of a-z, 0-9 or _")
	}
	return username, nil
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	// Reject anything but a bare address, e.g. "Ann <a@b.com>".
	if err != nil || address.Address != email || len(email) > maxEmailLen {
		return "", InvalidField("email", "email is not a valid address")
	}
	return email, nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return InvalidField("password", "password must be 8-72 bytes")
	}
	return nil
}
