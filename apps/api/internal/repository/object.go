package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ObjectRepository struct {
	db *pgxpool.Pool
}

func NewObjectRepository(db *pgxpool.Pool) *ObjectRepository {
	return &ObjectRepository{db: db}
}

func (r *ObjectRepository) Create(ctx context.Context, object *models.Object) error {
	query := `
		INSERT INTO objects (id, owner_id, name, object_type, plate, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, owner_id, name, object_type, plate, created_at, updated_at
	`
	return r.db.QueryRow(
		ctx, query,
		object.ID, 
		object.OwnerID, 
		object.Name, 
		object.ObjectType, 
		object.Plate,
		object.CreatedAt, 
		object.UpdatedAt,
	).Scan(
		&object.ID, 
		&object.OwnerID, 
		&object.Name, 
		&object.ObjectType, 
		&object.Plate,
		&object.CreatedAt, 
		&object.UpdatedAt,
	)
}

func (r *ObjectRepository) FindAll(ctx context.Context, ownerID uuid.UUID, filters map[string]string) ([]models.Object, error) {
	query := `
		SELECT id, owner_id, name, object_type, plate, created_at, updated_at
		FROM objects
		WHERE owner_id = $1
	`
	args := []any{ownerID}
	paramCount := 2

	if name, ok := filters["name"]; ok {
		addRepoFilter(&query, &args, "name", "%"+name+"%", &paramCount)
	}
	if objectType, ok := filters["object_type"]; ok {
		addRepoFilter(&query, &args, "object_type", objectType, &paramCount)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query objects: %w", err)
	}
	defer rows.Close()

	var objects []models.Object
	for rows.Next() {
		var obj models.Object
		if err := rows.Scan(
			&obj.ID, 
			&obj.OwnerID, 
			&obj.Name, 
			&obj.ObjectType, 
			&obj.Plate,
			&obj.CreatedAt, 
			&obj.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan object: %w", err)
		}
		objects = append(objects, obj)
	}

	return objects, rows.Err()
}

func (r *ObjectRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) (*models.Object, error) {
	query := `
		SELECT id, owner_id, name, object_type, plate, created_at, updated_at
		FROM objects
		WHERE id = $1 AND owner_id = $2
	`
	var obj models.Object
	err := r.db.QueryRow(ctx, query, id, ownerID).Scan(
		&obj.ID, &obj.OwnerID, &obj.Name, &obj.ObjectType, &obj.Plate,
		&obj.CreatedAt, &obj.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query object: %w", err)
	}
	return &obj, nil
}

func (r *ObjectRepository) Delete(ctx context.Context, id uuid.UUID, ownerID uuid.UUID) error {
	query := `DELETE FROM objects WHERE id = $1 AND owner_id = $2`
	_, err := r.db.Exec(ctx, query, id, ownerID)
	return err
}

func (r *ObjectRepository) HasActiveConversations(ctx context.Context, objectID uuid.UUID, ownerID uuid.UUID) (bool, error) {
	query := `
		SELECT COUNT(*) FROM conversations c
		INNER JOIN qr_codes q 
		ON c.qr_code_id = q.id
		WHERE 
			q.object_id = $1 AND c.owner_id = $2
		AND c.status NOT IN ('RESOLVED', 'EXPIRED')
	`
	var count int64
	err := r.db.QueryRow(
		ctx, query, 
		objectID, 
		ownerID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check active conversations: %w", err)
	}
	return count > 0, nil
}
