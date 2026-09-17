package service

import (
	"context"

	"tenant/internal/biz"

	v1 "common/proto/tenant/v1"
)

// CatalogService implements the tenant catalog API.
type CatalogService struct {
	v1.UnimplementedCatalogServiceServer

	uc *biz.CatalogUsecase
}

func NewCatalogService(uc *biz.CatalogUsecase) *CatalogService { return &CatalogService{uc: uc} }

func (s *CatalogService) ListChallengeTypes(ctx context.Context, _ *v1.ListChallengeTypesRequest) (*v1.ChallengeTypeSet, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	types, err := s.uc.ListChallengeTypes(ctx, userID)
	if err != nil {
		return nil, err
	}
	set := &v1.ChallengeTypeSet{ChallengeTypes: make([]*v1.ChallengeType, 0, len(types))}
	for _, challengeType := range types {
		set.ChallengeTypes = append(set.ChallengeTypes, &v1.ChallengeType{Id: challengeType.ID, Code: challengeType.Code})
	}
	return set, nil
}
