package service

import (
	"context"
	"net/netip"

	"challenge/internal/biz"
	"challenge/internal/conf"

	"common/clientip"
	"common/envconf"
	v1 "common/proto/challenge/v1"

	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

// ChallengeService implements the challenge API.
type ChallengeService struct {
	v1.UnimplementedChallengeServiceServer

	uc *biz.ChallengeUsecase
	// Proxies whose X-Forwarded-For names the real caller, e.g. the gateway.
	trusted []netip.Prefix
}

// NewChallengeService creates a ChallengeService.
func NewChallengeService(uc *biz.ChallengeUsecase, c *conf.Server) (*ChallengeService, error) {
	trusted, err := clientip.ParsePrefixes(envconf.List(c.GetTrustedProxies()))
	if err != nil {
		return nil, err
	}
	return &ChallengeService{uc: uc, trusted: trusted}, nil
}

// NewChallenge issues a challenge for a site key, for an address the key allows.
func (s *ChallengeService) NewChallenge(ctx context.Context, req *v1.NewChallengeRequest) (*v1.Challenge, error) {
	c, err := s.uc.New(ctx, req.GetSiteKey(), req.GetChallengeType(), s.clientAddr(ctx))
	if err != nil {
		return nil, err
	}
	return &v1.Challenge{
		ChallengeId:   c.ID,
		ChallengeType: c.Type,
		Image:         c.Media.Data,
		MimeType:      c.Media.MIMEType,
		Width:         int32(c.Media.Width),
		Height:        int32(c.Media.Height),
		ExpiresIn:     int32(c.TTL.Seconds()),
	}, nil
}

// clientAddr is the address the request comes from, read through the proxies we trust.
// The zero value means it could not be read, e.g. on a gRPC call.
func (s *ChallengeService) clientAddr(ctx context.Context) netip.Addr {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return netip.Addr{}
	}
	request, ok := tr.(*khttp.Transport)
	if !ok {
		return netip.Addr{}
	}
	return clientip.From(request.Request(), s.trusted)
}

// CheckAnswer consumes a challenge and reports whether the answer is correct.
func (s *ChallengeService) CheckAnswer(ctx context.Context, req *v1.CheckAnswerRequest) (*v1.CheckAnswerReply, error) {
	c, ok, err := s.uc.Check(ctx, req.GetChallengeId(), req.GetAnswer())
	if err != nil {
		return nil, err
	}
	return &v1.CheckAnswerReply{Success: ok, ApiKeyId: c.APIKeyID, ChallengeType: c.Type, UserId: c.UserID}, nil
}
