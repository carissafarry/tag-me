package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Conversation represents a group of messages around a QR code scan
type Conversation struct {
	ID         uuid.UUID  `json:"id"`
	QRCodeID   uuid.UUID  `json:"qr_code_id"`
	OwnerID    uuid.UUID  `json:"-"` // Never expose to scanner
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"-"`
	OpenedAt   *time.Time `json:"-"`
	OnTheWayAt *time.Time `json:"-"`
	ResolvedAt *time.Time `json:"-"`
	CreatedAt  time.Time  `json:"-"`
	UpdatedAt  time.Time  `json:"-"`
}

// MarshalJSON formats timestamps as "2006-01-02 15:04:05"
func (c *Conversation) MarshalJSON() ([]byte, error) {
	type Alias Conversation
	return json.Marshal(&struct {
		*Alias
		ExpiresAt  string `json:"expires_at,omitempty"`
		OpenedAt   string `json:"opened_at,omitempty"`
		OnTheWayAt string `json:"on_the_way_at,omitempty"`
		ResolvedAt string `json:"resolved_at,omitempty"`
		CreatedAt  string `json:"created_at"`
		UpdatedAt  string `json:"updated_at"`
	}{
		Alias:      (*Alias)(c),
		ExpiresAt:  formatTimePtr(c.ExpiresAt),
		OpenedAt:   formatTimePtr(c.OpenedAt),
		OnTheWayAt: formatTimePtr(c.OnTheWayAt),
		ResolvedAt: formatTimePtr(c.ResolvedAt),
		CreatedAt:  c.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:  c.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
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
	Plate      *string   `json:"-"`
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
	CanFollowUp    bool   `json:"can_follow_up"`
}

// ScanResponse is returned by GET /scan?token=xxx
type ScanResponse struct {
	Plate          *string `json:"plate,omitempty"`
	ConversationID *string `json:"conversation_id,omitempty"`
	HasActive      bool    `json:"has_active"`
}
