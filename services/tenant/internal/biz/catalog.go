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
	repo CatalogRepo
}

func NewCatalogUsecase(repo CatalogRepo) *CatalogUsecase { return &CatalogUsecase{repo: repo} }

func (uc *CatalogUsecase) ListChallengeTypes(ctx context.Context, userID uuid.UUID) ([]ChallengeType, error) {
	return uc.repo.ListChallengeTypes(ctx, userID)
}
