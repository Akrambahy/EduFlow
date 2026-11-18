package server

import (
	"context"
	"net/http"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/config"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(cfg *config.Config, db *pgxpool.Pool) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "Something went wrong", nil)
	}))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.NoRoute(func(c *gin.Context) {
		httpx.Error(c, http.StatusNotFound, "not_found", "Resource not found", nil)
	})

	v1 := r.Group("/api/v1")
	v1.GET("/health", health(db))

	return r
}

func health(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			httpx.Error(c, http.StatusServiceUnavailable, "database_unavailable", "Database is not reachable", nil)
			return
		}

		httpx.Data(c, http.StatusOK, gin.H{"status": "ok", "database": "up"})
	}
}
