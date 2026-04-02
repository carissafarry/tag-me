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

// setupTestDB creates a test database connection and cleans up tables
func setupTestDB(t *testing.T) *pgxpool.Pool {
	dbURL := "postgres://postgres:rahasia@localhost:5432/tag_me_test"
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
		status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
		created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
		updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
	);

	CREATE TABLE messages (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
		sender_type VARCHAR(50) NOT NULL DEFAULT 'SCANNER',
		message_type VARCHAR(100) NOT NULL,
		content TEXT,
		location_latitude DECIMAL(10, 8),
		location_longitude DECIMAL(11, 8),
		location_text TEXT,
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
	db := setupTestDB(t)
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

// TestQRTokenResolution: AC3 - Invalid QR token returns safe error response (service layer)
func TestQRTokenResolution(t *testing.T) {
	db := setupTestDB(t)
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

// TestInvalidQRTokenHTTPResponse: AC3 - HTTP endpoint validates qr_token with proper error response
func TestInvalidQRTokenHTTPResponse(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	tests := []struct {
		name           string
		qrToken        string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "nonexistent token",
			qrToken:        "nonexistent-token",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "invalid_qr_token",
		},
		{
			name:           "empty token",
			qrToken:        "",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "validation_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"qr_token":     tt.qrToken,
				"message_type": "text",
			}

			payloadBytes, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp models.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if resp.Code != tt.expectedCode {
				t.Errorf("expected code %s, got %s", tt.expectedCode, resp.Code)
			}

			// Verify safe error message (doesn't leak token info)
			if resp.Error == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

// TestConversationAndMessageCreation: AC2 - Valid request creates conversation and message
func TestConversationAndMessageCreation(t *testing.T) {
	db := setupTestDB(t)
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
	if conversation.Status != "PENDING" {
		t.Errorf("expected status 'PENDING', got %s", conversation.Status)
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
	ipAddr := "192.168.1.1" // Valid INET format

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
	db := setupTestDB(t)
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
	req.Header.Set("X-Forwarded-For", "127.0.0.1")

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
	if resp.Status != "PENDING" {
		t.Errorf("expected status 'PENDING', got %s", resp.Status)
	}
	if resp.CreatedAt == "" {
		t.Error("response missing created_at")
	}

	// Verify response structure: exactly 4 fields, no sensitive data
	respMap := make(map[string]interface{})
	err = json.Unmarshal(w.Body.Bytes(), &respMap)
	if err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Count fields - should be exactly conversation_id, message_id, status, created_at
	expectedFields := map[string]bool{
		"conversation_id": true,
		"message_id":      true,
		"status":          true,
		"created_at":      true,
	}

	for field := range respMap {
		if !expectedFields[field] {
			t.Errorf("response contains unexpected field: %s", field)
		}
	}

	for field := range expectedFields {
		if _, ok := respMap[field]; !ok {
			t.Errorf("response missing expected field: %s", field)
		}
	}

	// Verify sensitive fields are absent
	sensitiveFields := []string{"owner_id", "owner", "session_id", "session", "ip_address", "ip"}
	respBody := w.Body.String()
	for _, field := range sensitiveFields {
		if strings.Contains(respBody, field) {
			t.Errorf("response should not contain sensitive field: %s", field)
		}
	}
}

// TestSessionMetadataCapture: AC5 - Request metadata for session/rate-limit logic is captured
func TestSessionMetadataCapture(t *testing.T) {
	db := setupTestDB(t)
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
		SELECT session_id, host(ip_address) FROM messages WHERE id = $1
	`, resp.MessageID).Scan(&storedSessionID, &storedIP)
	if err != nil {
		t.Fatalf("Failed to query message: %v", err)
	}

	// Session ID should match the header value provided
	if storedSessionID == nil {
		t.Error("session_id should not be null")
	} else if *storedSessionID != "custom-session-123" {
		t.Errorf("expected session_id 'custom-session-123', got %s", *storedSessionID)
	}
	// IP should be the one from X-Forwarded-For header
	if storedIP == nil {
		t.Error("ip_address should not be null")
	} else if *storedIP != "203.0.113.5" {
		t.Errorf("expected ip_address '203.0.113.5', got %s", *storedIP)
	}
}

// TestInactiveQRToken: Edge case - inactive QR token
func TestInactiveQRToken(t *testing.T) {
	db := setupTestDB(t)
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
	db := setupTestDB(t)
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
	req.Header.Set("X-Forwarded-For", "127.0.0.1")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.CreateMessageResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

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

// TestHandlerErrorResponseForCreateConversation: Handler returns 500 when conversation creation fails
func TestHandlerErrorResponseForCreateConversation(t *testing.T) {
	db := setupTestDB(t)
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

	// Now drop conversations table to force DB error on insert
	_, err = db.Exec(context.Background(), "DROP TABLE IF EXISTS conversations CASCADE")
	if err != nil {
		t.Fatalf("Failed to drop conversations table: %v", err)
	}

	payload := map[string]interface{}{
		"qr_token":     qrToken,
		"message_type": "text",
	}

	payloadBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 500 error response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var resp models.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if resp.Code != "database_error" {
		t.Errorf("expected code 'database_error', got %s", resp.Code)
	}

	if resp.Error == "" {
		t.Error("error message should not be empty")
	}
}

// TestHandlerErrorResponseForCreateMessage: Handler returns 500 when message creation fails
func TestHandlerErrorResponseForCreateMessage(t *testing.T) {
	db := setupTestDB(t)
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

	// Drop messages table to force DB error on message insert (after conversation succeeds)
	_, err = db.Exec(context.Background(), "DROP TABLE IF EXISTS messages CASCADE")
	if err != nil {
		t.Fatalf("Failed to drop messages table: %v", err)
	}

	payload := map[string]interface{}{
		"qr_token":     qrToken,
		"message_type": "text",
	}

	payloadBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 500 error response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var resp models.ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if resp.Code != "database_error" {
		t.Errorf("expected code 'database_error', got %s", resp.Code)
	}

	if resp.Error == "" {
		t.Error("error message should not be empty")
	}
}

// TestGetConversationStatusValid: TAG-8 - Valid status lookup returns correct response
func TestGetConversationStatusValid(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test QR code and conversation
	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-token-" + uuid.New().String()
	conversationID := uuid.New()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	_, err = db.Exec(context.Background(), `
		INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, conversationID, qrCodeID, ownerID, "PENDING")
	if err != nil {
		t.Fatalf("Failed to insert test conversation: %v", err)
	}

	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.GET("/conversations/:id/status", handler.GetConversationStatus)

	req, _ := http.NewRequest("GET", "/conversations/"+conversationID.String()+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.ConversationStatusResponse
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.ConversationID != conversationID.String() {
		t.Errorf("expected conversation_id %s, got %s", conversationID.String(), resp.ConversationID)
	}
	if resp.Status != "PENDING" {
		t.Errorf("expected status 'PENDING', got %s", resp.Status)
	}
	if resp.CreatedAt == "" {
		t.Error("response missing created_at")
	}

	// Verify response has exactly 3 fields, no owner/session data
	respMap := make(map[string]interface{})
	json.Unmarshal(w.Body.Bytes(), &respMap)

	expectedFields := map[string]bool{
		"conversation_id": true,
		"status":          true,
		"created_at":      true,
	}
	for field := range respMap {
		if !expectedFields[field] {
			t.Errorf("response contains unexpected field: %s", field)
		}
	}

	// Verify no sensitive fields in response body
	respBody := w.Body.String()
	sensitiveFields := []string{"owner_id", "qr_code_id", "session_id", "ip_address"}
	for _, field := range sensitiveFields {
		if strings.Contains(respBody, field) {
			t.Errorf("response should not contain sensitive field: %s", field)
		}
	}
}

// TestGetConversationStatusNotFound: TAG-8 - Not found case returns safe error
func TestGetConversationStatusNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.GET("/conversations/:id/status", handler.GetConversationStatus)

	nonexistentID := uuid.New().String()
	req, _ := http.NewRequest("GET", "/conversations/"+nonexistentID+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	var resp models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if resp.Code != "not_found" {
		t.Errorf("expected code 'not_found', got %s", resp.Code)
	}
	if resp.Error == "" {
		t.Error("error message should not be empty")
	}
}

// TestStatusMappingLogic: AC2 & AC5 - Unit test for status mapping and allowed states
func TestStatusMappingLogic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	service := services.NewMessageService(db)

	// Test all allowed status states from AC2
	allowedStates := []string{"PENDING", "DELIVERED", "OPENED", "ON_THE_WAY", "RESOLVED"}

	for _, state := range allowedStates {
		t.Run("status_"+state, func(t *testing.T) {
			conversationID := uuid.New()
			qrCodeID := uuid.New()
			qrToken := "test-token-" + uuid.New().String()

			_, err := db.Exec(context.Background(), `
				INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
				VALUES ($1, $2, $3, $4, $5, true)
			`, qrCodeID, ownerID, qrToken, "link", uuid.New())
			if err != nil {
				t.Fatalf("Failed to insert QR code: %v", err)
			}

			// Insert conversation with specific status
			_, err = db.Exec(context.Background(), `
				INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
			`, conversationID, qrCodeID, ownerID, state)
			if err != nil {
				t.Fatalf("Failed to insert conversation with status %s: %v", state, err)
			}

			// Retrieve status and verify it matches
			conv, err := service.GetConversationStatus(context.Background(), conversationID.String())
			if err != nil {
				t.Errorf("unexpected error retrieving status %s: %v", state, err)
				return
			}

			if conv.Status != state {
				t.Errorf("expected status %s, got %s", state, conv.Status)
			}
		})
	}
}

// TestInvalidStatusRejection: AC2 - Invalid status in DB is caught and rejected
func TestInvalidStatusRejection(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	conversationID := uuid.New()
	qrToken := "test-token-" + uuid.New().String()
	service := services.NewMessageService(db)

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert QR code: %v", err)
	}

	// Insert conversation with INVALID status
	_, err = db.Exec(context.Background(), `
		INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, conversationID, qrCodeID, ownerID, "INVALID_STATUS")
	if err != nil {
		t.Fatalf("Failed to insert conversation: %v", err)
	}

	// Service should reject invalid status
	_, err = service.GetConversationStatus(context.Background(), conversationID.String())
	if err != services.ErrInvalidStatus {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}

// TestStateTransitionReadLogic: AC5 - Integration test verifying state transitions are readable
// Simulates: scanner sees initial state → owner changes state → scanner sees updated state
func TestStateTransitionReadLogic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	conversationID := uuid.New()
	qrToken := "test-token-" + uuid.New().String()
	service := services.NewMessageService(db)
	handler := handlers.NewMessageHandler(service)

	// Setup: Create QR code and conversation in PENDING state
	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert QR code: %v", err)
	}

	_, err = db.Exec(context.Background(), `
		INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, conversationID, qrCodeID, ownerID, "PENDING")
	if err != nil {
		t.Fatalf("Failed to insert conversation: %v", err)
	}

	router := gin.New()
	router.GET("/conversations/:id/status", handler.GetConversationStatus)

	// Step 1: Scanner sees initial PENDING state
	req1, _ := http.NewRequest("GET", "/conversations/"+conversationID.String()+"/status", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	var resp1 models.ConversationStatusResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)

	if resp1.Status != "PENDING" {
		t.Fatalf("expected initial status 'PENDING', got %s", resp1.Status)
	}

	// Step 2: Owner updates conversation status to DELIVERED (simulating owner action)
	_, err = db.Exec(context.Background(), `
		UPDATE conversations SET status = 'DELIVERED', updated_at = NOW() WHERE id = $1
	`, conversationID)
	if err != nil {
		t.Fatalf("Failed to update conversation status: %v", err)
	}

	// Step 3: Scanner polls again and sees DELIVERED state
	req2, _ := http.NewRequest("GET", "/conversations/"+conversationID.String()+"/status", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	var resp2 models.ConversationStatusResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp2.Status != "DELIVERED" {
		t.Errorf("expected updated status 'DELIVERED', got %s", resp2.Status)
	}

	// Verify scanner never sees owner_id in either response
	respBody1 := w1.Body.String()
	respBody2 := w2.Body.String()
	if strings.Contains(respBody1, "owner") || strings.Contains(respBody2, "owner") {
		t.Error("response must not expose owner information")
	}
}
