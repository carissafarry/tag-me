package services

import (
	"context"
	"testing"
	"time"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/google/uuid"
)

type reminderConversationRepoStub struct {
	conversation models.Conversation
}

func (s *reminderConversationRepoStub) FindByID(context.Context, string) (*models.Conversation, error) {
	return &s.conversation, nil
}

type reminderRepoStub struct{}

func (s *reminderRepoStub) GetState(context.Context, string, string) (*models.ReminderState, error) {
	return &models.ReminderState{Count: 0}, nil
}

func (s *reminderRepoStub) ReserveReminder(context.Context, string, string, time.Time, time.Duration, int) (*models.ReminderReservation, error) {
	return &models.ReminderReservation{
		Reason:            models.ReminderReasonSent,
		RemainingReminder: 2,
	}, nil
}

type messageRepoStub struct{}

func (s *messageRepoStub) GetState(context.Context, string, string) (*models.MessageState, error) {
	return &models.MessageState{Count: 0}, nil
}

type cooldownRepoStub struct{}

func (s *cooldownRepoStub) GetNextAllowedAt(context.Context, string, string, string) (*time.Time, error) {
	return nil, nil
}

type ipRateLimiterStub struct{}

func (s *ipRateLimiterStub) IncrementAndCheck(context.Context, string, string, int) (*models.IPRateLimitState, error) {
	return &models.IPRateLimitState{Allowed: true}, nil
}

type blockingNotificationEnqueuer struct {
	started chan struct{}
	release chan struct{}
}

func (s *blockingNotificationEnqueuer) EnqueueNotification(context.Context, string, string, string) error {
	select {
	case <-s.started:
	default:
		close(s.started)
	}

	<-s.release
	return nil
}

func TestSendReminderAsyncEnqueue(t *testing.T) {
	enqueue := &blockingNotificationEnqueuer{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	service := NewReminderServiceWithDependencies(
		&reminderConversationRepoStub{
			conversation: models.Conversation{
				ID:       uuid.New(),
				QRCodeID: uuid.New(),
				Status:   "PENDING",
			},
		},
		&reminderRepoStub{},
		&messageRepoStub{},
		&cooldownRepoStub{},
		&ipRateLimiterStub{},
		enqueue,
		&ReminderConfig{
			Cooldown:                2 * time.Minute,
			MaxReminders:            3,
			MaxMessagesPerSessionQR: 5,
			IPWindowLimit:           10,
		},
		time.Now,
	)

	resultCh := make(chan *models.ReminderResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := service.SendReminder(context.Background(), models.ReminderRequest{
			ConversationID: uuid.New().String(),
			SessionID:      "session-1",
			IPAddress:      "203.0.113.20",
		})
		if err != nil {
			errCh <- err
			return
		}

		resultCh <- result
	}()

	select {
	case err := <-errCh:
		t.Fatalf("expected reminder success, got error: %v", err)
	case result := <-resultCh:
		if !result.Success {
			t.Fatalf("expected successful reminder result, got %+v", result)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected SendReminder to return without blocking on enqueue")
	}

	select {
	case <-enqueue.started:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected enqueue to run asynchronously")
	}

	close(enqueue.release)
}
