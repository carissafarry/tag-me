package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
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

func stringPtr(value string) *string {
	return &value
}

func formatJakartaRFC3339(value time.Time) string {
	return value.In(jakartaLocation).Format(time.RFC3339)
}
