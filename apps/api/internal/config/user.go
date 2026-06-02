package config

const (
	UserIDHeader = "X-User-ID"
)

type User struct {
	ID string
	ContactType string
}