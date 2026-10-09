package biz

import (
	"context"
	"net/netip"
	"slices"

	"challenge/internal/pkg/quota"

	"github.com/google/uuid"
)

// NewRequest is a /challenge/new call: what it asks for and where it comes from.
type NewRequest struct {
	SiteKey string
	// image_distort when empty.
	ChallengeType string
	// The caller's address, read through the trusted proxies; zero when unknown.
	Client netip.Addr
	// The website the call comes from, scheme://host[:port] (requestinfo.Origin); empty when unknown.
	Origin    string
	UserAgent string
}

// New issues a challenge. The rate limits are counted before anything costly: per address before
// the key is even looked up, per key before the quota check and the rendering. bound reports
// whether the key lists the websites allowed to use it, so the reply, an error included, must name
// request.Origin in Access-Control-Allow-Origin rather than "*".
func (uc *ChallengeUsecase) New(ctx context.Context, request NewRequest) (challenge *Challenge, bound bool, err error) {
	if request.Client.IsValid() {
		if wait := uc.limiter.Allow(ctx, uc.limits.newPerIP, request.Client.String()); wait > 0 {
			return nil, false, rateLimited(wait)
		}
		if wait := uc.blockedFor(ctx, request.Client); wait > 0 {
			return nil, false, rateLimited(wait)
		}
	}
	challengeType := request.ChallengeType
	if challengeType == "" {
		challengeType = TypeImageDistort
	}
	kind, ok := challengeKinds[challengeType]
	if !ok {
		return nil, false, ErrChallengeTypeUnsupported
	}
	site, err := uc.sites.FindBySiteKey(ctx, request.SiteKey)
	if err != nil {
		return nil, false, err
	}
	bound = len(site.AllowedDomains) > 0
	switch {
	case site.OwnerBanned:
		return nil, bound, ErrAccountBanned
	case site.Suspended:
		return nil, bound, ErrApiKeySuspended
	case !addressAllowed(site.AllowedOrigins, request.Client):
		return nil, bound, ErrOriginNotAllowed
	case !domainAllowed(site.AllowedDomains, request.Origin):
		return nil, bound, ErrDomainNotAllowed
	case !slices.Contains(site.ChallengeTypes, challengeType):
		return nil, bound, ErrChallengeTypeNotAllowed
	}
	if wait := uc.limiter.Allow(ctx, uc.limits.perKey(uc.flaggedToday(site)), site.APIKeyID); wait > 0 {
		return nil, bound, rateLimited(wait)
	}
	if err := uc.admitChallenge(ctx, site); err != nil {
		return nil, bound, err
	}
	answer, media, err := kind.generate()
	if err != nil {
		return nil, bound, err
	}
	challenge = &Challenge{
		ID:              uuid.NewString(),
		Type:            challengeType,
		APIKeyID:        site.APIKeyID,
		SiteKey:         request.SiteKey,
		UserID:          site.UserID,
		Answer:          answer,
		Media:           media,
		TTL:             uc.ttl,
		IssuedAt:        uc.now(),
		Origin:          request.Origin,
		UserAgentHash:   hashUserAgent(request.UserAgent),
		RequireBehavior: site.RequireBehavior,
		PowDifficulty:   uc.powBits,
	}
	if request.Client.IsValid() {
		challenge.ClientIP = request.Client.String()
	}
	if uc.powBits > 0 {
		challenge.PowSalt = newPowSalt()
	}
	if err := uc.challenges.Save(ctx, challenge); err != nil {
		return nil, bound, err
	}
	uc.recordIssued(ctx, challenge)
	return challenge, bound, nil
}

// flaggedToday reports whether the scraping check flagged the key on the current local day.
func (uc *ChallengeUsecase) flaggedToday(site *Site) bool {
	return !site.FlaggedAt.IsZero() && quota.Day(site.FlaggedAt, uc.location) == quota.Day(uc.now(), uc.location)
}

// addressAllowed reports whether a request may use the key. A key with no allowed origins
// is open to every address; an unspecified entry ("0.0.0.0" or "::") also stands for every
// address, and any other entry covers the addresses of its network. A request whose address
// is unknown only gets through those two.
func addressAllowed(allowed []netip.Prefix, client netip.Addr) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, prefix := range allowed {
		if prefix.Addr().IsUnspecified() {
			return true
		}
		if client.IsValid() && prefix.Contains(client.Unmap()) {
			return true
		}
	}
	return false
}
