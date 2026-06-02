package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/carissafarry/tag-me/api/internal/config"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/services"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// RequestOTP handles POST /auth/request-otp
// Returns 200 on success, 400 on invalid input, 429 on rate limit
func (h *AuthHandler) RequestOTP(c *gin.Context) {
	var req services.OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: services.ErrInvalidRequest.Error(),
			Code:  "validation_error",
		})
		return
	}

	otpCode, err := h.authService.RequestOTP(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case services.ErrContactRequired, services.ErrContactTypeRequired, services.ErrInvalidContact:
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error: err.Error(),
				Code:  "validation_error",
			})
		case services.ErrTooManyRequests:
			// Rate limited (cooldown or hourly limit)
			c.Header("Retry-After", "180") // 3 minutes
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to request otp"})
		}
		return
	}

	response := gin.H{"message": "otp sent successfully"}
	// In dev, include OTP code for testing
	cfg := config.Get()
	if cfg.Environment == "development" {
		response["otp_code"] = otpCode
	}
	c.JSON(http.StatusOK, response)
}

// VerifyOTP handles POST /auth/verify-otp
// Returns 200 on success, 400 on expired OTP, 401 on invalid OTP, 403 on account locked
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req services.OTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: services.ErrInvalidRequest.Error(),
			Code:  "validation_error",
		})
		return
	}

	resp, err := h.authService.VerifyOTP(c.Request.Context(), &req)
	if err != nil {
		switch err {
		case services.ErrContactRequired, services.ErrOTPRequired:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case services.ErrOTPNotFound:
			// OTP expired or not found
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "OTP_NOT_FOUND"})
		case services.ErrInvalidOTP:
			// Invalid OTP (mismatch), allow retry
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "code": "INVALID_OTP"})
		case services.ErrAccountBlocked:
			// Account locked permanently
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error(), "code": "ACCOUNT_LOCKED"})
		case services.ErrAccountDisabled:
			// Account disabled
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error(), "code": "ACCOUNT_DISABLED"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify otp"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}
