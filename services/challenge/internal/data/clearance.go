package data

import (
	"context"
	"fmt"
	"time"

	"challenge/internal/biz"
	"challenge/internal/pkg/quota"

	"github.com/redis/go-redis/v9"
)

// The passes of a clearance are counted like the history of an address (iphistory.go): one key per
// 10-minute bucket, wau:clearance:{<id>}:pass:<bucket>, read for an hour, and one per local day,
// wau:clearance:{<id>}:day:<YYYY-MM-DD>, which expires quota.CounterTTL after its first pass. The
// hash tag keeps all of a clearance on one node of the cluster, so its counters are read with one
// MGET and counted in one transaction.
type clearanceRepo struct {
	data *Data
}

func NewClearanceRepo(data *Data) biz.ClearanceRepo {
	return &clearanceRepo{data: data}
}

func clearanceBucketKey(id string, bucket int64) string {
	return fmt.Sprintf("wau:clearance:{%s}:pass:%d", id, bucket)
}

func clearanceDayKey(id, day string) string {
	return fmt.Sprintf("wau:clearance:{%s}:day:%s", id, day)
}

func (r *clearanceRepo) Passes(ctx context.Context, id string, now time.Time, day string) (int64, int64, error) {
	current := bucketOf(now)
	keys := make([]string, historyBuckets, historyBuckets+1)
	for i := range keys {
		keys[i] = clearanceBucketKey(id, current-int64(i))
	}
	counts, err := readTaggedCounters(ctx, r.data.redis, append(keys, clearanceDayKey(id, day)))
	if err != nil {
		return 0, 0, err
	}
	var lastHour int64
	for _, count := range counts[:historyBuckets] {
		lastHour += count
	}
	return lastHour, counts[historyBuckets], nil
}

func (r *clearanceRepo) CountPass(ctx context.Context, id string, now time.Time, day string) error {
	bucket, daily := clearanceBucketKey(id, bucketOf(now)), clearanceDayKey(id, day)
	_, err := r.data.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Incr(ctx, bucket)
		pipe.ExpireNX(ctx, bucket, historyBuckets*biz.HistoryBucket)
		pipe.Incr(ctx, daily)
		pipe.ExpireNX(ctx, daily, quota.CounterTTL)
		return nil
	})
	return err
}
