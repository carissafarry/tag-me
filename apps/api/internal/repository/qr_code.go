package repository

import (
	"context"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QRCodeRepository struct {
	db *pgxpool.Pool
}

func NewQRCodeRepository(db *pgxpool.Pool) *QRCodeRepository {
	return &QRCodeRepository{db: db}
}

func (r *QRCodeRepository) FindByObjectID(ctx context.Context, ownerID uuid.UUID, objectID uuid.UUID) (*models.QRCode, error) {
	qr := &models.QRCode{}

	err := r.db.QueryRow(
		ctx,
		`SELECT qc.id, qc.owner_id, qc.qr_token, o.object_type, qc.object_id, qc.is_active, o.plate, qc.created_at, qc.updated_at
		 FROM qr_codes qc
		 LEFT JOIN objects o ON qc.object_id = o.id
		 WHERE qc.object_id = $1 AND qc.owner_id = $2`,
		objectID,
		ownerID,
	).Scan(
		&qr.ID,
		&qr.OwnerID,
		&qr.QRToken,
		&qr.ObjectType,
		&qr.ObjectID,
		&qr.IsActive,
		&qr.Plate,
		&qr.CreatedAt,
		&qr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return qr, nil
}

func (r *QRCodeRepository) FindByToken(ctx context.Context, qrToken string) (*models.QRCode, error) {
	qr := &models.QRCode{}

	err := r.db.QueryRow(
		ctx,
		`SELECT qc.id, qc.owner_id, qc.qr_token, o.object_type, qc.object_id, qc.is_active, o.plate
		 FROM qr_codes qc
		 LEFT JOIN objects o ON qc.object_id = o.id
		 WHERE LOWER(qc.qr_token) = LOWER($1)`,
		qrToken,
	).Scan(
		&qr.ID,
		&qr.OwnerID,
		&qr.QRToken,
		&qr.ObjectType,
		&qr.ObjectID,
		&qr.IsActive,
		&qr.Plate,
	)
	if err != nil {
		return nil, err
	}

	return qr, nil
}

func (r *QRCodeRepository) CreateToken(ctx context.Context, ownerID uuid.UUID, objectID uuid.UUID, qrToken string) (*models.QRCode, error) {
	id := uuid.New()

	_, err := r.db.Exec(
		ctx,
		`INSERT INTO qr_codes (id, owner_id, object_id, qr_token, is_active)
		 VALUES ($1, $2, $3, $4, $5)`,
		id,
		ownerID,
		objectID,
		qrToken,
		true,
	)
	if err != nil {
		return nil, err
	}

	// Fetch the created QR code with joined object data
	return r.FindByObjectID(ctx, ownerID, objectID)
}

func (r *QRCodeRepository) UpdateToken(ctx context.Context, objectID uuid.UUID, newToken string) (*models.QRCode, error) {
	qr := &models.QRCode{}

	err := r.db.QueryRow(
		ctx,
		`UPDATE qr_codes
		 SET qr_token = $1, updated_at = NOW()
		 WHERE object_id = $2
		 RETURNING id, owner_id, qr_token, object_id, is_active, created_at, updated_at`,
		newToken,
		objectID,
	).Scan(
		&qr.ID,
		&qr.OwnerID,
		&qr.QRToken,
		&qr.ObjectID,
		&qr.IsActive,
		&qr.CreatedAt,
		&qr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return qr, nil
}

// GenerateQRImage creates a QR Code, saves as PNG, and returns base64-encoded string
func (r *QRCodeRepository) GenerateQRImage(ctx context.Context, data string, filePath string) (string, error) {
	// Return if file already exists
	if _, err := os.Stat(filePath); err == nil {
		imageBytes, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read QR image: %w", err)
		}
		return base64.StdEncoding.EncodeToString(imageBytes), nil
	}

	// Encode data into a QR code
	qrCode, err := qr.Encode(data, qr.M, qr.Auto)
	if err != nil {
		return "", fmt.Errorf("failed to encode QR code: %w", err)
	}

	// Scale QR code (returns image.Image)
	scaledQR, err := barcode.Scale(qrCode, 300, 300)
	if err != nil {
		return "", fmt.Errorf("failed to scale QR code: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file to save the QR code image
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Encode directly as PNG
	err = png.Encode(file, scaledQR)
	if err != nil {
		return "", fmt.Errorf("failed to encode QR code image: %w", err)
	}

	// Read the file and return as base64
	imageBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read generated QR image: %w", err)
	}

	return base64.StdEncoding.EncodeToString(imageBytes), nil
}