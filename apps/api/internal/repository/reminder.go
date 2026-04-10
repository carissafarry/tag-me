package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/redis/go-redis/v9"
)

const reminderKeyPrefix = "reminder"

var reserveReminderScript = redis.NewScript(`
local reminder_key = KEYS[1]
local cooldown_key = KEYS[2]
local state_ttl = tonumber(ARGV[1])
local now_ts = tonumber(ARGV[2])
local cooldown_seconds = tonumber(ARGV[3])
local max_reminders = tonumber(ARGV[4])

local count = tonumber(redis.call("HGET", reminder_key, "count") or "0")
local last_sent_at = tonumber(redis.call("HGET", reminder_key, "last_sent_at") or "0")
local next_allowed_at = tonumber(redis.call("GET", cooldown_key) or "0")

if count >= max_reminders then
	return {"limit_reached", tostring(count), tostring(last_sent_at), tostring(next_allowed_at), "0"}
end

if next_allowed_at > now_ts then
	return {"cooldown", tostring(count), tostring(last_sent_at), tostring(next_allowed_at), tostring(max_reminders - count)}
end

count = count + 1
last_sent_at = now_ts
next_allowed_at = now_ts + cooldown_seconds

redis.call("HSET", reminder_key, "count", count, "last_sent_at", last_sent_at)
if state_ttl > 0 then
	redis.call("EXPIRE", reminder_key, state_ttl)
end

if cooldown_seconds > 0 then
	redis.call("SET", cooldown_key, tostring(next_allowed_at), "EX", cooldown_seconds)
end

return {"sent", tostring(count), tostring(last_sent_at), tostring(next_allowed_at), tostring(max_reminders - count)}
`)

type ReminderRepository struct {
	client       redis.Cmdable
	stateTTL     time.Duration
	cooldownRepo *CooldownRepository
}

func NewReminderRepository(client redis.Cmdable, stateTTL time.Duration, cooldownRepo *CooldownRepository) *ReminderRepository {
	if stateTTL <= 0 {
		stateTTL = 6 * time.Hour
	}

	return &ReminderRepository{
		client:       client,
		stateTTL:     stateTTL,
		cooldownRepo: cooldownRepo,
	}
}

func (r *ReminderRepository) Key(sessionID string, qrID string) string {
	return fmt.Sprintf("%s:%s:%s", reminderKeyPrefix, sessionID, qrID)
}

func (r *ReminderRepository) GetState(ctx context.Context, sessionID string, qrID string) (*models.ReminderState, error) {
	values, err := r.client.HGetAll(ctx, r.Key(sessionID, qrID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get reminder state: %w", err)
	}

	if len(values) == 0 {
		return &models.ReminderState{}, nil
	}

	count, err := strconv.Atoi(values[stateCountField])
	if err != nil {
		return nil, fmt.Errorf("parse reminder count: %w", err)
	}

	lastSentAtUnix, err := strconv.ParseInt(values[stateLastSentAtField], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse reminder last_sent_at: %w", err)
	}

	return &models.ReminderState{
		Count:      count,
		LastSentAt: time.Unix(lastSentAtUnix, 0).UTC(),
	}, nil
}

func (r *ReminderRepository) ReserveReminder(
	ctx context.Context,
	sessionID string,
	qrID string,
	now time.Time,
	cooldown time.Duration,
	maxReminders int,
) (*models.ReminderReservation, error) {
	raw, err := reserveReminderScript.Run(
		ctx,
		r.client,
		[]string{
			r.Key(sessionID, qrID),
			r.cooldownRepo.Key(sessionID, qrID, "reminder"),
		},
		int(r.stateTTL.Seconds()),
		now.UTC().Unix(),
		int(cooldown.Seconds()),
		maxReminders,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("reserve reminder: %w", err)
	}

	values, ok := raw.([]interface{})
	if !ok || len(values) != 5 {
		return nil, fmt.Errorf("reserve reminder: unexpected response %T", raw)
	}

	count, err := strconv.Atoi(fmt.Sprint(values[1]))
	if err != nil {
		return nil, fmt.Errorf("parse reminder count: %w", err)
	}

	lastSentAtUnix, err := parseOptionalUnix(values[2])
	if err != nil {
		return nil, fmt.Errorf("parse reminder last_sent_at: %w", err)
	}

	nextAllowedAtUnix, err := parseOptionalUnix(values[3])
	if err != nil {
		return nil, fmt.Errorf("parse reminder next_allowed_at: %w", err)
	}

	remainingReminder, err := strconv.Atoi(fmt.Sprint(values[4]))
	if err != nil {
		return nil, fmt.Errorf("parse reminder remaining count: %w", err)
	}

	result := &models.ReminderReservation{
		Reason:            models.ReminderReason(fmt.Sprint(values[0])),
		Count:             count,
		RemainingReminder: remainingReminder,
	}

	if lastSentAtUnix > 0 {
		lastSentAt := time.Unix(lastSentAtUnix, 0).UTC()
		result.LastSentAt = &lastSentAt
	}

	if nextAllowedAtUnix > 0 {
		nextAllowedAt := time.Unix(nextAllowedAtUnix, 0).UTC()
		result.NextAllowedAt = &nextAllowedAt
	}

	return result, nil
}
