package handlers

import (
	"net/http"
	"time"

	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
)

var reminderTimeLocation = func() *time.Location {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("GMT+7", 7*60*60)
	}

	return location
}()

type ReminderHandler struct {
	service *services.ReminderService
}

func NewReminderHandler(service *services.ReminderService) *ReminderHandler {
	return &ReminderHandler{service: service}
}

// CreateReminder handles POST /conversations/:id/reminder.
func (h *ReminderHandler) CreateReminder(c *gin.Context) {
	sessionIDValue, _ := c.Get(middleware.SessionIDKey)
	ipAddressValue, _ := c.Get(middleware.IPAddressKey)

	result, err := h.service.SendReminder(c.Request.Context(), models.ReminderRequest{
		ConversationID: c.Param("id"),
		SessionID:      stringValue(sessionIDValue),
		IPAddress:      stringValue(ipAddressValue),
	})
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ReminderResponse{
			Success: false,
			Reason:  string(models.ReminderReasonUnavailable),
		})
		return
	}

	response := models.ReminderResponse{
		Success:           result.Success,
		RemainingReminder: result.RemainingReminder,
	}

	if result.Success {
		response.Message = result.Message
		if result.NextAllowedAt != nil {
			response.NextAllowedAt = stringPtr(formatReminderTime(*result.NextAllowedAt))
		}
		c.JSON(http.StatusOK, response)
		return
	}

	response.Reason = string(result.Reason)
	if result.NextAllowedAt != nil {
		response.NextAllowedAt = stringPtr(formatReminderTime(*result.NextAllowedAt))
	}

	if result.Reason == models.ReminderReasonInvalidConversation {
		c.JSON(http.StatusNotFound, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

func stringPtr(value string) *string {
	return &value
}

func stringValue(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}

	return ""
}

func formatReminderTime(value time.Time) string {
	return value.In(reminderTimeLocation).Format(time.RFC3339)
}

