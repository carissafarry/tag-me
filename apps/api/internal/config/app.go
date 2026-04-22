package config

import "time"

type App struct {
	Server   Server
	Database Database
	Redis    Redis
	HTTP     HTTP
	Session  Session
	Message  Message
	Reminder Reminder
	Worker   Worker
	Auth     Auth
}

type Server struct {
	Port string
}

type Database struct {
	URL string
}

type Redis struct {
	URL              string
	MessageStateTTL  time.Duration
	ReminderStateTTL time.Duration
	IPRateLimitTTL   time.Duration
}

type HTTP struct {
	CORS CORS
}

type Reminder struct {
	Cooldown                time.Duration
	MaxReminders            int
	MaxMessagesPerSessionQR int
	IPWindowLimit           int
}

type Message struct {
	ConversationCreationCooldown time.Duration
	MaxMessagesPerSessionQR      int
}

type Worker struct {
	URL string
}

type Auth struct {
	JWTSecret     string
	JWTExpiry     time.Duration
	OTPTTL        time.Duration
	OTPAttemptTTL time.Duration
	MaxOTPAttempts int64
}

// Load reads configuration from environment variables and returns an App struct with the values.
func Load() App {
	return App{
		Server: Server{
			Port: String("PORT", "8080"),
		},
		Database: Database{
			URL: String("DATABASE_URL", "postgres://postgres:rahasia@localhost:5432/tag_me?sslmode=disable"),
		},
		Redis: Redis{
			URL:              String("REDIS_URL", "redis://localhost:6379/0"),
			MessageStateTTL:  6 * time.Hour,
			ReminderStateTTL: 6 * time.Hour,
			IPRateLimitTTL:   10 * time.Minute,
		},
		HTTP: HTTP{
			CORS: DefaultCORS(),
		},
		Session: DefaultSession(),
		Auth: Auth{
			JWTSecret:      String("JWT_SECRET", ""),
			JWTExpiry:      DurationFromSeconds("JWT_EXPIRY_SECONDS", 24*time.Hour),
			OTPTTL:         5 * time.Minute,
			OTPAttemptTTL:  15 * time.Minute,
			MaxOTPAttempts: 5,
		},
		Message: Message{
			ConversationCreationCooldown: DurationFromSeconds("CONVERSATION_CREATION_COOLDOWN_SECONDS", 60*time.Second),
			MaxMessagesPerSessionQR:      PositiveInt("MESSAGE_MAX_PER_SESSION_QR", 5),
		},
		Reminder: Reminder{
			Cooldown:                DurationFromSeconds("REMINDER_COOLDOWN_SECONDS", 2*time.Minute),
			MaxReminders:            PositiveInt("REMINDER_MAX_ATTEMPTS", 3),
			MaxMessagesPerSessionQR: PositiveInt("MESSAGE_MAX_PER_SESSION_QR", 5),
			IPWindowLimit:           PositiveInt("REMINDER_IP_WINDOW_LIMIT", 10),
		},
		Worker: Worker{
			URL: String("WORKER_URL", "http://localhost:3010"),
		},
	}
}
