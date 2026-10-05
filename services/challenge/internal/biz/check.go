package biz

import (
	"context"
	"math"
	"time"
)

// Check takes the challenge, so each one is answered once, and reports whether answer solves it,
// together with what the behaviour check made of behavior (nil when the client sent none). A right
// answer judged automated does not solve it. A malformed answer is a wrong one, not an error.
func (uc *ChallengeUsecase) Check(ctx context.Context, id, answer string, behavior *Behavior) (*Challenge, bool, BehaviorResult, error) {
	challenge, err := uc.challenges.Take(ctx, id)
	if err != nil {
		return nil, false, BehaviorResult{}, err
	}
	sinceIssue := uc.sinceIssue(challenge)
	result := checkBehavior(behavior, challenge.Type, sinceIssue)
	kind, ok := challengeKinds[challenge.Type]
	if !ok {
		return challenge, false, result, nil
	}
	right := kind.check(answer, challenge.Answer, sinceIssue) && result.Verdict != VerdictAutomated
	return challenge, right, result, nil
}

// sinceIssue is how long ago the challenge was issued. One stored without its issue time, by the
// version before during a rollout, counts as issued long ago.
func (uc *ChallengeUsecase) sinceIssue(challenge *Challenge) time.Duration {
	if challenge.IssuedAt.IsZero() {
		return math.MaxInt64
	}
	return uc.now().Sub(challenge.IssuedAt)
}
