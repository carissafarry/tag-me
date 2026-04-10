package repository

import (
	"context"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, message *models.Message) (*models.Message, error) {
	stored := &models.Message{}

	err := r.db.QueryRow(
		ctx,
		`INSERT INTO messages (id, conversation_id, sender_type, message_type, content, location_latitude, location_longitude, location_text, session_id, ip_address, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'))
		 RETURNING id, conversation_id, sender_type, message_type, content, location_latitude, location_longitude, location_text, session_id, ip_address::text, created_at`,
		message.ID,
		message.ConversationID,
		message.SenderType,
		message.MessageType,
		message.Content,
		message.LocationLatitude,
		message.LocationLongitude,
		message.LocationText,
		message.SessionID,
		message.IPAddress,
	).Scan(
		&stored.ID,
		&stored.ConversationID,
		&stored.SenderType,
		&stored.MessageType,
		&stored.Content,
		&stored.LocationLatitude,
		&stored.LocationLongitude,
		&stored.LocationText,
		&stored.SessionID,
		&stored.IPAddress,
		&stored.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	stored.ConversationID = message.ConversationID

	return stored, nil
}
