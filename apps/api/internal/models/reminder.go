package models

import "time"

type ReminderReason string

const (
	ReminderReasonCooldown            ReminderReason = "cooldown"
	ReminderReasonLimitReached        ReminderReason = "limit_reached"
	ReminderReasonInvalidConversation ReminderReason = "invalid_conversation"
	ReminderReasonRateLimited         ReminderReason = "rate_limited"
	ReminderReasonUnavailable         ReminderReason = "temporarily_unavailable"
	ReminderReasonSent                ReminderReason = "sent"
)

type ReminderRequest struct {
	ConversationID string
	SessionID      string
	IPAddress      string
}

type ReminderState struct {
	Count      int       `json:"count"`
	LastSentAt time.Time `json:"last_sent_at"`
}

type MessageState struct {
	Count      int       `json:"count"`
	LastSentAt time.Time `json:"last_sent_at"`
}

type IPRateLimitState struct {
	Allowed   bool `json:"allowed"`
	Count     int  `json:"count"`
	Remaining int  `json:"remaining"`
}

type ReminderReservation struct {
	Reason            ReminderReason
	Count             int
	RemainingReminder int
	LastSentAt        *time.Time
	NextAllowedAt     *time.Time
}

type ReminderResult struct {
	Success           bool
	Message           string
	Reason            ReminderReason
	RemainingReminder *int
	NextAllowedAt     *time.Time
}

type ReminderResponse struct {
	Success           bool    `json:"success"`
	Message           string  `json:"message,omitempty"`
	Reason            string  `json:"reason,omitempty"`
	RemainingReminder *int    `json:"remaining_reminder,omitempty"`
	NextAllowedAt     *string `json:"next_allowed_at,omitempty"`
}
