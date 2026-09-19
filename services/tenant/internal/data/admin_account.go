package data

import (
	"context"
	"errors"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type adminAccountRepo struct {
	data *Data
}

func NewAdminAccountRepo(data *Data) biz.AdminAccountRepo { return &adminAccountRepo{data: data} }

func (r *adminAccountRepo) FindByEmail(ctx context.Context, email string) (*biz.Admin, error) {
	row, err := r.data.queries.FindAdminByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrNotAdmin
	}
	if err != nil {
		return nil, err
	}
	return toAdmin(row), nil
}

func (r *adminAccountRepo) Get(ctx context.Context, id uuid.UUID) (*biz.Admin, error) {
	row, err := r.data.queries.GetAdmin(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		// The token names an administrator that was removed.
		return nil, biz.ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	return toAdmin(row), nil
}

func toAdmin(row db.Admin) *biz.Admin {
	return &biz.Admin{
		ID:         row.AdminID,
		Username:   row.Username,
		Email:      row.Email,
		CreateDate: row.CreateDate.Time,
	}
}
