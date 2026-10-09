package data

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"challenge/internal/biz"
	"challenge/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
		APIKeyID:        row.APIKeyID.String(),
		UserID:          row.UserID.String(),
		ChallengeTypes:  row.ChallengeTypes,
		Suspended:       row.IsSuspended,
		OwnerBanned:     row.IsBanned,
		AllowedOrigins:  parsePrefixes(row.AllowedOrigins),
		AllowedDomains:  row.AllowedDomains,
		RequireBehavior: row.RequireBehavior,
		// NULL scans as the zero time: never flagged.
		FlaggedAt:  row.ScrapingFlaggedAt.Time,
		DailyQuota: int64(row.DailyQuota),
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

func (r *siteRepo) SuspendForScraping(ctx context.Context, apiKeyID string) (bool, error) {
	id, err := uuid.Parse(apiKeyID)
	if err != nil {
		return false, err
	}
	suspended, err := r.data.queries.SuspendAPIKeyForScraping(ctx, id)
	return suspended > 0, err
}

func (r *siteRepo) FlagForScraping(ctx context.Context, apiKeyID string, dayStart time.Time) (bool, error) {
	id, err := uuid.Parse(apiKeyID)
	if err != nil {
		return false, err
	}
	flagged, err := r.data.queries.FlagAPIKeyForScraping(ctx, db.FlagAPIKeyForScrapingParams{
		APIKeyID: id,
		DayStart: pgtype.Timestamptz{Time: dayStart, Valid: true},
	})
	return flagged > 0, err
}

func (r *siteRepo) RecordEvent(ctx context.Context, apiKeyID, action string, detail map[string]any) error {
	id, err := uuid.Parse(apiKeyID)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	return r.data.queries.CreateSystemAPIKeyEvent(ctx, db.CreateSystemAPIKeyEventParams{APIKeyID: id, Action: action, Detail: encoded})
}
