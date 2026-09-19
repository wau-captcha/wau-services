package data

import (
	"context"
	"errors"

	"challenge/internal/biz"

	"common/quota"

	"github.com/redis/go-redis/v9"
)

type counterRepo struct {
	data *Data
}

func NewCounterRepo(data *Data) biz.CounterRepo {
	return &counterRepo{data: data}
}

func (r *counterRepo) QuotaUsed(ctx context.Context, userID, day string) (int64, error) {
	return getCount(ctx, r.data.rdb, quota.UsedKey(userID, day))
}

func (r *counterRepo) CountChallenge(ctx context.Context, apiKeyID, day string) (int64, int64, error) {
	issued, err := incrDaily(ctx, r.data.rdb, quota.ChallengesKey(apiKeyID, day))
	if err != nil {
		return 0, 0, err
	}
	verified, err := getCount(ctx, r.data.rdb, quota.VerificationsKey(apiKeyID, day))
	if err != nil {
		return 0, 0, err
	}
	return issued, verified, nil
}

// incrDaily adds one to a daily counter. The first increment creates it, and only
// then does the expiry get set, so the counter lives for quota.TTL from its creation.
func incrDaily(ctx context.Context, rdb redis.Cmdable, key string) (int64, error) {
	var count *redis.IntCmd
	_, err := rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		count = pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, quota.TTL)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count.Val(), nil
}

// getCount reads a counter; one that does not exist yet counts as zero.
func getCount(ctx context.Context, rdb redis.Cmdable, key string) (int64, error) {
	count, err := rdb.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return count, err
}
