package data

import (
	"context"

	"tenant/internal/biz"
	"tenant/internal/data/db"

	"github.com/google/uuid"
)

func (r *apiKeyRepo) AllowedDomains(ctx context.Context, userID, id uuid.UUID) ([]biz.AllowedDomain, error) {
	rows, err := r.data.queries.ListAllowedDomains(ctx, db.ListAllowedDomainsParams{UserID: userID, APIKeyID: id})
	if err != nil {
		return nil, err
	}
	domains := make([]biz.AllowedDomain, 0, len(rows))
	for _, row := range rows {
		domains = append(domains, biz.AllowedDomain{Domain: row.Domain, CreateDate: row.CreateDate.Time})
	}
	return domains, nil
}

func (r *apiKeyRepo) SetAllowedDomains(ctx context.Context, userID, id uuid.UUID, domains []string) ([]biz.AllowedDomain, error) {
	err := r.data.queries.ReplaceAllowedDomains(ctx, db.ReplaceAllowedDomainsParams{APIKeyID: id, UserID: userID, Domains: domains})
	if err != nil {
		return nil, err
	}
	return r.AllowedDomains(ctx, userID, id)
}
