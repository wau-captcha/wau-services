package service

import (
	"context"

	v1 "tenant/api/tenant/v1"
	"tenant/internal/biz"
)

// CatalogService implements the tenant catalog API.
type CatalogService struct {
	v1.UnimplementedCatalogServiceServer

	catalog *biz.CatalogUsecase
}

func NewCatalogService(catalog *biz.CatalogUsecase) *CatalogService {
	return &CatalogService{catalog: catalog}
}

func (s *CatalogService) ListChallengeTypes(ctx context.Context, _ *v1.ListChallengeTypesRequest) (*v1.ChallengeTypeSet, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	types, err := s.catalog.ListChallengeTypes(ctx, userID)
	if err != nil {
		return nil, err
	}
	typeSet := &v1.ChallengeTypeSet{ChallengeTypes: make([]*v1.ChallengeType, 0, len(types))}
	for _, challengeType := range types {
		typeSet.ChallengeTypes = append(typeSet.ChallengeTypes, &v1.ChallengeType{Id: challengeType.ID, Code: challengeType.Code})
	}
	return typeSet, nil
}

// ListPublicPlans needs no token: the pricing page shows the plans to anyone.
func (s *CatalogService) ListPublicPlans(ctx context.Context, _ *v1.ListPublicPlansRequest) (*v1.PublicPlanSet, error) {
	plans, err := s.catalog.ListPublicPlans(ctx)
	if err != nil {
		return nil, err
	}
	reply := &v1.PublicPlanSet{Plans: make([]*v1.PublicPlan, 0, len(plans))}
	for _, plan := range plans {
		reply.Plans = append(reply.Plans, &v1.PublicPlan{
			Id:             plan.ID,
			Name:           plan.Name,
			Description:    plan.Description,
			MonthlyPrice:   plan.MonthlyPrice,
			YearlyPrice:    plan.YearlyPrice,
			DailyQuota:     plan.DailyQuota,
			ChallengeTypes: plan.ChallengeTypes,
		})
	}
	return reply, nil
}
