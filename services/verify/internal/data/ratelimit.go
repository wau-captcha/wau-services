package data

import (
	"verify/internal/biz"
	"verify/internal/pkg/ratelimit"
)

// NewRateLimiter counts the rate limits in the Redis Cluster the grants live in.
func NewRateLimiter(data *Data) biz.RateLimiter {
	return ratelimit.New(data.redis)
}
