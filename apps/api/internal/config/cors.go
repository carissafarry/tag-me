package config

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type CORS struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func DefaultCORS() CORS {
	return CORS{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", SessionIDHeader},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}

func (c CORS) GinConfig() cors.Config {
	return cors.Config{
		AllowOrigins:     c.AllowOrigins,
		AllowMethods:     c.AllowMethods,
		AllowHeaders:     c.AllowHeaders,
		ExposeHeaders:    c.ExposeHeaders,
		AllowCredentials: c.AllowCredentials,
		MaxAge:           c.MaxAge,
	}
}

func NewCORSMiddleware(config CORS) gin.HandlerFunc {
	return cors.New(config.GinConfig())
}
