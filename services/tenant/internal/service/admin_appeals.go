package service

import (
	"context"

	v1 "tenant/api/tenant/v1"
	"tenant/internal/biz"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *AdminService) ListAppeals(ctx context.Context, req *v1.ListAppealsRequest) (*v1.AdminAppealSet, error) {
	appeals, err := s.admin.ListAppeals(ctx, req.GetStatus())
	if err != nil {
		return nil, err
	}
	reply := &v1.AdminAppealSet{Appeals: make([]*v1.AdminAppeal, 0, len(appeals))}
	for _, appeal := range appeals {
		reply.Appeals = append(reply.Appeals, convertAdminAppeal(appeal))
	}
	return reply, nil
}

func (s *AdminService) DecideAppeal(ctx context.Context, req *v1.DecideAppealRequest) (*v1.AdminAppeal, error) {
	adminID, err := currentAdmin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(req.GetAppealId())
	if err != nil {
		return nil, biz.ErrAppealNotFound
	}
	appeal, err := s.admin.DecideAppeal(ctx, adminID, id, req.GetApprove(), req.GetNote())
	if err != nil {
		return nil, err
	}
	return convertAdminAppeal(appeal), nil
}

func convertAdminAppeal(appeal *biz.AdminAppeal) *v1.AdminAppeal {
	return &v1.AdminAppeal{
		Id:         appeal.ID.String(),
		Status:     appeal.Status,
		Message:    appeal.Message,
		AdminNote:  appeal.AdminNote,
		CreateDate: timestamppb.New(appeal.CreateDate),
		DecidedAt:  optionalTime(appeal.DecidedAt),
		ApiKey:     convertAdminApiKey(appeal.ApiKey),
		UserId:     appeal.UserID.String(),
		UserEmail:  appeal.UserEmail,
		UserName:   appeal.UserName,
	}
}

func convertAdminApiKey(key *biz.AdminApiKey) *v1.AdminApiKey {
	return &v1.AdminApiKey{
		Id:                 key.ID.String(),
		Name:               key.Name,
		SiteKey:            key.SiteKey,
		IsActive:           key.IsActive,
		IsSuspended:        key.IsSuspended,
		SuspendReason:      key.SuspendReason,
		CreateDate:         timestamppb.New(key.CreateDate),
		ChallengesToday:    key.Challenges,
		VerificationsToday: key.Verifications,
		ScrapingFlagged:    key.ScrapingFlagged,
		SolvedToday:        key.Solved,
		AutomatedToday:     key.Automated,
	}
}
