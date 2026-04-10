package repository

import (
	"context"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversationRepository struct {
	db *pgxpool.Pool
}

func NewConversationRepository(db *pgxpool.Pool) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, conversation *models.Conversation) (*models.Conversation, error) {
	stored := &models.Conversation{}

	err := r.db.QueryRow(
		ctx,
		`INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'), date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'))
		 RETURNING id, qr_code_id, owner_id, status, created_at, updated_at`,
		conversation.ID,
		conversation.QRCodeID,
		conversation.OwnerID,
		conversation.Status,
	).Scan(
		&stored.ID,
		&stored.QRCodeID,
		&stored.OwnerID,
		&stored.Status,
		&stored.CreatedAt,
		&stored.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return stored, nil
}

func (r *ConversationRepository) FindByID(ctx context.Context, conversationID string) (*models.Conversation, error) {
	conversation := &models.Conversation{}

	err := r.db.QueryRow(
		ctx,
		`SELECT id, qr_code_id, owner_id, status, created_at, updated_at
		 FROM conversations
		 WHERE id = $1`,
		conversationID,
	).Scan(
		&conversation.ID,
		&conversation.QRCodeID,
		&conversation.OwnerID,
		&conversation.Status,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return conversation, nil
}
