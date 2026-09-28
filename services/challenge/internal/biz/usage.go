package biz

import (
	"context"

	"challenge/internal/pkg/afterresponse"

	"github.com/go-kratos/kratos/v3/log"
)

// UsageRepo keeps the usage history in usage_logs.
type UsageRepo interface {
	// RecordIssued writes the /challenge/new call of a challenge, not yet answered.
	RecordIssued(ctx context.Context, challenge *Challenge) error
}

// recordIssued writes the challenge into the usage history once the response is on its way,
// so issuing a challenge never waits for the database. A failure is logged, not returned: the
// person already has the challenge.
func (uc *ChallengeUsecase) recordIssued(ctx context.Context, challenge *Challenge) {
	afterresponse.Run(ctx, func(ctx context.Context) {
		if err := uc.usage.RecordIssued(ctx, challenge); err != nil {
			log.ErrorContext(ctx, "failed recording issued challenge", "err", err, "challenge_id", challenge.ID)
		}
	})
}
