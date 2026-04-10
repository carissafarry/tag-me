package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
)

type MessageTracker interface {
	TrackMessage(ctx context.Context, sessionID string, qrID string, now time.Time) (*models.MessageState, error)
}

type MessageHandler struct {
	service        *services.MessageService
	messageTracker MessageTracker
}

func NewMessageHandler(service *services.MessageService) *MessageHandler {
	return &MessageHandler{service: service}
}

func NewMessageHandlerWithTracker(service *services.MessageService, messageTracker MessageTracker) *MessageHandler {
	return &MessageHandler{
		service:        service,
		messageTracker: messageTracker,
	}
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
	sessionIDStr := contextString(c, middleware.SessionIDKey)
	ipAddressStr := contextString(c, middleware.IPAddressKey)

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
	conversation, err := h.service.CreateConversation(c.Request.Context(), qrCode, sessionIDStr, ipAddressStr)
	if err != nil {
		var rateLimitErr *services.ConversationRateLimitError
		switch {
		case errors.As(err, &rateLimitErr):
			// DAILY_LIMIT
			if rateLimitErr.Reason == "DAILY_LIMIT" {
                c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
                    Error: "you have reached the daily message limit for this QR",
                    Code:  "daily_limit_exceeded",
                })
                return
            }

			// COOLDOWN
			if rateLimitErr.RetryAfter > 0 {
				c.Header("Retry-After", strconv.FormatInt(int64(rateLimitErr.RetryAfter/time.Second), 10))
			}
			c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
				Error: "you are creating conversations too quickly, please try again later",
				Code:  "rate_limited",
			})
			return
		case errors.Is(err, services.ErrMessageServiceUnavailable):
			c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
				Error: "message service temporarily unavailable",
				Code:  "service_unavailable",
			})
			return
		}

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

	if h.messageTracker != nil {
		if _, err := h.messageTracker.TrackMessage(c.Request.Context(), sessionIDStr, qrCode.ID.String(), time.Now().UTC()); err != nil {
			c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
				Error: "message tracking temporarily unavailable",
				Code:  "service_unavailable",
			})
			return
		}
	}

	// Return response with conversation ID for status tracking
	// Exclude owner contact and session metadata
	response := models.CreateMessageResponse{
		ConversationID: conversation.ID.String(),
		MessageID:      message.ID.String(),
		Status:         conversation.Status,
		CreatedAt:      formatJakartaRFC3339(message.CreatedAt),
	}

	c.JSON(http.StatusCreated, response)
}

// GetConversationStatus handles GET /conversations/:id/status
// Returns only scanner-safe fields: conversation_id, status, created_at
// Handles not found safely
func (h *MessageHandler) GetConversationStatus(c *gin.Context) {
	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "missing conversation id",
			Code:  "validation_error",
		})
		return
	}

	conversation, err := h.service.GetConversationStatus(c.Request.Context(), conversationID)
	if err != nil {
		// Return safe not found error - don't leak DB details
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "conversation not found",
			Code:  "not_found",
		})
		return
	}

	// Determine if follow-up is allowed
	// Allow follow-up if conversation is not resolved
	canFollowUp := conversation.Status != "RESOLVED"

	response := models.ConversationStatusResponse{
		ConversationID: conversation.ID.String(),
		Status:         conversation.Status,
		CreatedAt:      formatJakartaRFC3339(conversation.CreatedAt),
		CanFollowUp:    canFollowUp,
	}

	c.JSON(http.StatusOK, response)
}

// GetScan handles GET /scan?token=xxx
// Resolves QR token, returns plate info, and checks for active conversation
func (h *MessageHandler) GetScan(c *gin.Context) {
	qrToken := c.Query("token")
	if qrToken == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "missing qr token",
			Code:  "validation_error",
		})
		return
	}

	// Resolve QR token to get qr_id and plate
	qrCode, err := h.service.ResolveQRToken(c.Request.Context(), qrToken)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "qr code not found",
			Code:  "not_found",
		})
		return
	}

	response := models.ScanResponse{
		Plate: qrCode.Plate,
	}

	// Check for active conversation if session exists
	sessionIDStr := contextString(c, middleware.SessionIDKey)
	if sessionIDStr != "" {
		conversation, err := h.service.FindActiveConversationBySessionAndQR(c.Request.Context(), sessionIDStr, qrCode.ID.String())
		if err == nil {
			convID := conversation.ID.String()
			response.ConversationID = &convID
			response.HasActive = true
		}
	}

	c.JSON(http.StatusOK, response)
}
