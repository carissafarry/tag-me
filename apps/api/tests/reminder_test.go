package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	"github.com/redis/go-redis/v9"
)

type reminderNotifierSpy struct {
	mu    sync.Mutex
	calls []string
}

func (s *reminderNotifierSpy) EnqueueNotification(_ context.Context, notificationType string, conversationID string, ownerContact string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = append(s.calls, conversationID+":"+notificationType)
	return nil
}

func (s *reminderNotifierSpy) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.calls)
}

type reminderTestDeps struct {
	router             *gin.Engine
	conversationID     string
	qrID               string
	messageStateRepo   *repository.MessageStateRepository
	reminderRepository *repository.ReminderRepository
	cooldownRepository *repository.CooldownRepository
	ipRateLimiter      *repository.IPRateLimiter
	notifier           *reminderNotifierSpy
	miniredis          *miniredis.Miniredis
}

func setupReminderTestDeps(t *testing.T, now time.Time, config *services.ReminderConfig) *reminderTestDeps {
	t.Helper()

	db := setupTestDB(t)
	t.Cleanup(db.Close)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	messageStateRepository := repository.NewMessageStateRepository(client, 6*time.Hour)
	cooldownRepository := repository.NewCooldownRepository(client)
	reminderRepository := repository.NewReminderRepository(client, 6*time.Hour, cooldownRepository)
	ipRateLimiter := repository.NewIPRateLimiter(client, 10*time.Minute)
	notifier := &reminderNotifierSpy{}

	service := services.NewReminderService(
		db,
		reminderRepository,
		messageStateRepository,
		cooldownRepository,
		ipRateLimiter,
		notifier,
		config,
		func() time.Time { return now.UTC() },
	)
	handler := handlers.NewReminderHandler(service)

	router := gin.New()
	router.Use(middleware.SessionTracking())
	router.POST("/conversations/:id/reminder", handler.CreateReminder)

	ownerID := uuid.New()
	qrCodeID := uuid.New()
	conversationID := uuid.New()

	_, err := db.Exec(context.Background(), `
		INSERT INTO qr_codes (id, owner_id, qr_token, object_type, object_id, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
	`, qrCodeID, ownerID, "reminder-token-"+uuid.New().String(), "vehicle", uuid.New())
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

	return &reminderTestDeps{
		router:             router,
		conversationID:     conversationID.String(),
		qrID:               qrCodeID.String(),
		messageStateRepo:   messageStateRepository,
		reminderRepository: reminderRepository,
		cooldownRepository: cooldownRepository,
		ipRateLimiter:      ipRateLimiter,
		notifier:           notifier,
		miniredis:          mr,
	}
}

func performReminderRequest(t *testing.T, router *gin.Engine, conversationID string, sessionID string, ipAddress string) *httptest.ResponseRecorder {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, "/conversations/"+conversationID+"/reminder", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	req.Header.Set("X-Forwarded-For", ipAddress)
	req.AddCookie(&http.Cookie{
		Name:  middleware.SessionCookieName,
		Value: sessionID,
		Path:  "/",
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func decodeReminderResponse(t *testing.T, recorder *httptest.ResponseRecorder) models.ReminderResponse {
	t.Helper()

	var response models.ReminderResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode reminder response: %v", err)
	}

	return response
}

func TestReminderEndpointSuccess(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, &services.ReminderConfig{
		Cooldown:                  2 * time.Minute,
		MaxReminders:              3,
		MaxMessagesPerSessionQR: 5,
		IPWindowLimit:             10,
	})

	sessionID := "session-success"
	if _, err := deps.messageStateRepo.TrackMessage(context.Background(), sessionID, deps.qrID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("failed to seed message state: %v", err)
	}

	recorder := performReminderRequest(t, deps.router, deps.conversationID, sessionID, "203.0.113.10")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %+v", response)
	}
	if response.Message != "Reminder sent" {
		t.Fatalf("expected success message, got %+v", response)
	}
	if response.RemainingReminder == nil || *response.RemainingReminder != 2 {
		t.Fatalf("expected remaining_reminder=2, got %+v", response.RemainingReminder)
	}
	if response.NextAllowedAt == nil || *response.NextAllowedAt != "2026-04-09T15:02:00+07:00" {
		t.Fatalf("expected jakarta ISO next_allowed_at, got %+v", response.NextAllowedAt)
	}
	if deps.notifier.Count() != 1 {
		t.Fatalf("expected one notification enqueue, got %d", deps.notifier.Count())
	}
}

func TestReminderEndpointCooldown(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, &services.ReminderConfig{
		Cooldown:                  2 * time.Minute,
		MaxReminders:              3,
		MaxMessagesPerSessionQR: 5,
		IPWindowLimit:             10,
	})

	sessionID := "session-cooldown"
	if _, err := deps.messageStateRepo.TrackMessage(context.Background(), sessionID, deps.qrID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("failed to seed message state: %v", err)
	}
	if _, err := deps.reminderRepository.ReserveReminder(context.Background(), sessionID, deps.qrID, now.Add(-time.Minute), 2*time.Minute, 3); err != nil {
		t.Fatalf("failed to seed reminder reservation: %v", err)
	}

	recorder := performReminderRequest(t, deps.router, deps.conversationID, sessionID, "203.0.113.11")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if response.Success || response.Reason != string(models.ReminderReasonCooldown) {
		t.Fatalf("expected cooldown response, got %+v", response)
	}
	if response.NextAllowedAt == nil || *response.NextAllowedAt != "2026-04-09T15:01:00+07:00" {
		t.Fatalf("expected cooldown next_allowed_at, got %+v", response.NextAllowedAt)
	}
}

func TestReminderEndpointLimitReached(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, &services.ReminderConfig{
		Cooldown:                  2 * time.Minute,
		MaxReminders:              3,
		MaxMessagesPerSessionQR: 5,
		IPWindowLimit:             10,
	})

	sessionID := "session-limit"
	if _, err := deps.messageStateRepo.TrackMessage(context.Background(), sessionID, deps.qrID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("failed to seed message state: %v", err)
	}

	for i := 0; i < 3; i++ {
		reserveAt := now.Add(-10 * time.Minute).Add(time.Duration(i) * 3 * time.Minute)
		if _, err := deps.reminderRepository.ReserveReminder(context.Background(), sessionID, deps.qrID, reserveAt, 2*time.Minute, 3); err != nil {
			t.Fatalf("failed to seed reminder reservation %d: %v", i, err)
		}
	}

	recorder := performReminderRequest(t, deps.router, deps.conversationID, sessionID, "203.0.113.12")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if response.Success || response.Reason != string(models.ReminderReasonLimitReached) {
		t.Fatalf("expected limit_reached response, got %+v", response)
	}
}

func TestReminderEndpointMessageRateLimited(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, &services.ReminderConfig{
		Cooldown:                  2 * time.Minute,
		MaxReminders:              3,
		MaxMessagesPerSessionQR: 2,
		IPWindowLimit:             10,
	})

	sessionID := "session-msg-limit"
	for i := 0; i < 2; i++ {
		if _, err := deps.messageStateRepo.TrackMessage(context.Background(), sessionID, deps.qrID, now.Add(-time.Duration(i+1)*time.Minute)); err != nil {
			t.Fatalf("failed to seed message state: %v", err)
		}
	}

	recorder := performReminderRequest(t, deps.router, deps.conversationID, sessionID, "203.0.113.13")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if response.Success || response.Reason != string(models.ReminderReasonRateLimited) {
		t.Fatalf("expected rate_limited response, got %+v", response)
	}
}

func TestReminderEndpointIPRateLimited(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, &services.ReminderConfig{
		Cooldown:                  2 * time.Minute,
		MaxReminders:              3,
		MaxMessagesPerSessionQR: 5,
		IPWindowLimit:             1,
	})

	sessionID := "session-ip-limit"
	if _, err := deps.messageStateRepo.TrackMessage(context.Background(), sessionID, deps.qrID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("failed to seed message state: %v", err)
	}

	if _, err := deps.ipRateLimiter.IncrementAndCheck(context.Background(), "203.0.113.14", deps.qrID, 1); err != nil {
		t.Fatalf("failed to seed ip rate limit: %v", err)
	}

	recorder := performReminderRequest(t, deps.router, deps.conversationID, sessionID, "203.0.113.14")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if response.Success || response.Reason != string(models.ReminderReasonRateLimited) {
		t.Fatalf("expected rate_limited response, got %+v", response)
	}
}

func TestReminderEndpointInvalidConversation(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, nil)

	recorder := performReminderRequest(t, deps.router, uuid.New().String(), "session-invalid", "203.0.113.15")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if response.Success || response.Reason != string(models.ReminderReasonInvalidConversation) {
		t.Fatalf("expected invalid_conversation response, got %+v", response)
	}
}

func TestReminderEndpointRedisUnavailable(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, nil)
	deps.miniredis.Close()

	recorder := performReminderRequest(t, deps.router, deps.conversationID, "session-redis-down", "203.0.113.16")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if response.Success || response.Reason != string(models.ReminderReasonUnavailable) {
		t.Fatalf("expected temporarily_unavailable response, got %+v", response)
	}
}

// TestReminderEnqueueNotification verifies reminder success enqueues notification
func TestReminderEnqueueNotification(t *testing.T) {
	now := time.Date(2026, 4, 9, 8, 0, 0, 0, time.UTC)
	deps := setupReminderTestDeps(t, now, &services.ReminderConfig{
		Cooldown:                  2 * time.Minute,
		MaxReminders:              3,
		MaxMessagesPerSessionQR: 5,
		IPWindowLimit:             10,
	})

	sessionID := "session-enqueue"
	if _, err := deps.messageStateRepo.TrackMessage(context.Background(), sessionID, deps.qrID, now.Add(-time.Minute)); err != nil {
		t.Fatalf("failed to seed message state: %v", err)
	}

	recorder := performReminderRequest(t, deps.router, deps.conversationID, sessionID, "203.0.113.30")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	response := decodeReminderResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected successful reminder, got %+v", response)
	}

	// Verify notification was enqueued
	if deps.notifier.Count() != 1 {
		t.Fatalf("expected 1 notification enqueue, got %d", deps.notifier.Count())
	}
}
