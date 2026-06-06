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
	conversationRepository := repository.NewConversationRepository(db)
	cooldownRepository := repository.NewCooldownRepository(redisClient)
	reminderRepository := repository.NewReminderRepository(redisClient, appConfig.Redis.ReminderStateTTL, cooldownRepository)
	ipRateLimiter := repository.NewIPRateLimiter(redisClient, appConfig.Redis.IPRateLimitTTL)
	reminderService := services.NewReminderServiceWithDependencies(
		conversationRepository,
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
	authHandler := handlers.NewAuthHandler(authService, appConfig)

	// Routes
	router.POST("/auth/request-otp", authHandler.RequestOTP)
	router.POST("/auth/verify-otp", authHandler.VerifyOTP)
	
	api := router.Group("/api")
	api.GET("/scan", messageHandler.GetScan)
	api.POST("/messages", messageHandler.CreateMessage)
	api.GET("/conversations/:id/status", messageHandler.GetConversationStatus)
	api.POST("/conversations/:id/reminder", reminderHandler.CreateReminder)

	v1 := api.Group("/v1")
	v1.Use(middleware.AuthRequired())

	conversations := v1.Group("/conversations")
	conversations.GET("/", messageHandler.GetConversations)
	conversations.GET("/:id", messageHandler.GetDetailConversation)

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
