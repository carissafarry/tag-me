package repository

import (
	"context"

	"github.com/carissafarry/tag-me/api/internal/models"
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
		`SELECT id, owner_id, qr_token, object_type, object_id, is_active, plate
		 FROM qr_codes
		 WHERE qr_token = $1`,
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
