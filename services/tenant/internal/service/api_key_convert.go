package service

import (
	v1 "tenant/api/tenant/v1"
	"tenant/internal/biz"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func convertApiKey(key *biz.ApiKey) *v1.ApiKey {
	converted := &v1.ApiKey{
		Id:                key.ID.String(),
		Name:              key.Name,
		SiteKey:           key.SiteKey,
		SecretKey:         key.SecretKey,
		IsActive:          key.IsActive,
		CreateDate:        timestamppb.New(key.CreateDate),
		IsSuspended:       key.IsSuspended,
		SuspendReason:     key.SuspendReason,
		AllowedOrigins:    convertAllowedOrigins(key.AllowedOrigins),
		AllowedDomains:    convertAllowedDomains(key.AllowedDomains),
		RequireBehavior:   key.RequireBehavior,
		ScrapingFlagged:   key.ScrapingFlagged,
		ScrapingFlaggedAt: optionalTime(key.ScrapingFlaggedAt),
	}
	if key.PreviousSecretKey != "" {
		converted.PreviousSecretKey = key.PreviousSecretKey
		converted.PreviousSecretKeyExpiresAt = timestamppb.New(key.PreviousSecretExpiresAt)
	}
	if today := key.Today; today != nil {
		converted.Today = &v1.KeyCounters{Challenges: today.Challenges, Answers: today.Verifications, Solved: today.Solved, Automated: today.Automated}
	}
	if key.LatestAppeal != nil {
		converted.LatestAppeal = convertAppeal(key.LatestAppeal)
	}
	return converted
}

func convertAllowedOrigins(origins []biz.AllowedOrigin) []*v1.AllowedOrigin {
	converted := make([]*v1.AllowedOrigin, 0, len(origins))
	for _, origin := range origins {
		converted = append(converted, &v1.AllowedOrigin{
			IpAddress:   origin.IPAddress,
			Description: origin.Description,
			CreateDate:  timestamppb.New(origin.CreateDate),
		})
	}
	return converted
}

func convertAllowedDomains(domains []biz.AllowedDomain) []*v1.AllowedDomain {
	converted := make([]*v1.AllowedDomain, 0, len(domains))
	for _, domain := range domains {
		converted = append(converted, &v1.AllowedDomain{Domain: domain.Domain, CreateDate: timestamppb.New(domain.CreateDate)})
	}
	return converted
}

func convertAppeal(appeal *biz.Appeal) *v1.ApiKeyAppeal {
	return &v1.ApiKeyAppeal{
		Id:         appeal.ID.String(),
		Status:     appeal.Status,
		Message:    appeal.Message,
		AdminNote:  appeal.AdminNote,
		CreateDate: timestamppb.New(appeal.CreateDate),
		DecidedAt:  optionalTime(appeal.DecidedAt),
	}
}

// convertEvent fails only for a detail that is not JSON, which the database does not hold.
func convertEvent(event biz.ApiKeyEvent) (*v1.ApiKeyEvent, error) {
	detail, err := structpb.NewStruct(event.Detail)
	if err != nil {
		return nil, err
	}
	return &v1.ApiKeyEvent{
		Id:         event.ID,
		Actor:      event.Actor,
		Action:     event.Action,
		Detail:     detail,
		CreateDate: timestamppb.New(event.CreateDate),
	}, nil
}
