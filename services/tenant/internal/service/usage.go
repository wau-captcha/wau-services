package service

import (
	"context"

	"tenant/internal/biz"

	v1 "common/proto/tenant/v1"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UsageService implements the tenant usage API.
type UsageService struct {
	v1.UnimplementedUsageServiceServer

	usage *biz.UsageUsecase
	quota *biz.QuotaUsecase
}

func NewUsageService(usage *biz.UsageUsecase, quota *biz.QuotaUsecase) *UsageService {
	return &UsageService{usage: usage, quota: quota}
}

func (s *UsageService) GetUsage(ctx context.Context, req *v1.GetUsageRequest) (*v1.Usage, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	apiKeyID, err := parseOptionalKeyID(req.GetApiKeyId())
	if err != nil {
		return nil, err
	}
	usage, err := s.usage.Get(ctx, userID, apiKeyID, int(req.GetDays()))
	if err != nil {
		return nil, err
	}
	return convertUsage(usage), nil
}

func (s *UsageService) GetQuota(ctx context.Context, _ *v1.GetQuotaRequest) (*v1.Quota, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	quota, err := s.quota.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &v1.Quota{
		Used:     quota.Used,
		Limit:    quota.Limit,
		Date:     quota.Date,
		ResetsAt: timestamppb.New(quota.ResetsAt),
	}, nil
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
		Total:           usage.Total,
		Success:         usage.Success,
		Days:            convertDays(usage.Days),
		ChallengeTypes:  make([]*v1.ChallengeTypeUsage, 0, len(usage.ChallengeTypes)),
		ByApiKey:        convertSeries(usage.ByApiKey),
		ByChallengeType: convertSeries(usage.ByChallengeType),
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

func convertDays(days []biz.DailyUsage) []*v1.DailyUsage {
	reply := make([]*v1.DailyUsage, 0, len(days))
	for _, day := range days {
		reply = append(reply, &v1.DailyUsage{Date: day.Date, Total: day.Total, Success: day.Success})
	}
	return reply
}

func convertSeries(all []biz.UsageSeries) []*v1.UsageSeries {
	reply := make([]*v1.UsageSeries, 0, len(all))
	for _, series := range all {
		reply = append(reply, &v1.UsageSeries{
			Id:      series.ID,
			Total:   series.Total,
			Success: series.Success,
			Days:    convertDays(series.Days),
		})
	}
	return reply
}
