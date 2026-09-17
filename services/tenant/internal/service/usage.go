package service

import (
	"context"

	"tenant/internal/biz"

	v1 "common/proto/tenant/v1"

	"github.com/google/uuid"
)

// UsageService implements the tenant usage API.
type UsageService struct {
	v1.UnimplementedUsageServiceServer

	uc *biz.UsageUsecase
}

func NewUsageService(uc *biz.UsageUsecase) *UsageService { return &UsageService{uc: uc} }

func (s *UsageService) GetUsage(ctx context.Context, req *v1.GetUsageRequest) (*v1.Usage, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	apiKeyID, err := parseOptionalKeyID(req.GetApiKeyId())
	if err != nil {
		return nil, err
	}
	usage, err := s.uc.Get(ctx, userID, apiKeyID, int(req.GetDays()))
	if err != nil {
		return nil, err
	}
	return convertUsage(usage), nil
}

// parseOptionalKeyID treats an empty id as "every key of the user".
func parseOptionalKeyID(rawID string) (uuid.NullUUID, error) {
	if rawID == "" {
		return uuid.NullUUID{}, nil
	}
	id, err := parseKeyID(rawID)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: id, Valid: true}, nil
}

func convertUsage(usage *biz.Usage) *v1.Usage {
	reply := &v1.Usage{
		Total:          usage.Total,
		Success:        usage.Success,
		Days:           make([]*v1.DailyUsage, 0, len(usage.Days)),
		ChallengeTypes: make([]*v1.ChallengeTypeUsage, 0, len(usage.ChallengeTypes)),
	}
	for _, day := range usage.Days {
		reply.Days = append(reply.Days, &v1.DailyUsage{Date: day.Date, Total: day.Total, Success: day.Success})
	}
	for _, byType := range usage.ChallengeTypes {
		reply.ChallengeTypes = append(reply.ChallengeTypes, &v1.ChallengeTypeUsage{
			ChallengeType: byType.ChallengeType,
			Total:         byType.Total,
			Success:       byType.Success,
		})
	}
	return reply
}
