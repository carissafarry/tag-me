package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidQRToken            = errors.New("invalid qr token")
	ErrInactiveFQRToken          = errors.New("qr token inactive")
	ErrDatabaseError             = errors.New("database error")
	ErrInvalidStatus             = errors.New("invalid conversation status")
	ErrConversationRateLimited   = errors.New("conversation rate limited")
	ErrMessageServiceUnavailable = errors.New("message service unavailable")
	ErrDailyMessageLimitExceeded = errors.New("daily message limit exceeded for this QR")
)

// AllowedConversationStatuses defines the valid status states from TAG-8
var AllowedConversationStatuses = map[string]bool{
	"PENDING":    true,
	"DELIVERED":  true,
	"OPENED":     true,
	"ON_THE_WAY": true,
	"RESOLVED":   true,
}

type MessageService struct {
	qrCodes          QRCodeRepository
	conversations    ConversationRepository
	messages         MessageRepository
	messageStates    MessageStateReader
	creationGuard    ConversationCreationGuardRepository
	enqueue          NotificationEnqueuer
	creationCooldown time.Duration
	maxMessagesPerQR int
}

type QRCodeRepository interface {
	FindByToken(ctx context.Context, qrToken string) (*models.QRCode, error)
}

type ConversationRepository interface {
	Create(ctx context.Context, conversation *models.Conversation) (*models.Conversation, error)
	FindAll(ctx context.Context, ownerID string) ([]*models.Conversation, error)
	FindByID(ctx context.Context, conversationID string) (*models.Conversation, error)
	FindActiveBySessionAndQR(ctx context.Context, sessionID string, qrCodeID string) (*models.Conversation, error)
}

type MessageRepository interface {
	Create(ctx context.Context, message *models.Message) (*models.Message, error)
}

type MessageStateReader interface {
	GetState(ctx context.Context, sessionID string, qrID string) (*models.MessageState, error)
}

type ConversationCreationGuardRepository interface {
	ReserveConversationCreation(ctx context.Context, sessionID string, ipAddress string, qrID string, ttl time.Duration) (bool, time.Duration, error)
}

type MessageConfig struct {
	ConversationCreationCooldown time.Duration
	MaxMessagesPerSessionQR      int
}

type ConversationRateLimitError struct {
	RetryAfter time.Duration
	Reason     string // COOLDOWN or DAILY_LIMIT
}

func (e *ConversationRateLimitError) Error() string {
	if e.Reason == "DAILY_LIMIT" {
		return "you have reached the daily message limit"
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("you have reached the limit for creating conversations, please try again later (retry after %s)", e.RetryAfter)
	}
	return "you have reached the limit for creating conversations, please try again later"
}

func (e *ConversationRateLimitError) Is(target error) bool {
	if e.Reason == "DAILY_LIMIT" {
		return target == ErrDailyMessageLimitExceeded
	}
	return target == ErrConversationRateLimited
}

func NewMessageService(db *pgxpool.Pool) *MessageService {
	return NewMessageServiceWithDependencies(
		repository.NewQRCodeRepository(db),
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
		nil,
		nil,
		nil,
		nil,
	)
}

func NewMessageServiceWithDependencies(
	qrCodes QRCodeRepository,
	conversations ConversationRepository,
	messages MessageRepository,
	messageStates MessageStateReader,
	creationGuard ConversationCreationGuardRepository,
	enqueue NotificationEnqueuer,
	config *MessageConfig,
) *MessageService {
	finalConfig := MessageConfig{
		ConversationCreationCooldown: time.Minute,
		MaxMessagesPerSessionQR:      5,
	}

	if config != nil {
		if config.ConversationCreationCooldown > 0 {
			finalConfig.ConversationCreationCooldown = config.ConversationCreationCooldown
		}
		if config.MaxMessagesPerSessionQR > 0 {
			finalConfig.MaxMessagesPerSessionQR = config.MaxMessagesPerSessionQR
		}
	}

	return &MessageService{
		qrCodes:          qrCodes,
		conversations:    conversations,
		messages:         messages,
		messageStates:    messageStates,
		creationGuard:    creationGuard,
		enqueue:          enqueue,
		creationCooldown: finalConfig.ConversationCreationCooldown,
		maxMessagesPerQR: finalConfig.MaxMessagesPerSessionQR,
	}
}

// ResolveQRToken validates qr_token and returns owner and object context
func (s *MessageService) ResolveQRToken(ctx context.Context, qrToken string) (*models.QRCode, error) {
	if qrToken == "" {
		return nil, ErrInvalidQRToken
	}

	qr, err := s.qrCodes.FindByToken(ctx, qrToken)
	if err != nil {
		// QueryRow returns ErrNoRows if not found
		return nil, ErrInvalidQRToken
	}

	if !qr.IsActive {
		return nil, ErrInactiveFQRToken
	}

	return qr, nil
}

// CreateConversation creates a new conversation for a QR code if one doesn't exist
func (s *MessageService) CreateConversation(ctx context.Context, qrCode *models.QRCode, sessionID string, ipAddress string) (*models.Conversation, error) {
	if s.creationGuard != nil && sessionID != "" && ipAddress != "" {
		allowed, retryAfter, err := s.creationGuard.ReserveConversationCreation(
			ctx,
			sessionID,
			ipAddress,
			qrCode.ID.String(),
			s.creationCooldown,
		)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMessageServiceUnavailable, err)
		}
		if !allowed {
			return nil, &ConversationRateLimitError{RetryAfter: retryAfter, Reason: "COOLDOWN"}
		}
	}

	if s.messageStates != nil && sessionID != "" && s.maxMessagesPerQR > 0 {
		messageState, err := s.messageStates.GetState(ctx, sessionID, qrCode.ID.String())
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMessageServiceUnavailable, err)
		}
		if messageState.Count >= s.maxMessagesPerQR {
			return nil, &ConversationRateLimitError{Reason: "DAILY_LIMIT"}
		}
	}

	conv := &models.Conversation{
		ID:       uuid.New(),
		QRCodeID: qrCode.ID,
		OwnerID:  qrCode.OwnerID,
		Status:   "PENDING",
	}

	storedConversation, err := s.conversations.Create(ctx, conv)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseError, err)
	}

	return storedConversation, nil
}

// CreateMessage creates a new message in a conversation
func (s *MessageService) CreateMessage(
	ctx context.Context,
	conversationID uuid.UUID,
	messageType string,
	content *string,
	locationLatitude *float64,
	locationLongitude *float64,
	locationText *string,
	sessionID *string,
	ipAddress *string,
) (*models.Message, error) {
	msg := &models.Message{
		ID:                uuid.New(),
		ConversationID:    conversationID,
		SenderType:        "SCANNER",
		MessageType:       messageType,
		Content:           content,
		LocationLatitude:  locationLatitude,
		LocationLongitude: locationLongitude,
		LocationText:      locationText,
		SessionID:         sessionID,
		IPAddress:         ipAddress,
	}

	storedMessage, err := s.messages.Create(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseError, err)
	}

	return storedMessage, nil
}

// EnqueueNewMessageNotification enqueues a new_message notification
func (s *MessageService) EnqueueNewMessageNotification(ctx context.Context, conversationID string, ownerContact string) error {
	if s.enqueue == nil {
		return nil // No enqueuer configured, skip
	}

	return s.enqueue.EnqueueNotification(ctx, "new_message", conversationID, ownerContact)
}

func (s *MessageService) GetConversations(ctx context.Context, ownerID string) ([]*models.Conversation, error) {
	return s.conversations.FindAll(ctx, ownerID)
}

// GetDetailConversation retrieves the current status of a conversation
// Validates that status is in the allowed set (AC5: status values map correctly from owner actions)
func (s *MessageService) GetDetailConversation(ctx context.Context, conversationID string) (*models.Conversation, error) {
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		// QueryRow returns ErrNoRows if not found
		return nil, ErrDatabaseError
	}

	// AC2: Validate that returned status is one of the allowed states
	if !AllowedConversationStatuses[conv.Status] {
		return nil, ErrInvalidStatus
	}

	return conv, nil
}

// FindActiveConversationBySessionAndQR finds the latest active conversation for a session+QR pair
// Used in /scan to detect if the user already has an ongoing conversation
func (s *MessageService) FindActiveConversationBySessionAndQR(ctx context.Context, sessionID string, qrCodeID string) (*models.Conversation, error) {
	return s.conversations.FindActiveBySessionAndQR(ctx, sessionID, qrCodeID)
}
