package service

import (
	"context"
	"net/netip"

	v1 "challenge/api/challenge/v1"
	"challenge/internal/biz"
	"challenge/internal/conf"
	"challenge/internal/pkg/clientip"
	"challenge/internal/pkg/envconf"
	"challenge/internal/pkg/requestinfo"

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

// NewChallenge issues a challenge for a site key, to an address and a website the key allows.
func (s *ChallengeService) NewChallenge(ctx context.Context, req *v1.NewChallengeRequest) (*v1.Challenge, error) {
	request := biz.NewRequest{SiteKey: req.GetSiteKey(), ChallengeType: req.GetChallengeType(), FallbackType: req.GetFallbackType()}
	tr, httpRequest := httpTransport(ctx)
	if httpRequest != nil {
		request.Client = clientip.From(httpRequest, s.trusted)
		request.Origin = requestinfo.Origin(httpRequest)
		request.UserAgent = httpRequest.Header.Get("User-Agent")
	}
	challenge, bound, err := s.challenges.New(ctx, request)
	// A key bound to its domains answers its caller by name, refusals included, so the page can
	// read them; the CORS filter answers "*" for any other key. The header is echoed as sent:
	// browsers compare it byte for byte.
	if bound && httpRequest != nil {
		if origin := httpRequest.Header.Get("Origin"); origin != "" {
			tr.ReplyHeader().Set("Access-Control-Allow-Origin", origin)
			tr.ReplyHeader().Add("Vary", "Origin")
		}
	}
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
		PowSalt:       challenge.PowSalt,
		PowDifficulty: int32(challenge.PowDifficulty),
	}
	// Left unset for the other types, so their replies carry none of the piece fields.
	if piece := challenge.Media.Piece; piece != nil {
		reply.PieceImage = piece.Data
		reply.PieceY = proto.Int32(int32(piece.Y))
		reply.PieceWidth = proto.Int32(int32(piece.Width))
		reply.PieceHeight = proto.Int32(int32(piece.Height))
	}
	// animal_jump and object_rotate only, so the other types' replies carry no second_image either.
	if challenge.Media.SecondRound != nil {
		reply.SecondImage = challenge.Media.SecondRound
	}
	// object_rotate only, like the piece fields.
	if challenge.Media.Frames > 0 {
		reply.FrameCount = proto.Int32(int32(challenge.Media.Frames))
	}
	// auto_check only, even when empty: the client then knows the key allows nothing to fall back to.
	if challenge.Type == biz.TypeAutoCheck {
		reply.FallbackType = proto.String(challenge.FallbackType)
	}
	return reply, nil
}

// httpTransport is the HTTP transport and request behind ctx; nil, e.g., on a gRPC call.
func httpTransport(ctx context.Context) (*khttp.Transport, *khttp.Request) {
	serverTransport, ok := transport.FromServerContext(ctx)
	if !ok {
		return nil, nil
	}
	httpTransport, ok := serverTransport.(*khttp.Transport)
	if !ok {
		return nil, nil
	}
	return httpTransport, httpTransport.Request()
}

// CheckAnswer consumes a challenge and reports whether the answer solves it, what the behaviour
// check made of the behaviour sent with it, and the website it was asked for from.
func (s *ChallengeService) CheckAnswer(ctx context.Context, req *v1.CheckAnswerRequest) (*v1.CheckAnswerReply, error) {
	client, _ := netip.ParseAddr(req.GetClientIp())
	outcome, err := s.challenges.Check(ctx, biz.Answer{
		ChallengeID:    req.GetChallengeId(),
		Text:           req.GetAnswer(),
		Behavior:       toBehavior(req.GetBehavior()),
		SiteKey:        req.GetSiteKey(),
		Origin:         req.GetOrigin(),
		Client:         client,
		UserAgent:      req.GetUserAgent(),
		SecFetchSite:   req.GetSecFetchSite(),
		AcceptLanguage: req.GetAcceptLanguage(),
		PowNonce:       req.GetPowNonce(),
	})
	if err != nil {
		return nil, err
	}
	challenge := outcome.Challenge
	return &v1.CheckAnswerReply{
		Success:          outcome.Right,
		ApiKeyId:         challenge.APIKeyID,
		ChallengeType:    challenge.Type,
		UserId:           challenge.UserID,
		Behavior:         toBehaviorResult(outcome.Behavior),
		Hostname:         requestinfo.Host(challenge.Origin),
		BehaviorRequired: outcome.BehaviorRequired,
		Clearance:        outcome.Clearance,
	}, nil
}
