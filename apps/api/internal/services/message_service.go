package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidQRToken   = errors.New("invalid qr token")
	ErrInactiveFQRToken = errors.New("qr token inactive")
	ErrDatabaseError    = errors.New("database error")
	ErrInvalidStatus    = errors.New("invalid conversation status")
)

// AllowedConversationStatuses defines the valid status states from TAG-8
var AllowedConversationStatuses = map[string]bool{
	"PENDING":    true,
	"DELIVERED": true,
	"OPENED":    true,
	"ON_THE_WAY": true,
	"RESOLVED":   true,
}

type MessageService struct {
	db *pgxpool.Pool
}

func NewMessageService(db *pgxpool.Pool) *MessageService {
	return &MessageService{db: db}
}

// ResolveQRToken validates qr_token and returns owner and object context
func (s *MessageService) ResolveQRToken(ctx context.Context, qrToken string) (*models.QRCode, error) {
	if qrToken == "" {
		return nil, ErrInvalidQRToken
	}

	qr := &models.QRCode{}
	err := s.db.QueryRow(
		ctx,
		`SELECT id, owner_id, qr_token, object_type, object_id, is_active
		 FROM qr_codes WHERE qr_token = $1`,
		qrToken,
	).Scan(
		&qr.ID,
		&qr.OwnerID,
		&qr.QRToken,
		&qr.ObjectType,
		&qr.ObjectID,
		&qr.IsActive,
	)

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

	err := s.db.QueryRow(
		ctx,
		`INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'), date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'))
		 RETURNING id, qr_code_id, owner_id, status, created_at, updated_at`,
		conv.ID,
		conv.QRCodeID,
		conv.OwnerID,
		conv.Status,
	).Scan(
		&conv.ID,
		&conv.QRCodeID,
		&conv.OwnerID,
		&conv.Status,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseError, err)
	}

	return conv, nil
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

	err := s.db.QueryRow(
		ctx,
		`INSERT INTO messages (id, conversation_id, sender_type, message_type, content, location_latitude, location_longitude, location_text, session_id, ip_address, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'))
		 RETURNING id, conversation_id, sender_type, message_type, content, location_latitude, location_longitude, location_text, session_id, ip_address::text, created_at`,
		msg.ID,
		msg.ConversationID,
		msg.SenderType,
		msg.MessageType,
		msg.Content,
		msg.LocationLatitude,
		msg.LocationLongitude,
		msg.LocationText,
		msg.SessionID,
		msg.IPAddress,
	).Scan(
		&msg.ID,
		&msg.ConversationID,
		&msg.SenderType,
		&msg.MessageType,
		&msg.Content,
		&msg.LocationLatitude,
		&msg.LocationLongitude,
		&msg.LocationText,
		&msg.SessionID,
		&msg.IPAddress,
		&msg.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseError, err)
	}

	return msg, nil
}

// GetConversationStatus retrieves the current status of a conversation
// Validates that status is in the allowed set (AC5: status values map correctly from owner actions)
func (s *MessageService) GetConversationStatus(ctx context.Context, conversationID string) (*models.Conversation, error) {
	conv := &models.Conversation{}
	err := s.db.QueryRow(
		ctx,
		`SELECT id, status, created_at FROM conversations WHERE id = $1`,
		conversationID,
	).Scan(
		&conv.ID,
		&conv.Status,
		&conv.CreatedAt,
	)

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
