package data

import (
	"context"
	"errors"
	"net/netip"

	"challenge/internal/biz"

	"github.com/google/uuid"
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
		UserID:         row.UserID.String(),
		ChallengeTypes: row.ChallengeTypes,
		Suspended:      row.IsSuspended,
		OwnerBanned:    row.IsBanned,
		AllowedOrigins: parsePrefixes(row.AllowedOrigins),
		DailyQuota:     int64(row.DailyQuota),
	}, nil
}

// parsePrefixes turns the inet values of api_key_allowed_origins into networks; a plain
// address covers only itself. Unreadable values are dropped rather than opening the key.
func parsePrefixes(values []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		if prefix, err := netip.ParsePrefix(value); err == nil {
			prefixes = append(prefixes, prefix)
			continue
		}
		if addr, err := netip.ParseAddr(value); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	return prefixes
}

func (r *siteRepo) SuspendForScraping(ctx context.Context, apiKeyID string) error {
	id, err := uuid.Parse(apiKeyID)
	if err != nil {
		return err
	}
	return r.data.queries.SuspendAPIKeyForScraping(ctx, id)
}
