package data

import (
	"context"
	"errors"

	"challenge/internal/biz"

	"github.com/jackc/pgx/v5"
)

type siteRepo struct {
	data *Data
}

func NewSiteRepo(data *Data) biz.SiteRepo {
	return &siteRepo{data: data}
}

func (r *siteRepo) FindBySiteKey(ctx context.Context, siteKey string) (*biz.Site, error) {
	row, err := r.data.queries.GetSiteBySiteKey(ctx, siteKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrSiteKeyInvalid
	}
	if err != nil {
		return nil, err
	}
	return &biz.Site{
		APIKeyID:       row.APIKeyID.String(),
		ChallengeTypes: row.ChallengeTypes,
	}, nil
}
