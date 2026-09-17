package data

import (
	"context"
	"errors"
	"strings"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is the PostgreSQL SQLSTATE for a duplicate key.
const uniqueViolation = "23505"

type userRepo struct {
	data *Data
}

func NewUserRepo(data *Data) biz.UserRepo { return &userRepo{data: data} }

func (r *userRepo) Create(ctx context.Context, username, email, passwordHash string) (uuid.UUID, error) {
	id, err := r.data.queries.CreateUser(ctx, db.CreateUserParams{
		Username: username,
		Email:    email,
		Password: passwordHash,
	})
	if err != nil {
		return uuid.Nil, takenError(err)
	}
	return id, nil
}

func (r *userRepo) FindCredentials(ctx context.Context, identifier string) (uuid.UUID, string, error) {
	if strings.Contains(identifier, "@") {
		row, err := r.data.queries.GetUserCredentialsByEmail(ctx, identifier)
		if err != nil {
			return uuid.Nil, "", credentialsError(err)
		}
		return row.UserID, row.Password, nil
	}
	row, err := r.data.queries.GetUserCredentialsByUsername(ctx, identifier)
	if err != nil {
		return uuid.Nil, "", credentialsError(err)
	}
	return row.UserID, row.Password, nil
}

func (r *userRepo) Profile(ctx context.Context, id uuid.UUID) (*biz.User, error) {
	row, err := r.data.queries.GetUserProfile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		// The token names a user that no longer exists.
		return nil, biz.ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	return &biz.User{
		ID:         row.UserID,
		Username:   row.Username,
		Email:      row.Email,
		TierID:     row.TierID,
		TierName:   row.TierName,
		CreateDate: row.CreateDate.Time,
	}, nil
}

// takenError maps the unique indexes of users onto domain errors.
func takenError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch pgErr.ConstraintName {
		case "users_username_key":
			return biz.ErrUsernameTaken
		case "users_email_key":
			return biz.ErrEmailTaken
		}
	}
	return err
}

func credentialsError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.ErrInvalidCredentials
	}
	return err
}
