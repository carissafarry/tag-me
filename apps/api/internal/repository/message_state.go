package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/redis/go-redis/v9"
)

const messageKeyPrefix = "msg"

var trackMessageScript = redis.NewScript(`
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local now_ts = tonumber(ARGV[2])

local count = tonumber(redis.call("HGET", key, "count") or "0")
count = count + 1

redis.call("HSET", key, "count", count, "last_sent_at", now_ts)
if ttl > 0 then
	redis.call("EXPIRE", key, ttl)
end

return {tostring(count), tostring(now_ts)}
`)

type MessageStateRepository struct {
	client redis.Cmdable
	ttl    time.Duration
}

func NewMessageStateRepository(client redis.Cmdable, ttl time.Duration) *MessageStateRepository {
	if ttl <= 0 {
		ttl = 6 * time.Hour
	}

	return &MessageStateRepository{
		client: client,
		ttl:    ttl,
	}
}

func (r *MessageStateRepository) Key(sessionID string, qrID string) string {
	return fmt.Sprintf("%s:%s:%s", messageKeyPrefix, sessionID, qrID)
}

func (r *MessageStateRepository) TrackMessage(ctx context.Context, sessionID string, qrID string, now time.Time) (*models.MessageState, error) {
	raw, err := trackMessageScript.Run(
		ctx,
		r.client,
		[]string{r.Key(sessionID, qrID)},
		int(r.ttl.Seconds()),
		now.UTC().Unix(),
	).Result()
	if err != nil {
		return nil, fmt.Errorf("track message: %w", err)
	}

	values, ok := raw.([]interface{})
	if !ok || len(values) != 2 {
		return nil, fmt.Errorf("track message: unexpected response %T", raw)
	}

	count, err := strconv.Atoi(fmt.Sprint(values[0]))
	if err != nil {
		return nil, fmt.Errorf("parse message count: %w", err)
	}

	lastSentAtUnix, err := strconv.ParseInt(fmt.Sprint(values[1]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse message last_sent_at: %w", err)
	}

	return &models.MessageState{
		Count:      count,
		LastSentAt: time.Unix(lastSentAtUnix, 0).UTC(),
	}, nil
}

func (r *MessageStateRepository) GetState(ctx context.Context, sessionID string, qrID string) (*models.MessageState, error) {
	values, err := r.client.HGetAll(ctx, r.Key(sessionID, qrID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get message state: %w", err)
	}

	if len(values) == 0 {
		return &models.MessageState{}, nil
	}

	count, err := strconv.Atoi(values[stateCountField])
	if err != nil {
		return nil, fmt.Errorf("parse message count: %w", err)
	}

	lastSentAtUnix, err := strconv.ParseInt(values[stateLastSentAtField], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse message last_sent_at: %w", err)
	}

	return &models.MessageState{
		Count:      count,
		LastSentAt: time.Unix(lastSentAtUnix, 0).UTC(),
	}, nil
}
