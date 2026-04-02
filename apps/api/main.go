package main

import (
	"context"
	"log"
	"os"

	"github.com/carissafarry/tag-me/api/internal/handlers"
	"github.com/carissafarry/tag-me/api/internal/middleware"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:rahasia@localhost:5432/tag_me?sslmode=disable"
	}

	// Connect to database
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Initialize router
	router := gin.Default()

	// Apply middleware
	router.Use(middleware.SessionTracking())

	// Initialize service and handler
	messageService := services.NewMessageService(db)
	messageHandler := handlers.NewMessageHandler(messageService)

	// Routes
	router.POST("/messages", messageHandler.CreateMessage)
	router.GET("/conversations/:id/status", messageHandler.GetConversationStatus)

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
