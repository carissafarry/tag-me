package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReminderRepository interface {
	GetState(ctx context.Context, sessionID string, qrID string) (*models.ReminderState, error)
	ReserveReminder(ctx context.Context, sessionID string, qrID string, now time.Time, cooldown time.Duration, maxReminders int) (*models.ReminderReservation, error)
}

type MessageStateRepository interface {
	GetState(ctx context.Context, sessionID string, qrID string) (*models.MessageState, error)
}

type CooldownRepository interface {
	GetNextAllowedAt(ctx context.Context, sessionID string, qrID string, action string) (*time.Time, error)
}

type IPRateLimiter interface {
	IncrementAndCheck(ctx context.Context, ipAddress string, qrID string, maxRequests int) (*models.IPRateLimitState, error)
}

type NotificationEnqueuer interface {
	EnqueueNotification(ctx context.Context, notificationType string, conversationID string, ownerContact string) error
}

// NoOpNotificationEnqueuer is a no-op implementation of NotificationEnqueuer
type NoOpNotificationEnqueuer struct{}

func (n *NoOpNotificationEnqueuer) EnqueueNotification(ctx context.Context, notificationType string, conversationID string, ownerContact string) error {
	return nil
}

type ReminderConfig struct {
	Cooldown                time.Duration
	MaxReminders            int
	MaxMessagesPerSessionQR int
	IPWindowLimit           int
}

type ConversationLookupRepository interface {
	FindByID(ctx context.Context, conversationID string) (*models.Conversation, error)
}

type ReminderService struct {
	conversations ConversationLookupRepository
	reminders     ReminderRepository
	messages      MessageStateRepository
	cooldowns     CooldownRepository
	ipRateLimits  IPRateLimiter
	enqueue       NotificationEnqueuer
	config        ReminderConfig
	now           func() time.Time
}

const reminderEnqueueTimeout = 2 * time.Second

// Unused yet
func NewReminderService(db *pgxpool.Pool) *ReminderService {
	return NewReminderServiceWithDependencies(
		repository.NewConversationRepository(db),
		nil,
		nil,
		nil,
		nil,
		&NoOpNotificationEnqueuer{},
		&ReminderConfig{
			Cooldown:                2 * time.Minute,
			MaxReminders:            3,
			MaxMessagesPerSessionQR: 5,
			IPWindowLimit:           10,
		},
		func() time.Time { return time.Now().UTC() },
	)
}

func NewReminderServiceWithDependencies(
	conversations ConversationLookupRepository,
	reminders ReminderRepository,
	messages MessageStateRepository,
	cooldowns CooldownRepository,
	ipRateLimits IPRateLimiter,
	enqueue NotificationEnqueuer,
	config *ReminderConfig,
	now func() time.Time,
) *ReminderService {
	if enqueue == nil {
		enqueue = &NoOpNotificationEnqueuer{}
	}

	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	if config == nil {
		config = &ReminderConfig{
			Cooldown:                2 * time.Minute,
			MaxReminders:            3,
			MaxMessagesPerSessionQR: 5,
			IPWindowLimit:           10,
		}
	}

	return &ReminderService{
		conversations: conversations,
		reminders:     reminders,
		messages:      messages,
		cooldowns:     cooldowns,
		ipRateLimits:  ipRateLimits,
		enqueue:       enqueue,
		config:        *config,
		now:           now,
	}
}

func (s *ReminderService) SendReminder(ctx context.Context, request models.ReminderRequest) (*models.ReminderResult, error) {
	if request.ConversationID == "" || request.SessionID == "" || request.IPAddress == "" {
		return &models.ReminderResult{
			Success: false,
			Reason:  models.ReminderReasonInvalidConversation,
		}, nil
	}

	if _, err := uuid.Parse(request.ConversationID); err != nil {
		return &models.ReminderResult{
			Success: false,
			Reason:  models.ReminderReasonInvalidConversation,
		}, nil
	}

	conversation, err := s.conversations.FindByID(ctx, request.ConversationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &models.ReminderResult{
				Success: false,
				Reason:  models.ReminderReasonInvalidConversation,
			}, nil
		}

		return nil, fmt.Errorf("lookup conversation: %w", err)
	}

	if conversation.Status == "RESOLVED" || !AllowedConversationStatuses[conversation.Status] {
		return &models.ReminderResult{
			Success: false,
			Reason:  models.ReminderReasonInvalidConversation,
		}, nil
	}

	qrID := conversation.QRCodeID.String()

	ipDecision, err := s.ipRateLimits.IncrementAndCheck(ctx, request.IPAddress, qrID, s.config.IPWindowLimit)
	if err != nil {
		return nil, fmt.Errorf("ip rate limit: %w", err)
	}
	if !ipDecision.Allowed {
		return &models.ReminderResult{
			Success: false,
			Reason:  models.ReminderReasonRateLimited,
		}, nil
	}

	messageState, err := s.messages.GetState(ctx, request.SessionID, qrID)
	if err != nil {
		return nil, fmt.Errorf("message state: %w", err)
	}
	if messageState.Count >= s.config.MaxMessagesPerSessionQR {
		return &models.ReminderResult{
			Success: false,
			Reason:  models.ReminderReasonRateLimited,
		}, nil
	}

	reminderState, err := s.reminders.GetState(ctx, request.SessionID, qrID)
	if err != nil {
		return nil, fmt.Errorf("reminder state: %w", err)
	}
	if reminderState.Count >= s.config.MaxReminders {
		return &models.ReminderResult{
			Success: false,
			Reason:  models.ReminderReasonLimitReached,
		}, nil
	}

	nextAllowedAt, err := s.cooldowns.GetNextAllowedAt(ctx, request.SessionID, qrID, "reminder")
	if err != nil {
		return nil, fmt.Errorf("cooldown state: %w", err)
	}
	now := s.now()
	if nextAllowedAt != nil && now.Before(*nextAllowedAt) {
		return &models.ReminderResult{
			Success:       false,
			Reason:        models.ReminderReasonCooldown,
			NextAllowedAt: nextAllowedAt,
		}, nil
	}

	reservation, err := s.reminders.ReserveReminder(
		ctx,
		request.SessionID,
		qrID,
		now,
		s.config.Cooldown,
		s.config.MaxReminders,
	)
	if err != nil {
		return nil, fmt.Errorf("reserve reminder: %w", err)
	}

	switch reservation.Reason {
	case models.ReminderReasonCooldown:
		return &models.ReminderResult{
			Success:       false,
			Reason:        models.ReminderReasonCooldown,
			NextAllowedAt: reservation.NextAllowedAt,
		}, nil
	case models.ReminderReasonLimitReached:
		return &models.ReminderResult{
			Success: false,
			Reason:  models.ReminderReasonLimitReached,
		}, nil
	case models.ReminderReasonSent:
		conversationID := request.ConversationID
		go func(conversationID string) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Printf("warning: panic enqueueing notification: conversation_id=%s panic=%v", conversationID, recovered)
				}
			}()

			enqueueCtx, cancel := context.WithTimeout(context.Background(), reminderEnqueueTimeout)
			defer cancel()

			if err := s.enqueue.EnqueueNotification(enqueueCtx, "reminder", conversationID, ""); err != nil {
				// Log enqueue error but don't fail the reminder request.
				// The reminder was already recorded successfully.
				log.Printf("warning: failed to enqueue reminder notification: conversation_id=%s type=reminder err=%v", conversationID, err)
			}
		}(conversationID)

		remainingReminder := reservation.RemainingReminder
		return &models.ReminderResult{
			Success:           true,
			Message:           "Reminder sent",
			RemainingReminder: &remainingReminder,
			NextAllowedAt:     reservation.NextAllowedAt,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected reservation reason: %s", reservation.Reason)
	}
}
