package handlers

import (
	"errors"
	"time"

	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var jakartaLocation = func() *time.Location {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("GMT+7", 7*60*60)
	}

	return location
}()

func contextString(c *gin.Context, key string) string {
	value, exists := c.Get(key)
	if !exists {
		return ""
	}

	text, _ := value.(string)
	return text
}

func getUserIDUUID(c *gin.Context) (uuid.UUID, error) {
	ownerIDVal, ok := c.Get(middleware.UserIDKey)
	if !ok {
		return uuid.UUID{}, errors.New("user_id not in context")
	}

	ownerIDStr, ok := ownerIDVal.(string)
	if !ok {
		return uuid.UUID{}, errors.New("user_id not a string")
	}

	return uuid.Parse(ownerIDStr)
}

func stringPtr(value string) *string {
	return &value
}

func formatJakartaTime(value time.Time) string {
	return value.In(jakartaLocation).Format("2006-01-02 15:04:05")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(jakartaLocation).Format("2006-01-02 15:04:05")
}
