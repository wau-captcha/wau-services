package service

import (
	"context"
	"net/netip"

	v1 "challenge/api/challenge/v1"
	"challenge/internal/biz"
	"challenge/internal/conf"
	"challenge/internal/pkg/clientip"
	"challenge/internal/pkg/envconf"

	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/proto"
)

// ChallengeService implements the challenge API.
type ChallengeService struct {
	v1.UnimplementedChallengeServiceServer

	challenges *biz.ChallengeUsecase
	// Proxies whose X-Forwarded-For names the real caller, e.g. the load balancer.
	trusted []netip.Prefix
}

// NewChallengeService creates a ChallengeService.
func NewChallengeService(challenges *biz.ChallengeUsecase, serverConf *conf.Server) (*ChallengeService, error) {
	trusted, err := clientip.ParsePrefixes(envconf.SplitList(serverConf.GetTrustedProxies()))
	if err != nil {
		return nil, err
	}
	return &ChallengeService{challenges: challenges, trusted: trusted}, nil
}

// NewChallenge issues a challenge for a site key, for an address the key allows.
func (s *ChallengeService) NewChallenge(ctx context.Context, req *v1.NewChallengeRequest) (*v1.Challenge, error) {
	challenge, err := s.challenges.New(ctx, req.GetSiteKey(), req.GetChallengeType(), s.clientAddr(ctx))
	if err != nil {
		return nil, err
	}
	reply := &v1.Challenge{
		ChallengeId:   challenge.ID,
		ChallengeType: challenge.Type,
		Image:         challenge.Media.Data,
		MimeType:      challenge.Media.MIMEType,
		Width:         int32(challenge.Media.Width),
		Height:        int32(challenge.Media.Height),
		ExpiresIn:     int32(challenge.TTL.Seconds()),
	}
	// Left unset for the other types, so their replies carry none of the piece fields.
	if piece := challenge.Media.Piece; piece != nil {
		reply.PieceImage = piece.Data
		reply.PieceY = proto.Int32(int32(piece.Y))
		reply.PieceWidth = proto.Int32(int32(piece.Width))
		reply.PieceHeight = proto.Int32(int32(piece.Height))
	}
	return reply, nil
}

// clientAddr is the address the request comes from, read through the proxies we trust.
// The zero value means it could not be read, e.g. on a gRPC call.
func (s *ChallengeService) clientAddr(ctx context.Context) netip.Addr {
	serverTransport, ok := transport.FromServerContext(ctx)
	if !ok {
		return netip.Addr{}
	}
	request, ok := serverTransport.(*khttp.Transport)
	if !ok {
		return netip.Addr{}
	}
	return clientip.From(request.Request(), s.trusted)
}

// CheckAnswer consumes a challenge and reports whether the answer is correct, and what the
// behaviour check made of the behaviour sent with it.
func (s *ChallengeService) CheckAnswer(ctx context.Context, req *v1.CheckAnswerRequest) (*v1.CheckAnswerReply, error) {
	challenge, ok, behavior, err := s.challenges.Check(ctx, req.GetChallengeId(), req.GetAnswer(), toBehavior(req.GetBehavior()))
	if err != nil {
		return nil, err
	}
	return &v1.CheckAnswerReply{
		Success:       ok,
		ApiKeyId:      challenge.APIKeyID,
		ChallengeType: challenge.Type,
		UserId:        challenge.UserID,
		Behavior:      toBehaviorResult(behavior),
	}, nil
}
