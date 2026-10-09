package data

import (
	"context"
	"net/http"

	challengev1 "verify/api/challenge/v1"
	"verify/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
)

// forwardedReasons are the refusals of CheckAnswer that verify passes on to its own clients, who
// act on them: the SDKs load a new challenge after CHALLENGE_NOT_FOUND, for instance.
var forwardedReasons = map[string]bool{
	"CHALLENGE_NOT_FOUND":  true,
	"CHALLENGE_ID_INVALID": true,
	"RATE_LIMITED":         true,
}

type challengeClient struct {
	client challengev1.ChallengeServiceClient
}

// NewChallengeClient adapts the challenge gRPC client to biz.ChallengeClient.
func NewChallengeClient(client challengev1.ChallengeServiceClient) biz.ChallengeClient {
	return &challengeClient{client: client}
}

func (c *challengeClient) CheckAnswer(ctx context.Context, answer biz.Answer) (*biz.Solution, error) {
	reply, err := c.client.CheckAnswer(ctx, checkAnswerRequest(answer))
	if err != nil {
		return nil, challengeError(ctx, err)
	}
	return &biz.Solution{
		Success:          reply.GetSuccess(),
		APIKeyID:         reply.GetApiKeyId(),
		ChallengeType:    reply.GetChallengeType(),
		UserID:           reply.GetUserId(),
		Behavior:         behaviorResult(reply.GetBehavior()),
		Hostname:         reply.GetHostname(),
		BehaviorRequired: reply.GetBehaviorRequired(),
	}, nil
}

// checkAnswerRequest passes the answer on with its request; the address is left empty when unknown,
// which tells the challenge service not to compare the request.
func checkAnswerRequest(answer biz.Answer) *challengev1.CheckAnswerRequest {
	request := &challengev1.CheckAnswerRequest{
		ChallengeId:    answer.ChallengeID,
		Answer:         answer.Text,
		Behavior:       behaviorRequest(answer.Behavior),
		SiteKey:        answer.SiteKey,
		Origin:         answer.Origin,
		UserAgent:      answer.UserAgent,
		SecFetchSite:   answer.SecFetchSite,
		AcceptLanguage: answer.AcceptLanguage,
		PowNonce:       answer.PowNonce,
	}
	if answer.Client.IsValid() {
		request.ClientIp = answer.Client.String()
	}
	return request
}

func behaviorRequest(behavior *biz.Behavior) *challengev1.Behavior {
	if behavior == nil {
		return nil
	}
	samples := make([]*challengev1.PointerSample, len(behavior.Pointer))
	for i, sample := range behavior.Pointer {
		samples[i] = &challengev1.PointerSample{T: int32(sample.T), X: float32(sample.X), Y: float32(sample.Y)}
	}
	intervals := make([]int32, len(behavior.KeyIntervalsMs))
	for i, interval := range behavior.KeyIntervalsMs {
		intervals[i] = int32(interval)
	}
	return &challengev1.Behavior{
		Version:         int32(behavior.Version),
		ElapsedMs:       int32(behavior.ElapsedMs),
		Pointer:         samples,
		PointerType:     behavior.PointerType,
		KeyIntervalsMs:  intervals,
		InputEvents:     int32(behavior.InputEvents),
		PasteEvents:     int32(behavior.PasteEvents),
		UntrustedEvents: int32(behavior.UntrustedEvents),
		Webdriver:       behavior.Webdriver,
		HiddenMs:        int32(behavior.HiddenMs),

		HeadlessUa:                behavior.HeadlessUA,
		NoLanguages:               behavior.NoLanguages,
		ChromeWithoutWindowChrome: behavior.ChromeWithoutWindowChrome,
	}
}

// behaviorResult reads the verdict of the challenge service; one that does not check behaviour yet
// sends none, which is the same as not measured.
func behaviorResult(result *challengev1.BehaviorResult) biz.BehaviorResult {
	if result.GetVerdict() == "" {
		return biz.BehaviorResult{Verdict: biz.VerdictNotMeasured}
	}
	return biz.BehaviorResult{Verdict: result.GetVerdict(), Score: result.Score, Reasons: result.GetReasons()}
}

// challengeError passes on what the challenge service refused and hides anything else, such as
// an unreachable service or a timeout, behind ErrInternal: the text of those names addresses
// inside the cluster.
func challengeError(ctx context.Context, err error) error {
	refused := errors.FromError(err)
	if refused.Code < http.StatusInternalServerError && forwardedReasons[refused.Reason] {
		return errors.New(int(refused.Code), refused.Reason, refused.Message).WithMetadata(refused.Metadata)
	}
	log.ErrorContext(ctx, "challenge service failed", "err", err)
	return biz.ErrInternal
}
