package config

import "time"

type App struct {
	Environment string
	Server      Server
	Database    Database
	Redis       Redis
	HTTP        HTTP
	Session     Session
	Message     Message
	Reminder    Reminder
	Worker      Worker
	Auth        Auth
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

type JWT struct {
	JWTSecret        string
	JWTExpiry        time.Duration
	JWTRefreshExpiry time.Duration
}

type OTP struct {
	OTPMaxRequestAttempts int
	OTPCodeTTL            time.Duration
	OTPVerifyCodeTTL      time.Duration
	OTPAttemptTTL         time.Duration
	OTPMaxVerifyAttempts  int
}

type Auth struct {
	JWT JWT
	OTP OTP
}

// Load reads configuration from environment variables and returns an App struct with the values.
func Load() App {
	return App{
		Environment: String("ENV", "development"),
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
			JWT: JWT{
				JWTSecret:        String("JWT_SECRET", "secret-jwt-signing-key"),
				JWTExpiry:        DurationFromSeconds("JWT_EXPIRY_SECONDS", 24*time.Hour),
				JWTRefreshExpiry: DurationFromSeconds("JWT_REFRESH_EXPIRY_SECONDS", 7*24*time.Hour),
			},
			OTP: OTP{
				OTPMaxRequestAttempts: PositiveInt("OTP_MAX_REQUEST_ATTEMPTS", 3),
				OTPCodeTTL:            DurationFromSeconds("OTP_TTL", 3*time.Minute),
				OTPVerifyCodeTTL:      DurationFromSeconds("OTP_VERIFY_CODE_TTL", 3*time.Minute),
				OTPAttemptTTL:         DurationFromSeconds("OTP_ATTEMPT_TTL", 60*time.Minute),
				OTPMaxVerifyAttempts:  PositiveInt("OTP_MAX_VERIFY_ATTEMPTS", 3),
			},
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
