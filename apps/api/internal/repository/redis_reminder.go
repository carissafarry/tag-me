package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultReminderKeyPrefix = "reminder"
	countField               = "count"
	lastSentAtField          = "last_sent_at"
)

var reserveReminderScript = redis.NewScript(`
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local now_ts = tonumber(ARGV[2])
local cooldown = tonumber(ARGV[3])
local max_attempts = tonumber(ARGV[4])

local count = tonumber(redis.call("HGET", key, "count") or "0")
local last_sent_at = tonumber(redis.call("HGET", key, "last_sent_at") or "0")

if count >= max_attempts then
	return {"limit_reached", tostring(count), tostring(last_sent_at), ""}
end

if last_sent_at > 0 then
	local next_allowed = last_sent_at + cooldown
	if now_ts < next_allowed then
		return {"cooldown", tostring(count), tostring(last_sent_at), tostring(next_allowed)}
	end
end

count = count + 1
redis.call("HSET", key, "count", count, "last_sent_at", now_ts)

if ttl > 0 then
	redis.call("EXPIRE", key, ttl)
end

return {"sent", tostring(count), tostring(now_ts), tostring(now_ts + cooldown)}
`)

type RedisReminderRepository struct {
	client    redis.Cmdable
	keyPrefix string
	stateTTL  time.Duration
}

func NewRedisReminderRepository(client redis.Cmdable, stateTTL time.Duration) *RedisReminderRepository {
	if stateTTL <= 0 {
		stateTTL = 30 * 24 * time.Hour
	}

	return &RedisReminderRepository{
		client:    client,
		keyPrefix: DefaultReminderKeyPrefix,
		stateTTL:  stateTTL,
	}
}

func (r *RedisReminderRepository) ReminderKey(conversationID string) string {
	return fmt.Sprintf("%s:%s", r.keyPrefix, conversationID)
}

// GetReminderState returns the current reminder state for observability and tests.
func (r *RedisReminderRepository) GetReminderState(ctx context.Context, conversationID string) (*models.ReminderState, error) {
	values, err := r.client.HGetAll(ctx, r.ReminderKey(conversationID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get reminder state: %w", err)
	}

	if len(values) == 0 {
		return &models.ReminderState{}, nil
	}

	count, err := strconv.Atoi(values[countField])
	if err != nil {
		return nil, fmt.Errorf("parse reminder count: %w", err)
	}

	lastSentUnix, err := strconv.ParseInt(values[lastSentAtField], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse reminder last_sent_at: %w", err)
	}

	return &models.ReminderState{
		Count:      count,
		LastSentAt: time.Unix(lastSentUnix, 0).UTC(),
	}, nil
}

// ReserveReminder atomically enforces cooldown and max-attempt rules.
func (r *RedisReminderRepository) ReserveReminder(
	ctx context.Context,
	conversationID string,
	now time.Time,
	cooldown time.Duration,
	maxAttempts int,
) (*models.ReminderReservation, error) {
	if maxAttempts <= 0 {
		return nil, errors.New("max attempts must be greater than zero")
	}

	if cooldown < 0 {
		return nil, errors.New("cooldown must be non-negative")
	}

	raw, err := reserveReminderScript.Run(
		ctx,
		r.client,
		[]string{r.ReminderKey(conversationID)},
		int(r.stateTTL.Seconds()),
		now.UTC().Unix(),
		int(cooldown.Seconds()),
		maxAttempts,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("reserve reminder: %w", err)
	}

	values, ok := raw.([]interface{})
	if !ok || len(values) != 4 {
		return nil, fmt.Errorf("reserve reminder: unexpected response %T", raw)
	}

	reason := models.ReminderReason(fmt.Sprint(values[0]))
	count, err := strconv.Atoi(fmt.Sprint(values[1]))
	if err != nil {
		return nil, fmt.Errorf("parse reservation count: %w", err)
	}

	lastSentUnix, err := parseOptionalUnix(values[2])
	if err != nil {
		return nil, fmt.Errorf("parse reservation last_sent_at: %w", err)
	}

	nextAllowedUnix, err := parseOptionalUnix(values[3])
	if err != nil {
		return nil, fmt.Errorf("parse reservation next_allowed_at: %w", err)
	}

	reservation := &models.ReminderReservation{
		Reason: reason,
		Count:  count,
	}

	if lastSentUnix > 0 {
		reservation.LastSentAt = time.Unix(lastSentUnix, 0).UTC()
	}

	if nextAllowedUnix > 0 {
		nextAllowedAt := time.Unix(nextAllowedUnix, 0).UTC()
		reservation.NextAllowedAt = &nextAllowedAt
	}

	return reservation, nil
}

func parseOptionalUnix(value interface{}) (int64, error) {
	text := fmt.Sprint(value)
	if text == "" {
		return 0, nil
	}

	return strconv.ParseInt(text, 10, 64)
}
