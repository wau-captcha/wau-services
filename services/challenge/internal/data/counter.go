package data

import (
	"context"
	"errors"

	"challenge/internal/biz"
	"challenge/internal/pkg/quota"

	"github.com/redis/go-redis/v9"
)

type counterRepo struct {
	data *Data
}

func NewCounterRepo(data *Data) biz.CounterRepo {
	return &counterRepo{data: data}
}

func (r *counterRepo) QuotaUsed(ctx context.Context, userID, day string) (int64, error) {
	return readCounter(ctx, r.data.redis, quota.DailyQuotaKey(userID, day))
}

func (r *counterRepo) CountChallenge(ctx context.Context, apiKeyID, day string) (int64, int64, error) {
	issued, err := incrementDailyCounter(ctx, r.data.redis, quota.IssuedChallengesKey(apiKeyID, day))
	if err != nil {
		return 0, 0, err
	}
	answered, err := readCounter(ctx, r.data.redis, quota.CheckedAnswersKey(apiKeyID, day))
	if err != nil {
		return 0, 0, err
	}
	return issued, answered, nil
}

// incrementDailyCounter adds one to a daily counter. The first increment creates it, and only
// then does the expiry get set, so the counter lives for quota.CounterTTL from its creation.
func incrementDailyCounter(ctx context.Context, client redis.Cmdable, key string) (int64, error) {
	var count *redis.IntCmd
	_, err := client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		count = pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, quota.CounterTTL)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count.Val(), nil
}

// readCounter reads a counter; one that does not exist yet counts as zero.
func readCounter(ctx context.Context, client redis.Cmdable, key string) (int64, error) {
	count, err := client.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return count, err
}
