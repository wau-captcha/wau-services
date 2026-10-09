package biz

import (
	"context"

	"github.com/google/uuid"
)

// ChallengeType is a captcha kind the tier of a user may request.
type ChallengeType struct {
	ID   string
	Code string
}

// PublicPlan is a tier as the public pricing page shows it.
type PublicPlan struct {
	ID          string
	Name        string
	Description string
	// VND; 0 when not sold that way.
	MonthlyPrice, YearlyPrice int64
	DailyQuota                int64
	ChallengeTypes            []string
}

type CatalogRepo interface {
	ListChallengeTypes(ctx context.Context, userID uuid.UUID) ([]ChallengeType, error)
	// ListPublicPlans returns the active tiers, cheapest first.
	ListPublicPlans(ctx context.Context) ([]PublicPlan, error)
}

// CatalogUsecase lists what a user may use.
type CatalogUsecase struct {
	catalog CatalogRepo
}

func NewCatalogUsecase(catalog CatalogRepo) *CatalogUsecase { return &CatalogUsecase{catalog: catalog} }

func (uc *CatalogUsecase) ListChallengeTypes(ctx context.Context, userID uuid.UUID) ([]ChallengeType, error) {
	return uc.catalog.ListChallengeTypes(ctx, userID)
}

// ListPublicPlans is what anyone may see of the plans: no account is involved.
func (uc *CatalogUsecase) ListPublicPlans(ctx context.Context) ([]PublicPlan, error) {
	return uc.catalog.ListPublicPlans(ctx)
}
