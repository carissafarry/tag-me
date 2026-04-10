package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/redis/go-redis/v9"
)

const ipRateLimitKeyPrefix = "ip"

var incrementIPRateLimitScript = redis.NewScript(`
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local max_requests = tonumber(ARGV[2])

local count = redis.call("INCR", key)
if count == 1 and ttl > 0 then
	redis.call("EXPIRE", key, ttl)
end

if count > max_requests then
	return {0, tostring(count), "0"}
end

return {1, tostring(count), tostring(max_requests - count)}
`)

type IPRateLimiter struct {
	client redis.Cmdable
	ttl    time.Duration
}

func NewIPRateLimiter(client redis.Cmdable, ttl time.Duration) *IPRateLimiter {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	return &IPRateLimiter{
		client: client,
		ttl:    ttl,
	}
}

func (r *IPRateLimiter) Key(ipAddress string, qrID string) string {
	return fmt.Sprintf("%s:%s:%s", ipRateLimitKeyPrefix, ipAddress, qrID)
}

func (r *IPRateLimiter) IncrementAndCheck(
	ctx context.Context,
	ipAddress string,
	qrID string,
	maxRequests int,
) (*models.IPRateLimitState, error) {
	raw, err := incrementIPRateLimitScript.Run(
		ctx,
		r.client,
		[]string{r.Key(ipAddress, qrID)},
		int(r.ttl.Seconds()),
		maxRequests,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("increment ip rate limit: %w", err)
	}

	values, ok := raw.([]interface{})
	if !ok || len(values) != 3 {
		return nil, fmt.Errorf("increment ip rate limit: unexpected response %T", raw)
	}

	allowed := fmt.Sprint(values[0]) == "1"
	count, err := strconv.Atoi(fmt.Sprint(values[1]))
	if err != nil {
		return nil, fmt.Errorf("parse ip rate count: %w", err)
	}
	remaining, err := strconv.Atoi(fmt.Sprint(values[2]))
	if err != nil {
		return nil, fmt.Errorf("parse ip rate remaining: %w", err)
	}

	return &models.IPRateLimitState{
		Allowed:   allowed,
		Count:     count,
		Remaining: remaining,
	}, nil
}
