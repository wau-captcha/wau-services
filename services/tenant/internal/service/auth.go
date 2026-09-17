package service

import (
	"context"

	"tenant/internal/biz"

	v1 "common/proto/tenant/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthService implements the tenant auth API.
type AuthService struct {
	v1.UnimplementedAuthServiceServer

	uc *biz.AuthUsecase
}

func NewAuthService(uc *biz.AuthUsecase) *AuthService { return &AuthService{uc: uc} }

func (s *AuthService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.AuthReply, error) {
	session, err := s.uc.Register(ctx, req.GetUsername(), req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, err
	}
	return convertSession(session), nil
}

func (s *AuthService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.AuthReply, error) {
	session, err := s.uc.Login(ctx, req.GetIdentifier(), req.GetPassword())
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
	user, err := s.uc.Me(ctx, userID)
	if err != nil {
		return nil, err
	}
	return convertUser(user), nil
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
		Id:         user.ID.String(),
		Username:   user.Username,
		Email:      user.Email,
		TierId:     user.TierID,
		TierName:   user.TierName,
		CreateDate: timestamppb.New(user.CreateDate),
	}
}
