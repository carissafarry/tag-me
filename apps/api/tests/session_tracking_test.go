package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestSessionTrackingSetsContextKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.SessionTracking())

	router.GET("/session-tracking", func(c *gin.Context) {
		ipAddress, ipOK := c.Get(middleware.IPAddressKey)
		sessionID, sessionOK := c.Get(middleware.SessionIDKey)

		c.JSON(http.StatusOK, gin.H{
			"ip_address":      ipAddress,
			"ip_present":      ipOK,
			"session_id":      sessionID,
			"session_present": sessionOK,
		})
	})

	req, err := http.NewRequest(http.MethodGet, "/session-tracking", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	req.Header.Set("X-Session-ID", "custom-session-123")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if got := w.Header().Get(middleware.SessionIDHeader); got != "custom-session-123" {
		t.Fatalf("expected response header %s to be %q, got %q", middleware.SessionIDHeader, "custom-session-123", got)
	}

	var body struct {
		IPAddress      string `json:"ip_address"`
		IPPresent      bool   `json:"ip_present"`
		SessionID      string `json:"session_id"`
		SessionPresent bool   `json:"session_present"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	if !body.IPPresent {
		t.Fatal("expected ip_address key to be present in Gin context")
	}
	if !body.SessionPresent {
		t.Fatal("expected session_id key to be present in Gin context")
	}
	if body.IPAddress != "203.0.113.5" {
		t.Fatalf("expected ip_address to be %q, got %q", "203.0.113.5", body.IPAddress)
	}
	if body.SessionID != "custom-session-123" {
		t.Fatalf("expected session_id to be %q, got %q", "custom-session-123", body.SessionID)
	}
}
