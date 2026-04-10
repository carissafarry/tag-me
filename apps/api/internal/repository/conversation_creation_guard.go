package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const conversationCreationGuardKeyPrefix = "conversation_creation_guard"

type ConversationCreationGuardRepository struct {
	client redis.Cmdable
}

func NewConversationCreationGuardRepository(client redis.Cmdable) *ConversationCreationGuardRepository {
	return &ConversationCreationGuardRepository{client: client}
}

func (r *ConversationCreationGuardRepository) Key(sessionID string, ipAddress string, qrID string) string {
	return fmt.Sprintf("%s:%s:%s:%s", conversationCreationGuardKeyPrefix, sessionID, ipAddress, qrID)
}

func (r *ConversationCreationGuardRepository) ReserveConversationCreation(
	ctx context.Context,
	sessionID string,
	ipAddress string,
	qrID string,
	ttl time.Duration,
) (bool, time.Duration, error) {
	key := r.Key(sessionID, ipAddress, qrID)

	allowed, err := r.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, 0, fmt.Errorf("reserve conversation creation: %w", err)
	}

	if allowed {
		return true, 0, nil
	}

	retryAfter, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return false, 0, fmt.Errorf("read conversation creation ttl: %w", err)
	}

	if retryAfter < 0 {
		retryAfter = ttl
	}

	return false, retryAfter, nil
}
