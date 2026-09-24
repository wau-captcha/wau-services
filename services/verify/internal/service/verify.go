package service

import (
	"context"
	"strings"

	"verify/internal/biz"

	v1 "common/proto/verify/v1"

	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// VerifyService implements the verify API.
type VerifyService struct {
	v1.UnimplementedVerifyServiceServer

	verifier *biz.VerifyUsecase
}

// NewVerifyService creates a VerifyService.
func NewVerifyService(verifier *biz.VerifyUsecase) *VerifyService {
	return &VerifyService{verifier: verifier}
}

// VerifyChallenge checks an answer and issues a one-time token on success.
func (s *VerifyService) VerifyChallenge(ctx context.Context, req *v1.VerifyChallengeRequest) (*v1.VerifyChallengeReply, error) {
	token, err := s.verifier.VerifyChallenge(ctx, req.GetChallengeId(), req.GetAnswer())
	if err != nil {
		return nil, err
	}
	if token == nil {
		return &v1.VerifyChallengeReply{}, nil
	}
	return &v1.VerifyChallengeReply{Success: true, Token: token.Value, ExpiresIn: int32(token.TTL.Seconds())}, nil
}

// SiteVerify consumes a token for the site identified by the bearer secret key.
func (s *VerifyService) SiteVerify(ctx context.Context, req *v1.SiteVerifyRequest) (*v1.SiteVerifyReply, error) {
	g, err := s.verifier.SiteVerify(ctx, bearerToken(ctx), req.GetToken())
	if err != nil {
		return nil, err
	}
	if g == nil {
		return &v1.SiteVerifyReply{}, nil
	}
	return &v1.SiteVerifyReply{
		Success:       true,
		ChallengeType: g.ChallengeType,
		ChallengeTs:   timestamppb.New(g.SolvedAt),
	}, nil
}

// bearerToken extracts the credential of an "Authorization: Bearer" header.
func bearerToken(ctx context.Context) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	scheme, credential, _ := strings.Cut(tr.RequestHeader().Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return credential
}
