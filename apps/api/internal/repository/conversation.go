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
