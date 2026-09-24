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

type CatalogRepo interface {
	ListChallengeTypes(ctx context.Context, userID uuid.UUID) ([]ChallengeType, error)
}

// CatalogUsecase lists what a user may use.
type CatalogUsecase struct {
	catalog CatalogRepo
}

func NewCatalogUsecase(catalog CatalogRepo) *CatalogUsecase { return &CatalogUsecase{catalog: catalog} }

func (uc *CatalogUsecase) ListChallengeTypes(ctx context.Context, userID uuid.UUID) ([]ChallengeType, error) {
	return uc.catalog.ListChallengeTypes(ctx, userID)
}
