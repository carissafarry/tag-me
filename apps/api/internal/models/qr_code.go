package models

import (
	"github.com/google/uuid"
)


// GenerateQRRequest is the payload for POST /api/v1/qrcode/generate
type GenerateQRRequest struct {
	ObjectID uuid.UUID `json:"object_id" binding:"required"`
}

// GenerateQRResponse is returned after successfully generating a QR code from GenerateQRRequest
type GenerateQRResponse struct {
	ID       uuid.UUID `json:"id"`
	QRToken  string    `json:"qr_token"`
	IsActive bool      `json:"is_active"`
	CreatedAt string   `json:"created_at"`
}

// GetQRResponse is returned by GET /api/v1/qrcode/:object_id
type GetQRResponse struct {
	ID       uuid.UUID `json:"id"`
	QRToken  string    `json:"qr_token"`
	ObjectID uuid.UUID `json:"object_id"`
	IsActive bool      `json:"is_active"`
	CreatedAt string   `json:"created_at"`
}
