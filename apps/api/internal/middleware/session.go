package middleware

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	SessionIDHeader = "X-Session-ID"
	IPAddressKey    = "ip_address"
	SessionIDKey    = "session_id"
)

// SessionTracking middleware extracts IP address and generates/tracks session ID
func SessionTracking() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract client IP
		ipAddress := extractClientIP(c.Request.Header.Get("X-Forwarded-For"))
		if ipAddress == "" {
			ipAddress = c.ClientIP()
		}

		// Generate or retrieve session ID
		sessionID := c.Request.Header.Get(SessionIDHeader)
		if sessionID == "" {
			sessionID = uuid.New().String()
		}

		// Store in context for downstream handlers
		c.Set(IPAddressKey, ipAddress)
		c.Set(SessionIDKey, sessionID)

		// Add session ID to response header
		c.Header(SessionIDHeader, sessionID)

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
