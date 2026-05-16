package models

import (
	"time"

	"github.com/google/uuid"
)

type Owner struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Contact     string    `db:"contact" json:"contact"`
	ContactType string    `db:"contact_type" json:"contact_type"`
	DNDEnabled  bool      `db:"dnd_enabled" json:"dnd_enabled"`
	IsActive    bool      `db:"is_active" json:"is_active"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
