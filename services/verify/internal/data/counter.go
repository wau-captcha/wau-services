package data

import (
	"context"

	"verify/internal/biz"

	"common/quota"

	"github.com/redis/go-redis/v9"
)

type counterRepo struct {
	data *Data
}

func NewCounterRepo(data *Data) biz.CounterRepo {
	return &counterRepo{data: data}
}

func (r *counterRepo) CountVerification(ctx context.Context, apiKeyID, day string) error {
	return incrementDailyCounter(ctx, r.data.redis, quota.CheckedAnswersKey(apiKeyID, day))
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
