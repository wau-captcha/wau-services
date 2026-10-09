package biz

import (
	"context"

	"verify/internal/pkg/afterresponse"
	"verify/internal/pkg/quota"

	"github.com/go-kratos/kratos/v3/log"
)

// CounterRepo keeps the daily counters shared with the challenge service (see internal/pkg/quota).
type CounterRepo interface {
	// CountAnswer records an answer checked for a challenge of the API key, for the scraping check
	// of the challenge service: every answer, and among them the right ones and the ones judged
	// automated.
	CountAnswer(ctx context.Context, apiKeyID, day string, solved, automated bool) error
	// CountSuccess records a correct answer against the daily quota of the user.
	CountSuccess(ctx context.Context, userID, day string) error
}

// recordAnswer updates the usage history of the challenge once the response is on its way,
// so answering never waits for the database. A failure is logged, not returned.
func (uc *VerifyUsecase) recordAnswer(ctx context.Context, challengeID string, solution *Solution) {
	afterresponse.Run(ctx, func(ctx context.Context) {
		if err := uc.sites.RecordAnswer(ctx, challengeID, solution); err != nil {
			log.ErrorContext(ctx, "failed recording answer", "err", err, "challenge_id", challengeID)
		}
	})
}

// recordCounters feeds the scraping check with every checked answer and the quota with correct
// ones. They are counted before the response, so the quota is exact; a failure is logged rather
// than denying the user the token.
func (uc *VerifyUsecase) recordCounters(ctx context.Context, solution *Solution) {
	day := quota.Day(uc.now(), uc.location)
	automated := solution.Behavior.Verdict == VerdictAutomated
	if err := uc.counters.CountAnswer(ctx, solution.APIKeyID, day, solution.Success, automated); err != nil {
		log.ErrorContext(ctx, "failed counting answer", "err", err)
	}
	if !solution.Success {
		return
	}
	if solution.UserID == "" {
		log.WarnContext(ctx, "challenge has no owner, quota not counted", "api_key_id", solution.APIKeyID)
		return
	}
	if err := uc.counters.CountSuccess(ctx, solution.UserID, day); err != nil {
		log.ErrorContext(ctx, "failed counting quota", "err", err)
	}
}
