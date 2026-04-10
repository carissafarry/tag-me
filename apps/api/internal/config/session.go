package config

import "time"

const (
	SessionCookieName = "tag_me_session"
	SessionIDHeader   = "X-Session-ID"
	SessionTTL        = 6 * time.Hour
)

type Session struct {
	CookieName string
	HeaderName string
	TTL        time.Duration
}

func DefaultSession() Session {
	return Session{
		CookieName: SessionCookieName,
		HeaderName: SessionIDHeader,
		TTL:        SessionTTL,
	}
}
