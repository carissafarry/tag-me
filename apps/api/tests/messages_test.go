package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/carissafarry/tag-me/api/internal/handlers"
	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/repository"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

type notificationEnqueuerSpy struct {
	mu    sync.Mutex
	calls []struct {
		notificationType string
		conversationID   string
		ownerContact     string
	}
}

func (s *notificationEnqueuerSpy) EnqueueNotification(ctx context.Context, notificationType string, conversationID string, ownerContact string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, struct {
		notificationType string
		conversationID   string
		ownerContact     string
	}{notificationType, conversationID, ownerContact})
	return nil
}

func (s *notificationEnqueuerSpy) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func (s *notificationEnqueuerSpy) LastCall() (string, string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) > 0 {
		c := s.calls[len(s.calls)-1]
		return c.notificationType, c.conversationID, c.ownerContact
	}
	return "", "", ""
}

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
	DROP TABLE IF EXISTS objects CASCADE;
	DROP TABLE IF EXISTS owners CASCADE;

	CREATE TABLE owners (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		contact VARCHAR(255) NOT NULL UNIQUE,
		contact_type VARCHAR(20) NOT NULL DEFAULT 'phone',
		is_active BOOLEAN NOT NULL DEFAULT true,
		dnd_enabled BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
		updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
	);

	CREATE TABLE objects (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		owner_id UUID NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		object_type VARCHAR(100) NOT NULL,
		plate VARCHAR(10),
		created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
		updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
	);

	CREATE TABLE qr_codes (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		owner_id UUID NOT NULL,
		qr_token VARCHAR(255) NOT NULL UNIQUE,
		object_type VARCHAR(100) NOT NULL,
		object_id UUID,
		is_active BOOLEAN NOT NULL DEFAULT true,
		path_file VARCHAR(255),
		created_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta'),
		updated_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta')
	);

	CREATE TABLE conversations (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		qr_code_id UUID NOT NULL,
		owner_id UUID NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
		expires_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT date_trunc('second', NOW() AT TIME ZONE 'Asia/Jakarta') + interval '24 hours',
		opened_at TIMESTAMP WITHOUT TIME ZONE,
		on_the_way_at TIMESTAMP WITHOUT TIME ZONE,
		resolved_at TIMESTAMP WITHOUT TIME ZONE,
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

	service := services.NewMessageServiceTest(db)
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

	service := services.NewMessageServiceTest(db)

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

	service := services.NewMessageServiceTest(db)
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

	service := services.NewMessageServiceTest(db)

	// Resolve QR token
	qrCode, err := service.ResolveQRToken(context.Background(), qrToken)
	if err != nil {
		t.Fatalf("Failed to resolve QR token: %v", err)
	}

	// Create conversation
	conversation, err := service.CreateConversation(context.Background(), qrCode, "session-create-conversation", "203.0.113.1")
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

type guardedMessageTestDeps struct {
	db           *pgxpool.Pool
	router       *gin.Engine
	redis        *miniredis.Miniredis
	messageState *repository.MessageStateRepository
	qrToken      string
	qrCodeID     string
}

func setupGuardedMessageTestDeps(t *testing.T, cooldown time.Duration) *guardedMessageTestDeps {
	t.Helper()

	db := setupTestDB(t)
	t.Cleanup(db.Close)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	messageStateRepository := repository.NewMessageStateRepository(client, 6*time.Hour)

	service := services.NewMessageServiceWithDependencies(
		repository.NewQRCodeRepository(db),
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
		messageStateRepository,
		repository.NewConversationCreationGuardRepository(client),
		nil, // no notification enqueuer for test
		&services.MessageConfig{
			ConversationCreationCooldown: cooldown,
			MaxMessagesPerSessionQR:    5,
		},
	)
	handler := handlers.NewMessageHandlerWithTracker(service, messageStateRepository)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "guarded-token-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	return &guardedMessageTestDeps{
		db:           db,
		router:       router,
		redis:        mr,
		messageState: messageStateRepository,
		qrToken:      qrToken,
		qrCodeID:     qrCodeID.String(),
	}
}

func performCreateMessageRequest(
	t *testing.T,
	router *gin.Engine,
	qrToken string,
	sessionID string,
	ipAddress string,
) *httptest.ResponseRecorder {
	t.Helper()

	payload := map[string]interface{}{
		"qr_token":     qrToken,
		"message_type": "text",
		"content":      "scanner message",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", ipAddress)
	req.Header.Set("X-Session-ID", sessionID)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func countRows(t *testing.T, db *pgxpool.Pool, table string) int {
	t.Helper()

	var count int
	err := db.QueryRow(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count rows in %s: %v", table, err)
	}

	return count
}

func insertGuardedQRToken(t *testing.T, db *pgxpool.Pool) string {
	t.Helper()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "guarded-token-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	return qrToken
}

func TestCreateMessageRejectsDuplicateConversationWithinCooldown(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Minute)

	first := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-dup", "203.0.113.20")
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first request status 201, got %d: %s", first.Code, first.Body.String())
	}

	second := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-dup", "203.0.113.20")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status 429, got %d: %s", second.Code, second.Body.String())
	}

	var response models.ErrorResponse
	if err := json.Unmarshal(second.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "rate_limited" {
		t.Fatalf("expected code rate_limited, got %s", response.Code)
	}
	if response.Error != "you are creating conversations too quickly, please try again later" {
		t.Fatalf("unexpected error message: %s", response.Error)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header to be set")
	}

	if countRows(t, deps.db, "conversations") != 1 {
		t.Fatalf("expected exactly 1 conversation row after duplicate request")
	}
	if countRows(t, deps.db, "messages") != 1 {
		t.Fatalf("expected exactly 1 message row after duplicate request")
	}
}

func TestCreateMessageAllowsSameSessionAndIPForDifferentQR(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Minute)
	secondQRToken := insertGuardedQRToken(t, deps.db)

	first := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-same-qr-scope", "203.0.113.21")
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first request status 201, got %d: %s", first.Code, first.Body.String())
	}

	second := performCreateMessageRequest(t, deps.router, secondQRToken, "session-same-qr-scope", "203.0.113.21")
	if second.Code != http.StatusCreated {
		t.Fatalf("expected second request status 201, got %d: %s", second.Code, second.Body.String())
	}

	if countRows(t, deps.db, "conversations") != 2 {
		t.Fatalf("expected 2 conversations for different QR tokens")
	}
	if countRows(t, deps.db, "messages") != 2 {
		t.Fatalf("expected 2 messages for different QR tokens")
	}
}

func TestCreateMessageAllowsSameSessionWithDifferentIP(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Minute)

	first := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-same-different-ip", "203.0.113.22")
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first request status 201, got %d: %s", first.Code, first.Body.String())
	}

	second := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-same-different-ip", "203.0.113.23")
	if second.Code != http.StatusCreated {
		t.Fatalf("expected second request status 201, got %d: %s", second.Code, second.Body.String())
	}

	if countRows(t, deps.db, "conversations") != 2 {
		t.Fatalf("expected 2 conversations when IP changes")
	}
	if countRows(t, deps.db, "messages") != 2 {
		t.Fatalf("expected 2 messages when IP changes")
	}
}

func TestCreateMessageAllowsSameIPWithDifferentSession(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Minute)

	first := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-a", "203.0.113.24")
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first request status 201, got %d: %s", first.Code, first.Body.String())
	}

	second := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-b", "203.0.113.24")
	if second.Code != http.StatusCreated {
		t.Fatalf("expected second request status 201, got %d: %s", second.Code, second.Body.String())
	}

	if countRows(t, deps.db, "conversations") != 2 {
		t.Fatalf("expected 2 conversations when session changes")
	}
	if countRows(t, deps.db, "messages") != 2 {
		t.Fatalf("expected 2 messages when session changes")
	}
}

func TestCreateMessageAllowsRequestAfterCooldownExpires(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Second)

	first := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-expiry", "203.0.113.25")
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first request status 201, got %d: %s", first.Code, first.Body.String())
	}

	deps.redis.FastForward(2 * time.Second)

	second := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-expiry", "203.0.113.25")
	if second.Code != http.StatusCreated {
		t.Fatalf("expected second request status 201 after cooldown, got %d: %s", second.Code, second.Body.String())
	}

	if countRows(t, deps.db, "conversations") != 2 {
		t.Fatalf("expected 2 conversations after cooldown expires")
	}
	if countRows(t, deps.db, "messages") != 2 {
		t.Fatalf("expected 2 messages after cooldown expires")
	}
}

func TestCreateMessageRejectsConcurrentDuplicateConversationCreation(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Minute)

	const attempts = 8
	statuses := make([]int, attempts)
	var wg sync.WaitGroup

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			recorder := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-concurrent", "203.0.113.26")
			statuses[index] = recorder.Code
		}(i)
	}

	wg.Wait()

	successCount := 0
	rateLimitedCount := 0
	for _, status := range statuses {
		if status == http.StatusCreated {
			successCount++
		}
		if status == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful request, got %d", successCount)
	}
	if rateLimitedCount != attempts-1 {
		t.Fatalf("expected %d rate-limited requests, got %d", attempts-1, rateLimitedCount)
	}
	if countRows(t, deps.db, "conversations") != 1 {
		t.Fatalf("expected exactly 1 conversation after concurrent duplicate requests")
	}
	if countRows(t, deps.db, "messages") != 1 {
		t.Fatalf("expected exactly 1 message after concurrent duplicate requests")
	}
}

func TestCreateMessageReturnsServiceUnavailableWhenGuardStoreIsDown(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Minute)
	deps.redis.Close()

	recorder := performCreateMessageRequest(t, deps.router, deps.qrToken, "session-redis-down", "203.0.113.27")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response models.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "service_unavailable" {
		t.Fatalf("expected code service_unavailable, got %s", response.Code)
	}
	if response.Error != "message service temporarily unavailable" {
		t.Fatalf("unexpected error message: %s", response.Error)
	}
	if countRows(t, deps.db, "conversations") != 0 {
		t.Fatalf("expected no conversation rows when guard store is unavailable")
	}
	if countRows(t, deps.db, "messages") != 0 {
		t.Fatalf("expected no message rows when guard store is unavailable")
	}
}

func TestCreateMessageRejectsWhenMessageLimitReached(t *testing.T) {
	deps := setupGuardedMessageTestDeps(t, time.Minute)
	sessionID := "session-maxed"
	ipAddress := "203.0.113.28"

	for i := 0; i < 5; i++ {
		if _, err := deps.messageState.TrackMessage(context.Background(), sessionID, deps.qrCodeID, time.Now().UTC()); err != nil {
			t.Fatalf("failed to seed message state: %v", err)
		}
	}

	recorder := performCreateMessageRequest(t, deps.router, deps.qrToken, sessionID, ipAddress)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response models.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response.Code != "daily_limit_exceeded" {
		t.Fatalf("expected code daily_limit_exceeded, got %s", response.Code)
	}
	if countRows(t, deps.db, "conversations") != 0 {
		t.Fatalf("expected no conversation rows when message limit is reached")
	}
	if countRows(t, deps.db, "messages") != 0 {
		t.Fatalf("expected no message rows when message limit is reached")
	}
}

// TestMessageCreationEnqueuesNotification: TAG-12 - Message creation enqueues new_message notification
func TestMessageCreationEnqueuesNotification(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test QR code
	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-enqueue-token-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	// Create service with notification spy
	enqueuer := &notificationEnqueuerSpy{}
	service := services.NewMessageServiceWithDependencies(
		repository.NewQRCodeRepository(db),
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
		nil,
		nil,
		enqueuer,
		nil,
	)
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
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify notification was enqueued
	if enqueuer.CallCount() != 1 {
		t.Fatalf("expected 1 notification enqueue call, got %d", enqueuer.CallCount())
	}

	notifType, convID, ownerContact := enqueuer.LastCall()
	if notifType != "new_message" {
		t.Errorf("expected notification type 'new_message', got %s", notifType)
	}
	if convID == "" {
		t.Error("notification should include conversation ID")
	}
	if ownerContact == "" {
		t.Error("notification should include owner contact")
	}
}

// TestMessageCreationWithEnqueueError verifies message is created even if enqueue fails
func TestMessageCreationWithEnqueueError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test QR code
	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "test-enqueue-error-" + uuid.New().String()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("Failed to insert test QR code: %v", err)
	}

	// Create service with failing enqueuer
	failingEnqueuer := &failingNotificationEnqueuer{}
	service := services.NewMessageServiceWithDependencies(
		repository.NewQRCodeRepository(db),
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
		nil,
		nil,
		failingEnqueuer,
		nil,
	)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	payload := map[string]interface{}{
		"qr_token":     qrToken,
		"message_type": "text",
		"content":      "test with failing enqueue",
	}
	payloadBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Message should still be created even though enqueue failed
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 despite enqueue error, got %d: %s", w.Code, w.Body.String())
	}

	// Verify conversation and message were created
	if countRows(t, db, "conversations") != 1 {
		t.Fatalf("expected 1 conversation created")
	}
	if countRows(t, db, "messages") != 1 {
		t.Fatalf("expected 1 message created")
	}
}

type failingNotificationEnqueuer struct{}

func (f *failingNotificationEnqueuer) EnqueueNotification(ctx context.Context, notificationType string, conversationID string, ownerContact string) error {
	return services.ErrMessageServiceUnavailable
}

// TestEnqueueNewMessageNotificationDirect tests service method directly
func TestEnqueueNewMessageNotificationDirect(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	enqueuer := &notificationEnqueuerSpy{}
	service := services.NewMessageServiceWithDependencies(
		repository.NewQRCodeRepository(db),
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
		nil,
		nil,
		enqueuer,
		nil,
	)

	ctx := context.Background()
	conversationID := "test-conv-456"
	ownerContact := "owner@test.com"

	// Call service method directly
	err := service.EnqueueNewMessageNotification(ctx, conversationID, ownerContact)
	if err != nil {
		t.Fatalf("enqueue should not error: %v", err)
	}

	// Verify enqueue was called
	if enqueuer.CallCount() != 1 {
		t.Fatalf("expected 1 enqueue call, got %d", enqueuer.CallCount())
	}

	notifType, convID, owner := enqueuer.LastCall()
	if notifType != "new_message" {
		t.Errorf("expected type=new_message, got %s", notifType)
	}
	if convID != conversationID {
		t.Errorf("expected conversationID=%s, got %s", conversationID, convID)
	}
	if owner != ownerContact {
		t.Errorf("expected owner=%s, got %s", ownerContact, owner)
	}
}

// TestEnqueueNewMessageNotificationWithNilEnqueuer tests that nil enqueuer doesn't error
func TestEnqueueNewMessageNotificationWithNilEnqueuer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := services.NewMessageServiceWithDependencies(
		repository.NewQRCodeRepository(db),
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
		nil,
		nil,
		nil, // nil enqueuer
		nil,
	)

	ctx := context.Background()
	// Should not panic or error when enqueuer is nil
	err := service.EnqueueNewMessageNotification(ctx, "conv-789", "owner@test.com")
	if err != nil {
		t.Fatalf("should not error with nil enqueuer: %v", err)
	}
}

// TestMessageCreationInvalidQRToken tests handler response for invalid token
func TestMessageCreationInvalidQRToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service := services.NewMessageServiceTest(db)
	handler := handlers.NewMessageHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/messages", handler.CreateMessage)

	payload := map[string]interface{}{
		"qr_token":     "nonexistent-token",
		"message_type": "text",
	}
	payloadBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/messages", strings.NewReader(string(payloadBytes)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var errResp models.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if errResp.Code != "invalid_qr_token" {
		t.Errorf("expected code invalid_qr_token, got %s", errResp.Code)
	}
}

// TestGetConversationStatusService tests service method directly
func TestGetConversationStatusService(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	conversationID := uuid.New()

	// Insert conversation
	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, "token-status-test", "link", uuid.New())
	if err != nil {
		t.Fatalf("failed to insert qr code: %v", err)
	}

	_, err = db.Exec(context.Background(), `
		INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, conversationID, qrCodeID, ownerID, "PENDING")
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	service := services.NewMessageServiceTest(db)
	conv, err := service.GetConversationStatus(context.Background(), conversationID.String())
	if err != nil {
		t.Fatalf("GetConversationStatus failed: %v", err)
	}

	if conv.ID != conversationID {
		t.Errorf("expected conversation ID %s, got %s", conversationID, conv.ID)
	}
	if conv.Status != "PENDING" {
		t.Errorf("expected status PENDING, got %s", conv.Status)
	}
}

// TestFindActiveConversationBySessionAndQR tests finding active conversation
func TestFindActiveConversationBySessionAndQR(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	conversationID := uuid.New()
	sessionID := "active-session"

	// Insert QR and conversation
	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, "active-token", "link", uuid.New())
	if err != nil {
		t.Fatalf("failed to insert qr code: %v", err)
	}

	_, err = db.Exec(context.Background(), `
		INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, conversationID, qrCodeID, ownerID, "PENDING")
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	service := services.NewMessageServiceTest(db)
	conv, err := service.FindActiveConversationBySessionAndQR(context.Background(), sessionID, qrCodeID.String())

	// Expect error since we haven't linked the session to the conversation
	// (the repository checks for active status and matching session)
	if err == nil {
		t.Fatalf("expected error for unlinked session, got conversation: %+v", conv)
	}
	if conv != nil {
		t.Errorf("expected nil conversation on error, got %+v", conv)
	}
}

// TestResolveQRTokenService tests ResolveQRToken service method
func TestResolveQRTokenService(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "resolve-token-" + uuid.New().String()

	// Insert active QR code
	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("failed to insert qr code: %v", err)
	}

	service := services.NewMessageServiceTest(db)
	qr, err := service.ResolveQRToken(context.Background(), qrToken)
	if err != nil {
		t.Fatalf("ResolveQRToken failed: %v", err)
	}

	if qr.ID != qrCodeID {
		t.Errorf("expected qr ID %s, got %s", qrCodeID, qr.ID)
	}
	if !qr.IsActive {
		t.Error("expected QR code to be active")
	}
}

// TestResolveQRTokenInactive tests ResolveQRToken with inactive QR code
func TestResolveQRTokenInactive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	qrToken := "inactive-token-" + uuid.New().String()

	// Insert inactive QR code
	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, false)
	`, qrCodeID, ownerID, qrToken, "link", uuid.New())
	if err != nil {
		t.Fatalf("failed to insert qr code: %v", err)
	}

	service := services.NewMessageServiceTest(db)
	_, err = service.ResolveQRToken(context.Background(), qrToken)
	if err == nil {
		t.Fatal("expected error for inactive QR code")
	}
	if !errors.Is(err, services.ErrInactiveFQRToken) {
		t.Errorf("expected ErrInactiveFQRToken, got %v", err)
	}
}

// TestCreateMessageAllOptionalFields tests message creation with all optional fields
func TestCreateMessageAllOptionalFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	conversationID := uuid.New()

	_, err := db.Exec(context.Background(), `
		INSERT INTO conversations (id, qr_code_id, owner_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, conversationID, qrCodeID, ownerID, "PENDING")
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	service := services.NewMessageServiceTest(db)

	// Create message with all fields including optional ones
	content := "detailed message"
	lat := 37.7749
	lon := -122.4194
	locText := "San Francisco"
	sessionID := "test-session"
	ipAddr := "192.168.1.1"

	msg, err := service.CreateMessage(
		context.Background(),
		conversationID,
		"photo",
		&content,
		&lat,
		&lon,
		&locText,
		&sessionID,
		&ipAddr,
	)

	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	if msg.Content == nil || *msg.Content != content {
		t.Errorf("expected content %s, got %v", content, msg.Content)
	}
	if msg.LocationLatitude == nil || *msg.LocationLatitude != lat {
		t.Errorf("expected lat %f, got %v", lat, msg.LocationLatitude)
	}
	if msg.LocationLongitude == nil || *msg.LocationLongitude != lon {
		t.Errorf("expected lon %f, got %v", lon, msg.LocationLongitude)
	}
	if msg.LocationText == nil || *msg.LocationText != locText {
		t.Errorf("expected location text %s, got %v", locText, msg.LocationText)
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

	service := services.NewMessageServiceTest(db)
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

	service := services.NewMessageServiceTest(db)
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

	service := services.NewMessageServiceTest(db)

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

	service := services.NewMessageServiceTest(db)
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

	service := services.NewMessageServiceTest(db)
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

	service := services.NewMessageServiceTest(db)
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

	service := services.NewMessageServiceTest(db)
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

	// Verify response has exactly 4 fields, no owner/session data
	respMap := make(map[string]interface{})
	json.Unmarshal(w.Body.Bytes(), &respMap)

	expectedFields := map[string]bool{
		"conversation_id": true,
		"status":          true,
		"created_at":      true,
		"can_follow_up":   true,
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

	service := services.NewMessageServiceTest(db)
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
	service := services.NewMessageServiceTest(db)

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
	service := services.NewMessageServiceTest(db)

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
	service := services.NewMessageServiceTest(db)
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
