package biz

import (
	"context"

	"common/quota"

	"github.com/go-kratos/kratos/v3/log"
)

// CounterRepo keeps the daily counters shared with the challenge service (see common/quota).
type CounterRepo interface {
	// CountVerification records an answer checked for a challenge of the API key.
	CountVerification(ctx context.Context, apiKeyID, day string) error
	// CountSuccess records a correct answer against the daily quota of the user.
	CountSuccess(ctx context.Context, userID, day string) error
}

// count feeds the scraping check with every checked answer and the quota with correct
// ones. Like usage logging, a failure is logged rather than denying the user the token.
func (uc *VerifyUsecase) count(ctx context.Context, sol *Solution) {
	day := quota.Day(uc.now(), uc.location)
	if err := uc.counters.CountVerification(ctx, sol.APIKeyID, day); err != nil {
		log.ErrorContext(ctx, "failed counting verification", "err", err)
	}
	if !sol.Success {
		return
	}
	if sol.UserID == "" {
		log.WarnContext(ctx, "challenge has no owner, quota not counted", "api_key_id", sol.APIKeyID)
		return
	}
	if err := uc.counters.CountSuccess(ctx, sol.UserID, day); err != nil {
		log.ErrorContext(ctx, "failed counting quota", "err", err)
	}
}
