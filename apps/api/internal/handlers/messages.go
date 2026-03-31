package handlers

import (
	"net/http"

	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	service *services.MessageService
}

func NewMessageHandler(service *services.MessageService) *MessageHandler {
	return &MessageHandler{service: service}
}

// CreateMessage handles POST /messages
// Accepts: qr_token, message_type, optional content, optional location
// Creates conversation and initial message
// Returns: conversation_id, message_id, status, created_at
// Never exposes owner contact data to scanner
func (h *MessageHandler) CreateMessage(c *gin.Context) {
	var req models.CreateMessageRequest

	// Validate request payload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid request payload",
			Code:  "validation_error",
		})
		return
	}

	// Get session metadata from context (set by middleware)
	sessionID, _ := c.Get(middleware.SessionIDKey)
	ipAddress, _ := c.Get(middleware.IPAddressKey)

	sessionIDStr := sessionID.(string)
	ipAddressStr := ipAddress.(string)

	// Convert empty IP to nil (database INET type doesn't accept empty strings)
	var ipAddressPtr *string
	if ipAddressStr != "" {
		ipAddressPtr = &ipAddressStr
	}

	// Resolve QR token to owner and object context
	qrCode, err := h.service.ResolveQRToken(c.Request.Context(), req.QRToken)
	if err != nil {
		// Return safe error - don't leak whether token exists
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "invalid or inactive qr code",
			Code:  "invalid_qr_token",
		})
		return
	}

	// Create conversation record
	conversation, err := h.service.CreateConversation(c.Request.Context(), qrCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:  "failed to create conversation",
			Code:   "database_error",
			Detail: err.Error(),
		})
		return
	}

	// Create initial message record
	message, err := h.service.CreateMessage(
		c.Request.Context(),
		conversation.ID,
		req.MessageType,
		req.Content,
		req.LocationLatitude,
		req.LocationLongitude,
		req.LocationText,
		&sessionIDStr,
		ipAddressPtr,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:  "failed to create message",
			Code:   "database_error",
			Detail: err.Error(),
		})
		return
	}

	// Return response with conversation ID for status tracking
	// Exclude owner contact and session metadata
	response := models.CreateMessageResponse{
		ConversationID: conversation.ID.String(),
		MessageID:      message.ID.String(),
		Status:         conversation.Status,
		CreatedAt:      message.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	c.JSON(http.StatusCreated, response)
}
