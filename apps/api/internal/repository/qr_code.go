package repository

import (
	"context"

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

func (r *QRCodeRepository) Create(ctx context.Context, ownerID uuid.UUID, objectID uuid.UUID, qrToken string) (*models.QRCode, error) {
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

