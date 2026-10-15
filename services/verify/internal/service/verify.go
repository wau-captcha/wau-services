package service

import (
	"context"
	"net/http"
	"net/netip"
	"strings"

	v1 "verify/api/verify/v1"
	"verify/internal/biz"
	"verify/internal/conf"
	"verify/internal/pkg/clientip"
	"verify/internal/pkg/envconf"
	"verify/internal/pkg/requestinfo"

	"github.com/go-kratos/kratos/v3/transport"
	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// VerifyService implements the verify API.
type VerifyService struct {
	v1.UnimplementedVerifyServiceServer

	verifier *biz.VerifyUsecase
	// Proxies whose X-Forwarded-For names the real caller, e.g. Traefik.
	trusted []netip.Prefix
}

// NewVerifyService creates a VerifyService.
func NewVerifyService(verifier *biz.VerifyUsecase, serverConf *conf.Server) (*VerifyService, error) {
	trusted, err := clientip.ParsePrefixes(envconf.SplitList(serverConf.GetTrustedProxies()))
	if err != nil {
		return nil, err
	}
	return &VerifyService{verifier: verifier, trusted: trusted}, nil
}

// VerifyChallenge checks an answer and issues a one-time token on success.
func (s *VerifyService) VerifyChallenge(ctx context.Context, req *v1.VerifyChallengeRequest) (*v1.VerifyChallengeReply, error) {
	answer := biz.Answer{
		ChallengeID: req.GetChallengeId(),
		Text:        req.GetAnswer(),
		Behavior:    toBehavior(req.GetBehavior()),
		SiteKey:     req.GetSiteKey(),
		PowNonce:    req.GetPowNonce(),
	}
	if request := httpRequest(ctx); request != nil {
		answer.Origin = requestinfo.Origin(request)
		answer.Client = clientip.From(request, s.trusted)
		answer.UserAgent = request.Header.Get("User-Agent")
		answer.SecFetchSite = request.Header.Get("Sec-Fetch-Site")
		answer.AcceptLanguage = request.Header.Get("Accept-Language")
	}
	token, err := s.verifier.VerifyChallenge(ctx, answer)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return &v1.VerifyChallengeReply{}, nil
	}
	return &v1.VerifyChallengeReply{Success: true, Token: token.Value, ExpiresIn: int32(token.TTL.Seconds()), Clearance: token.Clearance}, nil
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
		Behavior:      toBehaviorResult(g.Behavior),
		Hostname:      g.Hostname,
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

// httpRequest is the HTTP request behind ctx; nil, e.g., on a gRPC call.
func httpRequest(ctx context.Context) *http.Request {
	serverTransport, ok := transport.FromServerContext(ctx)
	if !ok {
		return nil
	}
	httpTransport, ok := serverTransport.(*khttp.Transport)
	if !ok {
		return nil
	}
	return httpTransport.Request()
}
