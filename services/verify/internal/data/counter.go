package data

import (
	"context"

	"verify/internal/biz"
	"verify/internal/pkg/quota"

	"github.com/redis/go-redis/v9"
)

type counterRepo struct {
	data *Data
}

func NewCounterRepo(data *Data) biz.CounterRepo {
	return &counterRepo{data: data}
}

// CountAnswer adds to up to three counters. Their keys fall in different slots of the cluster, so
// each gets its own transaction.
func (r *counterRepo) CountAnswer(ctx context.Context, apiKeyID, day string, solved, automated bool) error {
	keys := []string{quota.CheckedAnswersKey(apiKeyID, day)}
	if solved {
		keys = append(keys, quota.SolvedAnswersKey(apiKeyID, day))
	}
	if automated {
		keys = append(keys, quota.AutomatedAnswersKey(apiKeyID, day))
	}
	for _, key := range keys {
		if err := incrementDailyCounter(ctx, r.data.redis, key); err != nil {
			return err
		}
	}
	return nil
}

func (r *counterRepo) CountSuccess(ctx context.Context, userID, day string) error {
	return incrementDailyCounter(ctx, r.data.redis, quota.DailyQuotaKey(userID, day))
}

// incrementDailyCounter adds one to a daily counter. The first increment creates it, and only
// then does the expiry get set, so the counter lives for quota.CounterTTL from its creation.
func incrementDailyCounter(ctx context.Context, client redis.Cmdable, key string) error {
	_, err := client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, quota.CounterTTL)
		return nil
	})
	return err
}
