package service

import (
	"context"
	"time"

	v1 "tenant/api/tenant/v1"
	"tenant/internal/biz"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthService implements the tenant auth API, for users and, apart, for administrators.
type AuthService struct {
	v1.UnimplementedAuthServiceServer

	userAuth  *biz.AuthUsecase
	adminAuth *biz.AdminAuthUsecase
}

func NewAuthService(userAuth *biz.AuthUsecase, adminAuth *biz.AdminAuthUsecase) *AuthService {
	return &AuthService{userAuth: userAuth, adminAuth: adminAuth}
}

func (s *AuthService) SignInWithGoogle(ctx context.Context, req *v1.SignInWithGoogleRequest) (*v1.AuthReply, error) {
	session, err := s.userAuth.SignInWithGoogle(ctx, req.GetIdToken(), req.GetNonce())
	if err != nil {
		return nil, err
	}
	return convertSession(session), nil
}

func (s *AuthService) GetMe(ctx context.Context, _ *v1.GetMeRequest) (*v1.User, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.userAuth.Me(ctx, userID)
	if err != nil {
		return nil, err
	}
	return convertUser(user), nil
}

func (s *AuthService) SignInAdminWithGoogle(ctx context.Context, req *v1.SignInWithGoogleRequest) (*v1.AdminAuthReply, error) {
	session, err := s.adminAuth.SignInWithGoogle(ctx, req.GetIdToken(), req.GetNonce())
	if err != nil {
		return nil, err
	}
	return &v1.AdminAuthReply{
		AccessToken: session.AccessToken,
		ExpiresIn:   int32(session.ExpiresIn.Seconds()),
		Admin:       convertAdmin(session.Admin),
	}, nil
}

func convertSession(session *biz.Session) *v1.AuthReply {
	return &v1.AuthReply{
		AccessToken: session.AccessToken,
		ExpiresIn:   int32(session.ExpiresIn.Seconds()),
		User:        convertUser(session.User),
	}
}

func convertUser(user *biz.User) *v1.User {
	return &v1.User{
		Id:            user.ID.String(),
		Username:      user.Username,
		Email:         user.Email,
		TierId:        user.TierID,
		TierName:      user.TierName,
		CreateDate:    timestamppb.New(user.CreateDate),
		TierExpiresAt: optionalTime(user.TierExpiresAt),
	}
}

// optionalTime leaves a zero time out of the reply.
func optionalTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func convertAdmin(admin *biz.Admin) *v1.Admin {
	return &v1.Admin{
		Id:         admin.ID.String(),
		Username:   admin.Username,
		Email:      admin.Email,
		CreateDate: timestamppb.New(admin.CreateDate),
	}
}
