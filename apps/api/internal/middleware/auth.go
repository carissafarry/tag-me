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
	UserIDKey = "user_id"

	INVALID_AUTH_TOKEN = "Invalid auth token"
	INVALID_USER_ID = "Invalid User ID"
	TOKEN_ERROR = "Error parsing token"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from header
		auth := c.GetHeader("Authorization")
		if auth == "" {
				unauthorizedResponse(c, INVALID_AUTH_TOKEN)
				return
		}

		// Extract bearer token
		bearer := strings.Split(auth, " ")
		if len(bearer) != 2 || bearer[0] != "Bearer" {
			unauthorizedResponse(c, INVALID_AUTH_TOKEN)
			return
		}

		// Parse JWT by "."
		token := bearer[1]
		tokenParts := strings.Split(token, ".")
		if len(tokenParts) != 3 {
			unauthorizedResponse(c, INVALID_AUTH_TOKEN)
			return
		}

		// Decode AuthClaims (encoded in auth_service.go)
		claims := tokenParts[1]
		claimsJSON, err := base64.RawURLEncoding.DecodeString(claims)
		if err != nil {
			unauthorizedResponse(c, TOKEN_ERROR)
			return
		}

		// Extract JSON AuthClaims 
		var authClaims services.AuthClaims
		err = json.Unmarshal(claimsJSON, &authClaims)
		if err != nil {
			unauthorizedResponse(c, TOKEN_ERROR)
			return
		}

		c.Set(UserIDKey, authClaims.OwnerID)
		c.Header(UserIDHeader, authClaims.OwnerID)
		c.Next()
	}
}

func unauthorizedResponse(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, gin.H{"error": message, "code": "invalid_auth"})
	c.Abort()
}