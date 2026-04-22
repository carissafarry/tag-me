package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/carissafarry/tag-me/api/internal/models"
)

type OwnerRepository struct {
	db *pgxpool.Pool
}

func NewOwnerRepository(db *pgxpool.Pool) *OwnerRepository {
	return &OwnerRepository{db: db}
}

// UpsertByContact creates owner if not exists, returns existing owner if already exists.
func (r *OwnerRepository) UpsertByContact(ctx context.Context, contact, contactType string) (*models.Owner, error) {
	owner := &models.Owner{}
	query := `
		INSERT INTO owners (contact, contact_type, dnd_enabled, is_active, created_at, updated_at)
		VALUES ($1, $2, false, true, NOW(), NOW())
		ON CONFLICT (contact) DO UPDATE
		SET updated_at = NOW()
		RETURNING id, contact, contact_type, dnd_enabled, is_active, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, contact, contactType).Scan(
		&owner.ID,
		&owner.Contact,
		&owner.ContactType,
		&owner.DNDEnabled,
		&owner.IsActive,
		&owner.CreatedAt,
		&owner.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return owner, nil
}

// GetByContact retrieves owner by contact (phone/email).
func (r *OwnerRepository) GetByContact(ctx context.Context, contact string) (*models.Owner, error) {
	owner := &models.Owner{}
	query := `SELECT id, contact, contact_type, dnd_enabled, is_active, created_at, updated_at FROM owners WHERE contact = $1`
	err := r.db.QueryRow(ctx, query, contact).Scan(
		&owner.ID,
		&owner.Contact,
		&owner.ContactType,
		&owner.DNDEnabled,
		&owner.IsActive,
		&owner.CreatedAt,
		&owner.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return owner, nil
}

// GetByID retrieves owner by ID.
func (r *OwnerRepository) GetByID(ctx context.Context, id string) (*models.Owner, error) {
	owner := &models.Owner{}
	query := `SELECT id, contact, contact_type, dnd_enabled, is_active, created_at, updated_at FROM owners WHERE id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&owner.ID,
		&owner.Contact,
		&owner.ContactType,
		&owner.DNDEnabled,
		&owner.IsActive,
		&owner.CreatedAt,
		&owner.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return owner, nil
}

// DisableAccount sets is_active = false for an owner (permanent account lock).
func (r *OwnerRepository) DisableAccount(ctx context.Context, contact string) error {
	query := `UPDATE owners SET is_active = false, updated_at = NOW() WHERE contact = $1`
	_, err := r.db.Exec(ctx, query, contact)
	return err
}
