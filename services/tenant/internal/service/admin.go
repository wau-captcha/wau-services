package service

import (
	"context"

	v1 "tenant/api/tenant/v1"
	"tenant/internal/biz"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AdminService implements the administrator API. The server lets only administrator
// tokens in.
type AdminService struct {
	v1.UnimplementedAdminServiceServer

	admin     *biz.AdminUsecase
	adminAuth *biz.AdminAuthUsecase
}

func NewAdminService(admin *biz.AdminUsecase, adminAuth *biz.AdminAuthUsecase) *AdminService {
	return &AdminService{admin: admin, adminAuth: adminAuth}
}

func (s *AdminService) GetAdmin(ctx context.Context, _ *v1.GetAdminRequest) (*v1.Admin, error) {
	id, err := currentAdmin(ctx)
	if err != nil {
		return nil, err
	}
	admin, err := s.adminAuth.Me(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertAdmin(admin), nil
}

func (s *AdminService) GetStats(ctx context.Context, _ *v1.GetStatsRequest) (*v1.AdminStats, error) {
	stats, err := s.admin.Stats(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.AdminStats{
		Users:            stats.Users,
		BannedUsers:      stats.BannedUsers,
		ApiKeys:          stats.ApiKeys,
		SuspendedApiKeys: stats.SuspendedApiKeys,
		ScrapingApiKeys:  stats.ScrapingApiKeys,
	}, nil
}

func (s *AdminService) ListUsers(ctx context.Context, req *v1.ListUsersRequest) (*v1.UserPage, error) {
	page, err := s.admin.ListUsers(ctx, req.GetQuery(), int(req.GetPage()), int(req.GetPageSize()))
	if err != nil {
		return nil, err
	}
	reply := &v1.UserPage{Users: make([]*v1.AdminUser, 0, len(page.Users)), TotalSize: int32(page.TotalSize)}
	for _, user := range page.Users {
		reply.Users = append(reply.Users, convertAdminUser(user))
	}
	return reply, nil
}

func (s *AdminService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.AdminUser, error) {
	id, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	user, err := s.admin.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertAdminUser(user), nil
}

func (s *AdminService) SetUserBanned(ctx context.Context, req *v1.SetUserBannedRequest) (*v1.AdminUser, error) {
	id, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	user, err := s.admin.SetUserBanned(ctx, id, req.GetBanned())
	if err != nil {
		return nil, err
	}
	return convertAdminUser(user), nil
}

func (s *AdminService) ListUserApiKeys(ctx context.Context, req *v1.ListUserApiKeysRequest) (*v1.AdminApiKeySet, error) {
	id, err := parseUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	keys, err := s.admin.ListApiKeys(ctx, id)
	if err != nil {
		return nil, err
	}
	reply := &v1.AdminApiKeySet{ApiKeys: make([]*v1.AdminApiKey, 0, len(keys))}
	for _, key := range keys {
		reply.ApiKeys = append(reply.ApiKeys, convertAdminApiKey(key))
	}
	return reply, nil
}

func (s *AdminService) SetApiKeySuspended(ctx context.Context, req *v1.SetApiKeySuspendedRequest) (*v1.AdminApiKey, error) {
	id, err := parseKeyID(req.GetApiKeyId())
	if err != nil {
		return nil, err
	}
	key, err := s.admin.SetApiKeySuspended(ctx, id, req.GetSuspended())
	if err != nil {
		return nil, err
	}
	return convertAdminApiKey(key), nil
}

func (s *AdminService) GetUsage(ctx context.Context, req *v1.AdminUsageRequest) (*v1.Usage, error) {
	var filter biz.UsageFilter
	if req.GetUserId() != "" {
		id, err := parseUserID(req.GetUserId())
		if err != nil {
			return nil, err
		}
		filter.UserID = uuid.NullUUID{UUID: id, Valid: true}
	}
	apiKeyID, err := parseOptionalKeyID(req.GetApiKeyId())
	if err != nil {
		return nil, err
	}
	filter.APIKeyID = apiKeyID
	usage, err := s.admin.Usage(ctx, filter, biz.UsageWindow{Days: int(req.GetDays()), AllTime: req.GetAllTime()})
	if err != nil {
		return nil, err
	}
	return convertUsage(usage), nil
}

// parseUserID keeps unparseable ids indistinguishable from missing accounts.
func parseUserID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, biz.ErrNoSuchUser
	}
	return parsed, nil
}

func convertAdminUser(user *biz.AdminUser) *v1.AdminUser {
	return &v1.AdminUser{
		Id:            user.ID.String(),
		Username:      user.Username,
		Email:         user.Email,
		TierId:        user.TierID,
		TierName:      user.TierName,
		CreateDate:    timestamppb.New(user.CreateDate),
		IsBanned:      user.IsBanned,
		ApiKeyCount:   int32(user.ApiKeyCount),
		QuotaUsed:     user.QuotaUsed,
		DailyQuota:    user.DailyQuota,
		TierExpiresAt: optionalTime(user.TierExpiresAt),
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
	}
}
