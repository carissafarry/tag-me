package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	// "time"

	"github.com/carissafarry/tag-me/api/internal/handlers"
	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestSetup creates a test database connection and cleans up tables
func TestSetup(t *testing.T) *pgxpool.Pool {
	dbURL := "postgres://postgres:password@localhost:5432/tag_me_test"
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Create tables
	schema := `
	DROP TABLE IF EXISTS messages CASCADE;
	DROP TABLE IF EXISTS conversations CASCADE;
	DROP TABLE IF EXISTS qr_codes CASCADE;

	CREATE TABLE qr_codes (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		owner_id UUID NOT NULL,
		qr_token VARCHAR(255) NOT NULL UNIQUE,
		object_type VARCHAR(100) NOT NULL,
		object_id UUID NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
		updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
	);

	CREATE TABLE conversations (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		qr_code_id UUID NOT NULL,
		owner_id UUID NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'active',
		created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
		updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
	);

	CREATE TABLE messages (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
		message_type VARCHAR(100) NOT NULL,
		content TEXT,
		location_latitude DECIMAL(10, 8),
		location_longitude DECIMAL(11, 8),
		session_id VARCHAR(255),
		ip_address INET,
		created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
	);
	`

	_, err = db.Exec(context.Background(), schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

// TestPayloadValidation: AC1 - POST /messages accepts documented payload shape
func TestPayloadValidation(t *testing.T) {
	db := TestSetup(t)
	defer db.Close()

	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "valid payload with all fields",
			payload: `{
				"qr_token": "test-token",
				"message_type": "photo",
				"content": "test message",
				"location_latitude": 37.7749,
				"location_longitude": -122.4194
			}`,
			expectedStatus: http.StatusUnauthorized, // token invalid but payload accepted
			expectedCode:   "invalid_qr_token",
		},
		{
			name: "valid payload minimal fields",
			payload: `{
				"qr_token": "test-token",
				"message_type": "text"
			}`,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "invalid_qr_token",
		},
		{
			name:           "missing qr_token",
			payload:        `{"message_type": "text"}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "validation_error",
		},
		{
			name:           "missing message_type",
			payload:        `{"qr_token": "token"}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "validation_error",
		},
		{
			name:           "malformed JSON",
			payload:        `{"invalid": }`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "validation_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/messages", strings.NewReader(tt.payload))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp models.ErrorResponse
			_ = json.Unmarshal(w.Body.Bytes(), &resp)
			if resp.Code != tt.expectedCode {
				t.Errorf("expected code %s, got %s", tt.expectedCode, resp.Code)
			}
		})
	}
}

// TestQRTokenResolution: AC3 - Invalid QR token returns safe error response
func TestQRTokenResolution(t *testing.T) {
	db := TestSetup(t)
	defer db.Close()

	service := services.NewMessageService(db)

	tests := []struct {
		name       string
		qrToken    string
		setupFn    func() string // returns a valid token if needed
		shouldFail bool
		errorType  error
	}{
		{
			name:       "invalid token",
			qrToken:    "nonexistent-token",
			shouldFail: true,
			errorType:  services.ErrInvalidQRToken,
		},
		{
			name:       "empty token",
			qrToken:    "",
			shouldFail: true,
			errorType:  services.ErrInvalidQRToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ResolveQRToken(context.Background(), tt.qrToken)
			if !tt.shouldFail && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.shouldFail && err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

// TestConversationAndMessageCreation: AC2 - Valid request creates conversation and message
func TestConversationAndMessageCreation(t *testing.T) {
	db := TestSetup(t)
	defer db.Close()

	// Create test QR code
	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-valid-token-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	service := services.NewMessageService(db)

	// Resolve QR token
	qrCode, err := service.ResolveQRToken(context.Background(), qrToken)
	if err != nil {
		t.Fatalf("Failed to resolve QR token: %v", err)
	}

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), qrCode)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	if conversation.ID == uuid.Nil {
		t.Error("conversation ID is nil")
	}
	if conversation.Status != "active" {
		t.Errorf("expected status 'active', got %s", conversation.Status)
	}
	if conversation.OwnerID != ownerID {
		t.Errorf("expected owner_id %s, got %s", ownerID, conversation.OwnerID)
	}

	// Create initial message
	content := "test message"
	lat := 37.7749
	lon := -122.4194
	locationText := "Test Location"
	sessionID := "session-123"
	ipAddr := "192.168.1.1"

	message, err := service.CreateMessage(
		context.Background(),
		conversation.ID,
		"photo",
		&content,
		&lat,
		&lon,
		&locationText,
		&sessionID,
		&ipAddr,
	)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	if message.ID == uuid.Nil {
		t.Error("message ID is nil")
	}
	if message.ConversationID != conversation.ID {
		t.Errorf("expected conversation_id %s, got %s", conversation.ID, message.ConversationID)
	}
	if message.MessageType != "photo" {
		t.Errorf("expected message_type 'photo', got %s", message.MessageType)
	}
}

// TestResponseSerialization: AC6 - Response never exposes owner contact data
func TestResponseSerialization(t *testing.T) {
	db := TestSetup(t)
	defer db.Close()

	// Create test QR code
	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-token-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	payload := map[string]interface{}{
		"qr_token":     qrToken,
		"message_type": "text",
		"content":      "test message",
	}

	payloadBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	var resp models.CreateMessageResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response contains expected fields
	if resp.ConversationID == "" {
		t.Error("response missing conversation_id")
	}
	if resp.MessageID == "" {
		t.Error("response missing message_id")
	}
	if resp.Status != "active" {
		t.Errorf("expected status 'active', got %s", resp.Status)
	}
	if resp.CreatedAt == "" {
		t.Error("response missing created_at")
	}

	// Verify response DOES NOT contain owner data
	respBody := w.Body.String()
	if strings.Contains(respBody, ownerID.String()) {
		t.Error("response should not contain owner_id")
	}
	if strings.Contains(respBody, "owner") {
		t.Error("response should not contain owner data")
	}
	if strings.Contains(respBody, "session") {
		t.Error("response should not contain session data")
	}
	if strings.Contains(respBody, "ip") {
		t.Error("response should not contain IP address")
	}
}

// TestSessionMetadataCapture: AC5 - Request metadata for session/rate-limit logic is captured
func TestSessionMetadataCapture(t *testing.T) {
	db := TestSetup(t)
	defer db.Close()

	// Create test QR code
	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-token-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	payload := map[string]interface{}{
		"qr_token":     qrToken,
		"message_type": "text",
	}

	payloadBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	req.Header.Set("X-Session-ID", "custom-session-123")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session ID was returned in response header
	sessionIDHeader := w.Header().Get("X-Session-ID")
	if sessionIDHeader == "" {
		t.Error("response missing X-Session-ID header")
	}

	var resp models.CreateMessageResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Verify message was stored with metadata
	var storedSessionID *string
	var storedIP *string
	err = db.QueryRow(context.Background(), `
		SELECT session_id, ip_address::text FROM messages WHERE id = $1
	`, resp.MessageID).Scan(&storedSessionID, &storedIP)
	if err != nil {
		t.Fatalf("Failed to query message: %v", err)
	}

	if storedSessionID == nil || *storedSessionID == "" {
		t.Error("session_id was not stored")
	}
	if storedIP == nil || *storedIP == "" {
		t.Error("ip_address was not stored")
	}
}

// TestInactiveQRToken: Edge case - inactive QR token
func TestInactiveQRToken(t *testing.T) {
	db := TestSetup(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-inactive-token"

	// Create inactive QR code
	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, false)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	service := services.NewMessageService(db)

	_, err = service.ResolveQRToken(context.Background(), qrToken)
	if err != services.ErrInactiveFQRToken {
		t.Errorf("expected ErrInactiveFQRToken, got %v", err)
	}
}

// TestEdgeCaseMissingLocation: Location fields are optional
func TestEdgeCaseMissingLocation(t *testing.T) {
	db := TestSetup(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-token-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	// Request without location fields
	payload := map[string]interface{}{
		"qr_token":     qrToken,
		"message_type": "text",
	}

	payloadBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.CreateMessageResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Verify message exists but has null location
	var msg models.Message
	err = db.QueryRow(context.Background(), `
		SELECT id, message_type, location_latitude, location_longitude
		FROM messages WHERE id = $1
	`, resp.MessageID).Scan(&msg.ID, &msg.MessageType, &msg.LocationLatitude, &msg.LocationLongitude)

	if err != nil {
		t.Fatalf("Failed to query message: %v", err)
	}

	if msg.LocationLatitude != nil || msg.LocationLongitude != nil {
		t.Error("location fields should be null when not provided")
	}
}
