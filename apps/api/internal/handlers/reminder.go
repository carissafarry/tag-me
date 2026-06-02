package handlers

import (
	"net/http"

	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
)

type ReminderHandler struct {
	service *services.ReminderService
}

func NewReminderHandler(service *services.ReminderService) *ReminderHandler {
	return &ReminderHandler{service: service}
}

// CreateReminder handles POST /conversations/:id/reminder.
func (h *ReminderHandler) CreateReminder(c *gin.Context) {
	result, err := h.service.SendReminder(c.Request.Context(), models.ReminderRequest{
		ConversationID: c.Param("id"),
		SessionID:      contextString(c, middleware.SessionIDKey),
		IPAddress:      contextString(c, middleware.IPAddressKey),
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
			response.NextAllowedAt = stringPtr(formatTimePtr(result.NextAllowedAt))
		}
		c.JSON(http.StatusOK, response)
		return
	}

	response.Reason = string(result.Reason)
	if result.NextAllowedAt != nil {
		response.NextAllowedAt = stringPtr(formatTimePtr(result.NextAllowedAt))
	}

	if result.Reason == models.ReminderReasonInvalidConversation {
		c.JSON(http.StatusNotFound, response)
		return
	}

	c.JSON(http.StatusOK, response)
}
