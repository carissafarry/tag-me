package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
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

type NotificationEnqueuer func(ctx context.Context, conversationID string, notificationType string) error

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
	db           *pgxpool.Pool
	reminders    ReminderRepository
	messages     MessageStateRepository
	cooldowns    CooldownRepository
	ipRateLimits IPRateLimiter
	enqueue      NotificationEnqueuer
	config       ReminderConfig
	now          func() time.Time
}

func NewReminderService(
	db *pgxpool.Pool,
	reminders ReminderRepository,
	messages MessageStateRepository,
	cooldowns CooldownRepository,
	ipRateLimits IPRateLimiter,
	enqueue NotificationEnqueuer,
	config *ReminderConfig,
	now func() time.Time,
) *ReminderService {
	finalConfig := ReminderConfig{
		Cooldown:                2 * time.Minute,
		MaxReminders:            3,
		MaxMessagesPerSessionQR: 5,
		IPWindowLimit:           10,
	}

	if config != nil {
		if config.Cooldown > 0 {
			finalConfig.Cooldown = config.Cooldown
		}
		if config.MaxReminders > 0 {
			finalConfig.MaxReminders = config.MaxReminders
		}
		if config.MaxMessagesPerSessionQR > 0 {
			finalConfig.MaxMessagesPerSessionQR = config.MaxMessagesPerSessionQR
		}
		if config.IPWindowLimit > 0 {
			finalConfig.IPWindowLimit = config.IPWindowLimit
		}
	}

	if enqueue == nil {
		enqueue = func(context.Context, string, string) error { return nil }
	}

	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &ReminderService{
		db:           db,
		reminders:    reminders,
		messages:     messages,
		cooldowns:    cooldowns,
		ipRateLimits: ipRateLimits,
		enqueue:      enqueue,
		config:       finalConfig,
		now:          now,
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
		if err := s.enqueue(ctx, request.ConversationID, "REMINDER"); err != nil {
			return nil, fmt.Errorf("enqueue reminder notification: %w", err)
		}

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
