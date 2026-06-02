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
