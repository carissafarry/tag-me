package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/carissafarry/tag-me/api/internal/services"
)

// TestNotificationServiceEnqueuePayload verifies the notification service sends correct payload
func TestNotificationServiceEnqueuePayload(t *testing.T) {
	var capturedPayload map[string]interface{}

	// Mock worker server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/enqueue" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Capture payload
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Respond with success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":    "test-job-123",
			"enqueued":  true,
		})
	}))
	defer server.Close()

	// Test notification service
	notifSvc := services.NewNotificationService(server.URL)
	ctx := context.Background()

	// Enqueue new_message
	err := notifSvc.EnqueueNotification(ctx, "new_message", "conv-123", "owner@example.com")
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	// Verify payload
	if capturedPayload == nil {
		t.Fatal("payload not captured")
	}

	notificationType, ok := capturedPayload["type"].(string)
	if !ok || notificationType != "new_message" {
		t.Errorf("expected type=new_message, got %v", capturedPayload["type"])
	}

	conversationID, ok := capturedPayload["conversation_id"].(string)
	if !ok || conversationID != "conv-123" {
		t.Errorf("expected conversation_id=conv-123, got %v", capturedPayload["conversation_id"])
	}

	ownerContact, ok := capturedPayload["owner_contact"].(string)
	if !ok || ownerContact != "owner@example.com" {
		t.Errorf("expected owner_contact=owner@example.com, got %v", capturedPayload["owner_contact"])
	}
}

// TestNotificationServiceEnqueueFailure verifies error handling when worker is unavailable
func TestNotificationServiceEnqueueFailure(t *testing.T) {
	// Use non-existent server
	notifSvc := services.NewNotificationService("http://localhost:9999")
	ctx := context.Background()

	err := notifSvc.EnqueueNotification(ctx, "reminder", "conv-456", "owner2@example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestNotificationServiceValidation verifies payload validation
func TestNotificationServiceValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":   "test-job",
			"enqueued": true,
		})
	}))
	defer server.Close()

	notifSvc := services.NewNotificationService(server.URL)
	ctx := context.Background()

	// Test missing type
	err := notifSvc.EnqueueNotification(ctx, "", "conv-123", "owner@example.com")
	if err == nil {
		t.Error("expected error for missing type")
	}

	// Test missing conversation_id
	err = notifSvc.EnqueueNotification(ctx, "new_message", "", "owner@example.com")
	if err == nil {
		t.Error("expected error for missing conversation_id")
	}
}

// TestNotificationServiceWorkerError verifies error when worker returns error status
func TestNotificationServiceWorkerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("worker error"))
	}))
	defer server.Close()

	notifSvc := services.NewNotificationService(server.URL)
	ctx := context.Background()

	err := notifSvc.EnqueueNotification(ctx, "new_message", "conv-123", "owner@example.com")
	if err == nil {
		t.Fatal("expected error from worker error response")
	}
}

// TestNotificationServiceWorkerErrorResponse verifies error field in worker response
func TestNotificationServiceWorkerErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":   "test-job",
			"enqueued": false,
			"error":    "queue is full",
		})
	}))
	defer server.Close()

	notifSvc := services.NewNotificationService(server.URL)
	ctx := context.Background()

	err := notifSvc.EnqueueNotification(ctx, "reminder", "conv-456", "owner@example.com")
	if err == nil {
		t.Fatal("expected error from worker error field")
	}
	if !strings.Contains(err.Error(), "queue is full") {
		t.Errorf("expected error message to contain 'queue is full', got: %v", err)
	}
}

// TestNotificationServiceDefaultWorkerURL verifies default worker URL is used
func TestNotificationServiceDefaultWorkerURL(t *testing.T) {
	notifSvc := services.NewNotificationService("")
	// Just verify it doesn't panic and initializes with default URL
	if notifSvc == nil {
		t.Fatal("notification service should initialize with default URL")
	}
}

// TestNotificationServiceMalformedResponse tests handling of malformed worker response
func TestNotificationServiceMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json {"))
	}))
	defer server.Close()

	notifSvc := services.NewNotificationService(server.URL)
	ctx := context.Background()

	// Should handle invalid JSON gracefully
	err := notifSvc.EnqueueNotification(ctx, "test", "conv-123", "owner@test.com")
	if err != nil {
		t.Fatalf("should handle malformed response: %v", err)
	}
}

// TestNotificationServiceMultipleTypes tests enqueuing different notification types
func TestNotificationServiceMultipleTypes(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		callCount++

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":   "job-" + string(rune(callCount)),
			"enqueued": true,
		})
	}))
	defer server.Close()

	notifSvc := services.NewNotificationService(server.URL)
	ctx := context.Background()

	types := []string{"new_message", "reminder"}
	for _, notifType := range types {
		err := notifSvc.EnqueueNotification(ctx, notifType, "conv-123", "owner@test.com")
		if err != nil {
			t.Fatalf("failed to enqueue %s: %v", notifType, err)
		}
	}

	if callCount != 2 {
		t.Fatalf("expected 2 enqueue calls, got %d", callCount)
	}
}

// TestNotificationServiceContextCancel tests handling of cancelled context
func TestNotificationServiceContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"job_id": "test", "enqueued": true})
	}))
	defer server.Close()

	notifSvc := services.NewNotificationService(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := notifSvc.EnqueueNotification(ctx, "test", "conv-123", "owner@test.com")
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

// TestNotificationServiceSuccessResponse verifies successful enqueue with all fields
func TestNotificationServiceSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&struct{}{}); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"job_id":   "job-abc-123",
			"enqueued": true,
			"error":    "",
		})
	}))
	defer server.Close()

	notifSvc := services.NewNotificationService(server.URL)
	ctx := context.Background()

	err := notifSvc.EnqueueNotification(ctx, "new_message", "conv-abc", "user@test.com")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}
