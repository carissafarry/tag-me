package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Object struct {
	ID         uuid.UUID `db:"id" json:"id"`
	OwnerID    uuid.UUID `db:"owner_id" json:"-"`
	Name       string    `db:"name" json:"name"`
	ObjectType string    `db:"object_type" json:"object_type"`
	Plate      *string   `db:"plate" json:"plate"`
	CreatedAt  time.Time `db:"created_at" json:"-"`
	UpdatedAt  time.Time `db:"updated_at" json:"-"`
}

func (o *Object) MarshalJSON() ([]byte, error) {
	type Alias Object
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{
		Alias:     (*Alias)(o),
		CreatedAt: formatJakartaTime(o.CreatedAt),
		UpdatedAt: formatJakartaTime(o.UpdatedAt),
	})
}

type CreateObjectRequest struct {
	Name       string  `json:"name" binding:"required,max=255"`
	ObjectType string  `json:"object_type" binding:"required,oneof=car motorcycle luggage key device other"`
	Plate      *string `json:"plate" binding:"omitempty,max=10"`
}

var AllowedObjectTypes = map[string]bool{
	"car":        true,
	"motorcycle": true,
	"luggage":    true,
	"key":	      true,
	"device":     true,
	"other":      true,
}
