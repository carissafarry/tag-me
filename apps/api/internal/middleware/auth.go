package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/carissafarry/tag-me/api/internal/config"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
)

const (
	UserIDHeader = config.UserIDHeader
	UserIDKey    = "user_id"

	INVALID_CREDENTIALS = "Invalid credentials"
	TOKEN_ERROR         = "Error parsing token"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authClaims, ok := validateToken(c)
		if !ok {
			return
		}

		c.Set(UserIDKey, authClaims.OwnerID)
		c.Header(UserIDHeader, authClaims.OwnerID)
		c.Next()
	}
}

func validateToken(c *gin.Context) (services.AuthClaims, bool) {
	// Get token from header
	auth := c.GetHeader("Authorization")
	if auth == "" {
		unauthorizedResponse(c, INVALID_CREDENTIALS)
		return services.AuthClaims{}, false
	}

	// Extract bearer token
	bearer := strings.Split(auth, " ")
	if len(bearer) != 2 || bearer[0] != "Bearer" {
		unauthorizedResponse(c, INVALID_CREDENTIALS)
		return services.AuthClaims{}, false
	}

	// Parse JWT by "."
	token := bearer[1]
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) != 3 {
		unauthorizedResponse(c, INVALID_CREDENTIALS)
		return services.AuthClaims{}, false
	}

	// Decode AuthClaims (encoded in auth_service.go)
	claims := tokenParts[1]
	claimsJSON, err := base64.RawURLEncoding.DecodeString(claims)
	if err != nil {
		unauthorizedResponse(c, TOKEN_ERROR)
		return services.AuthClaims{}, false
	}

	// Extract JSON AuthClaims
	var authClaims services.AuthClaims
	err = json.Unmarshal(claimsJSON, &authClaims)
	if err != nil {
		unauthorizedResponse(c, TOKEN_ERROR)
		return services.AuthClaims{}, false
	}
	return authClaims, true
}

func unauthorizedResponse(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": message, "code": "invalid_auth"})
	c.Abort()
}
