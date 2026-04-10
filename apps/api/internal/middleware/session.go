package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/carissafarry/tag-me/api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	SessionCookieName = config.SessionCookieName
	SessionIDHeader   = config.SessionIDHeader
	IPAddressKey      = "ip_address"
	SessionIDKey      = "session_id"
	SessionTTL        = config.SessionTTL
)

// SessionTracking middleware extracts IP address and generates/tracks session ID
func SessionTracking() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract client IP
		ipAddress := extractClientIP(c.Request.Header.Get("X-Forwarded-For"))
		if ipAddress == "" {
			ipAddress = c.ClientIP()
		}

		// Prefer the cookie session, but keep header fallback for compatible clients and tests.
		sessionID, err := c.Cookie(SessionCookieName)
		if err != nil || sessionID == "" {
			sessionID = c.Request.Header.Get(SessionIDHeader)
		}
		if sessionID == "" {
			sessionID = uuid.New().String()
		}

		// Store in context for downstream handlers
		c.Set(IPAddressKey, ipAddress)
		c.Set(SessionIDKey, sessionID)

		// Preserve the header for compatibility while setting the cookie as the source of truth.
		c.Header(SessionIDHeader, sessionID)
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(
			SessionCookieName,
			sessionID,
			int(SessionTTL.Seconds()),
			"/",
			"",
			c.Request.TLS != nil,
			true,
		)

		c.Next()
	}
}

// extractClientIP parses X-Forwarded-For header and returns the first IP
func extractClientIP(xForwardedFor string) string {
	if xForwardedFor == "" {
		return ""
	}

	// X-Forwarded-For can be comma-separated list of IPs
	ips := strings.Split(xForwardedFor, ",")
	if len(ips) > 0 {
		ip := strings.TrimSpace(ips[0])
		// Validate it's a real IP
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
}
