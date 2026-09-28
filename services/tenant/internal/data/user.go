package data

import (
	"context"
	"errors"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// uniqueViolation is the PostgreSQL SQLSTATE for a duplicate key.
	uniqueViolation = "23505"

	usersEmailKey     = "users_email_key"
	usersGoogleSubKey = "users_google_sub_key"
)

type userRepo struct {
	data *Data
}

func NewUserRepo(data *Data) biz.UserRepo { return &userRepo{data: data} }

func (r *userRepo) FindByGoogleSub(ctx context.Context, sub string) (uuid.UUID, error) {
	id, err := r.data.queries.FindUserByGoogleSub(ctx, googleSub(sub))
	return id, notFound(err)
}

func (r *userRepo) LinkGoogleAccount(ctx context.Context, email, sub, name string) (uuid.UUID, error) {
	id, err := r.data.queries.LinkGoogleAccount(ctx, db.LinkGoogleAccountParams{
		GoogleSub: googleSub(sub),
		Username:  name,
		Email:     email,
	})
	return id, uniqueError(notFound(err))
}

func (r *userRepo) CreateGoogleUser(ctx context.Context, sub, email, name string) (uuid.UUID, error) {
	id, err := r.data.queries.CreateGoogleUser(ctx, db.CreateGoogleUserParams{
		GoogleSub: googleSub(sub),
		Email:     email,
		Username:  name,
	})
	return id, uniqueError(err)
}

func (r *userRepo) UpdateGoogleProfile(ctx context.Context, id uuid.UUID, email, name string) error {
	params := db.UpdateGoogleProfileParams{UserID: id, Email: email, Username: name}
	err := r.data.queries.UpdateGoogleProfile(ctx, params)
	if errors.Is(uniqueError(err), biz.ErrGoogleAccountConflict) {
		// Another account took the address in the meantime; keep the stored one.
		params.Email = ""
		err = r.data.queries.UpdateGoogleProfile(ctx, params)
	}
	return err
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
		DailyQuota: int64(row.DailyQuota),
		IsBanned:   row.IsBanned,
		// NULL scans as the zero time: a tier that never ends.
		TierExpiresAt: row.TierExpiresAt.Time,
	}, nil
}

func (r *userRepo) Status(ctx context.Context, id uuid.UUID) (biz.AccountStatus, error) {
	banned, err := r.data.queries.GetAccountStatus(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.AccountStatus{}, nil
	}
	if err != nil {
		return biz.AccountStatus{}, err
	}
	return biz.AccountStatus{Exists: true, Banned: banned}, nil
}

func googleSub(sub string) pgtype.Text { return pgtype.Text{String: sub, Valid: true} }

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return biz.ErrUserNotFound
	}
	return err
}

// uniqueError maps the unique indexes of users onto domain errors.
func uniqueError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch pgErr.ConstraintName {
		case usersEmailKey:
			return biz.ErrGoogleAccountConflict
		case usersGoogleSubKey:
			return biz.ErrGoogleSubTaken
		}
	}
	return err
}
