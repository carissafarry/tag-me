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

type QRCodeRepository interface {
	FindByToken(ctx context.Context, qrToken string) (*models.QRCode, error)
}

type ConversationRepository interface {
	Create(ctx context.Context, conversation *models.Conversation) (*models.Conversation, error)
	FindByID(ctx context.Context, conversationID string) (*models.Conversation, error)
}

type MessageRepository interface {
	Create(ctx context.Context, message *models.Message) (*models.Message, error)
}
}

func NewMessageService(db *pgxpool.Pool) *MessageService {
	return &MessageService{db: db}
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
func (s *MessageService) CreateConversation(ctx context.Context, qrCode *models.QRCode) (*models.Conversation, error) {
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

// GetConversationStatus retrieves the current status of a conversation
// Validates that status is in the allowed set (AC5: status values map correctly from owner actions)
func (s *MessageService) GetConversationStatus(ctx context.Context, conversationID string) (*models.Conversation, error) {
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
