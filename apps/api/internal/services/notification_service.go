package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EnqueueRequest matches the worker /enqueue endpoint contract
type EnqueueRequest struct {
	Type            string `json:"type"`
	ConversationID  string `json:"conversation_id"`
	OwnerContact    string `json:"owner_contact"`
}

// EnqueueResponse from worker
type EnqueueResponse struct {
	JobID    string `json:"job_id"`
	Enqueued bool   `json:"enqueued"`
	Error    string `json:"error,omitempty"`
}

// NotificationService handles enqueueing notifications to worker
type NotificationService struct {
	workerURL  string
	httpClient *http.Client
}

// NewNotificationService creates a new notification service
func NewNotificationService(workerURL string) *NotificationService {
	if workerURL == "" {
		workerURL = "http://localhost:3010"
	}

	return &NotificationService{
		workerURL: workerURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// EnqueueNotification sends a notification job request to the worker
// Returns error if worker is unavailable or payload is invalid
func (s *NotificationService) EnqueueNotification(ctx context.Context, notificationType string, conversationID string, ownerContact string) error {
	if notificationType == "" {
		return fmt.Errorf("notification type required")
	}
	if conversationID == "" {
		return fmt.Errorf("conversation id required")
	}

	req := EnqueueRequest{
		Type:           notificationType,
		ConversationID: conversationID,
		OwnerContact:   ownerContact,
	}

	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal enqueue request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.workerURL+"/enqueue", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("enqueue notification: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("worker returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response (optional, for logging)
	var respData EnqueueResponse
	if err := json.Unmarshal(respBody, &respData); err != nil {
		// Even if we can't parse, if status was 200, it's OK
		return nil
	}

	if respData.Error != "" {
		return fmt.Errorf("worker error: %s", respData.Error)
	}

	return nil
}
