package data

import (
	"context"

	challengev1 "verify/api/challenge/v1"
	"verify/internal/biz"
)

type challengeClient struct {
	client challengev1.ChallengeServiceClient
}

// NewChallengeClient adapts the challenge gRPC client to biz.ChallengeClient.
func NewChallengeClient(client challengev1.ChallengeServiceClient) biz.ChallengeClient {
	return &challengeClient{client: client}
}

func (c *challengeClient) CheckAnswer(ctx context.Context, challengeID, answer string, behavior *biz.Behavior) (*biz.Solution, error) {
	reply, err := c.client.CheckAnswer(ctx, &challengev1.CheckAnswerRequest{
		ChallengeId: challengeID,
		Answer:      answer,
		Behavior:    behaviorRequest(behavior),
	})
	if err != nil {
		return nil, err
	}
	return &biz.Solution{
		Success:       reply.GetSuccess(),
		APIKeyID:      reply.GetApiKeyId(),
		ChallengeType: reply.GetChallengeType(),
		UserID:        reply.GetUserId(),
		Behavior:      behaviorResult(reply.GetBehavior()),
	}, nil
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
