package service

import (
	"context"

	v1 "tenant/api/tenant/v1"
	"tenant/internal/biz"

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
	usage, err := s.usage.Get(ctx, userID, apiKeyID, biz.UsageWindow{Days: int(req.GetDays()), AllTime: req.GetAllTime()})
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
	return &v1.Usage{
		Challenges:      usage.Challenges,
		Verifications:   usage.Verifications,
		Successes:       usage.Successes,
		Days:            convertDays(usage.Days),
		ByApiKey:        convertSeries(usage.ByApiKey),
		ByChallengeType: convertSeries(usage.ByChallengeType),
	}
}

func convertDays(days []biz.DailyUsage) []*v1.DailyUsage {
	reply := make([]*v1.DailyUsage, 0, len(days))
	for _, day := range days {
		reply = append(reply, &v1.DailyUsage{
			Date:          day.Date,
			Challenges:    day.Challenges,
			Verifications: day.Verifications,
			Successes:     day.Successes,
		})
	}
	return reply
}

func convertSeries(all []biz.UsageSeries) []*v1.UsageSeries {
	reply := make([]*v1.UsageSeries, 0, len(all))
	for _, series := range all {
		converted := &v1.UsageSeries{
			Id:            series.ID,
			Challenges:    series.Challenges,
			Verifications: series.Verifications,
			Successes:     series.Successes,
			Days:          convertDays(series.Days),
			Name:          series.Key.Name,
			Deleted:       series.Key.Deleted,
		}
		if !series.Key.CreateDate.IsZero() {
			converted.CreateDate = timestamppb.New(series.Key.CreateDate)
		}
		reply = append(reply, converted)
	}
	return reply
}
