package main

import (
	"context"
	"log"

	"github.com/carissafarry/tag-me/api/internal/config"
	"github.com/carissafarry/tag-me/api/internal/handlers"
	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/repository"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	appConfig := config.Load()

	// Connect to database
	db, err := pgxpool.New(context.Background(), appConfig.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	redisOptions, err := redis.ParseURL(appConfig.Redis.URL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}

	// Initialize router
	router := gin.Default()

	router.Use(config.NewCORSMiddleware(appConfig.HTTP.CORS))

	// Apply middleware
	router.Use(middleware.SessionTracking())

	// Initialize service and handler
	messageStateRepository := repository.NewMessageStateRepository(redisClient, appConfig.Redis.MessageStateTTL)
	conversationCreationGuardRepository := repository.NewConversationCreationGuardRepository(redisClient)

	// Initialize notification service (enqueuer for worker queue)
	// Use environment variable for worker URL, default to localhost
	workerURL := appConfig.Worker.URL
	if workerURL == "" {
		workerURL = "http://localhost:3010"
	}
	notificationService := services.NewNotificationService(workerURL)

	messageService := services.NewMessageServiceWithDependencies(
		repository.NewQRCodeRepository(db),
		repository.NewConversationRepository(db),
		repository.NewMessageRepository(db),
		messageStateRepository,
		conversationCreationGuardRepository,
		notificationService,
		&services.MessageConfig{
			ConversationCreationCooldown: appConfig.Message.ConversationCreationCooldown,
			MaxMessagesPerSessionQR:      appConfig.Message.MaxMessagesPerSessionQR,
		},
	)
	messageHandler := handlers.NewMessageHandlerWithTracker(messageService, messageStateRepository)
	cooldownRepository := repository.NewCooldownRepository(redisClient)
	reminderRepository := repository.NewReminderRepository(redisClient, appConfig.Redis.ReminderStateTTL, cooldownRepository)
	ipRateLimiter := repository.NewIPRateLimiter(redisClient, appConfig.Redis.IPRateLimitTTL)
	reminderService := services.NewReminderService(
		db,
		reminderRepository,
		messageStateRepository,
		cooldownRepository,
		ipRateLimiter,
		notificationService,
		&services.ReminderConfig{
			Cooldown:                appConfig.Reminder.Cooldown,
			MaxReminders:            appConfig.Reminder.MaxReminders,
			MaxMessagesPerSessionQR: appConfig.Reminder.MaxMessagesPerSessionQR,
			IPWindowLimit:           appConfig.Reminder.IPWindowLimit,
		},
		nil,
	)
	reminderHandler := handlers.NewReminderHandler(reminderService)

	// Initialize auth service and handler
	otpRepository := repository.NewOTPRepository(
		redisClient,
		&repository.OTPRepositoryConfig{
			OTPMaxRequestAttempts: appConfig.Auth.OTP.OTPMaxRequestAttempts,
			OTPCodeTTL:            appConfig.Auth.OTP.OTPCodeTTL,
			OTPVerifyCodeTTL:      appConfig.Auth.OTP.OTPVerifyCodeTTL,
			OTPAttemptTTL:         appConfig.Auth.OTP.OTPAttemptTTL,
			OTPMaxVerifyAttempts:  appConfig.Auth.OTP.OTPMaxVerifyAttempts,
		},
	)
	ownerRepository := repository.NewOwnerRepository(db)
	authService := services.NewAuthService(ownerRepository, otpRepository, appConfig.Auth.JWT.JWTSecret, appConfig.Auth.JWT.JWTExpiry)
	authHandler := handlers.NewAuthHandler(authService)

	// Routes
	router.POST("/auth/request-otp", authHandler.RequestOTP)
	router.POST("/auth/verify-otp", authHandler.VerifyOTP)
	router.GET("/scan", messageHandler.GetScan)
	router.POST("/messages", messageHandler.CreateMessage)

	v1 := router.Group("/api/v1")
	v1.Use(middleware.AuthRequired())
	v1.GET("/conversations", messageHandler.GetConversations)
	v1.GET("/conversations/:id", messageHandler.GetDetailConversation)
	v1.GET("/conversations/:id/status", messageHandler.GetConversationStatus)
	v1.POST("/conversations/:id/reminder", reminderHandler.CreateReminder)
	
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Start server
	log.Printf("Starting server on port %s", appConfig.Server.Port)
	if err := router.Run(":" + appConfig.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
