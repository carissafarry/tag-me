package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const cooldownKeyPrefix = "cooldown"

type CooldownRepository struct {
	client redis.Cmdable
}

func NewCooldownRepository(client redis.Cmdable) *CooldownRepository {
	return &CooldownRepository{client: client}
}

func (r *CooldownRepository) Key(sessionID string, qrID string, action string) string {
	return fmt.Sprintf("%s:%s:%s:%s", cooldownKeyPrefix, sessionID, qrID, action)
}

func (r *CooldownRepository) GetNextAllowedAt(ctx context.Context, sessionID string, qrID string, action string) (*time.Time, error) {
	value, err := r.client.Get(ctx, r.Key(sessionID, qrID, action)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get cooldown: %w", err)
	}

	unixSeconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse cooldown value: %w", err)
	}

	nextAllowedAt := time.Unix(unixSeconds, 0).UTC()
	return &nextAllowedAt, nil
}

func (r *CooldownRepository) SetNextAllowedAt(
	ctx context.Context,
	sessionID string,
	qrID string,
	action string,
	nextAllowedAt time.Time,
	ttl time.Duration,
) error {
	if err := r.client.Set(ctx, r.Key(sessionID, qrID, action), strconv.FormatInt(nextAllowedAt.UTC().Unix(), 10), ttl).Err(); err != nil {
		return fmt.Errorf("set cooldown: %w", err)
	}

	return nil
}
