package models

import (
	"time"

	"github.com/google/uuid"
)

// Conversation represents a group of messages around a QR code scan
type Conversation struct {
	ID        uuid.UUID `json:"id"`
	QRCodeID  uuid.UUID `json:"qr_code_id"`
	OwnerID   uuid.UUID `json:"-"` // Never expose to scanner
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents an individual scanner message
type Message struct {
	ID                uuid.UUID `json:"id"`
	ConversationID    uuid.UUID `json:"conversation_id"`
	SenderType        string    `json:"sender_type"`
	MessageType       string    `json:"message_type"`
	Content           *string   `json:"content,omitempty"`
	LocationLatitude  *float64  `json:"location_latitude,omitempty"`
	LocationLongitude *float64  `json:"location_longitude,omitempty"`
	LocationText      *string   `json:"location_text,omitempty"`
	SessionID         *string   `json:"-"` // Never expose to scanner
	IPAddress         *string   `json:"-"` // Never expose to scanner
	CreatedAt         time.Time `json:"created_at"`
}

// QRCode maps qr_token to owner and object context
type QRCode struct {
	ID         uuid.UUID `json:"-"`
	OwnerID    uuid.UUID `json:"-"`
	QRToken    string    `json:"-"`
	ObjectType string    `json:"-"`
	ObjectID   uuid.UUID `json:"-"`
	IsActive   bool      `json:"-"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

// CreateMessageRequest is the payload for POST /messages
type CreateMessageRequest struct {
	QRToken           string   `json:"qr_token" binding:"required"`
	MessageType       string   `json:"message_type" binding:"required"`
	Content           *string  `json:"content,omitempty"`
	LocationLatitude  *float64 `json:"location_latitude,omitempty"`
	LocationLongitude *float64 `json:"location_longitude,omitempty"`
	LocationText      *string  `json:"location_text,omitempty"`
}

// CreateMessageResponse is returned after successfully creating a message
type CreateMessageResponse struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
}

// ConversationStatusResponse is returned by GET /conversations/:id/status
type ConversationStatusResponse struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
}

// ErrorResponse is a safe error returned to scanner clients
type ErrorResponse struct {
	Error  string `json:"error"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}
