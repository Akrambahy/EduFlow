package server

import (
	"context"
	"net/http"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/assessment"
	"github.com/Akrambahy/EduFlow/backend/internal/attendance"
	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/performance"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/config"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/files"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/storage"
	"github.com/Akrambahy/EduFlow/backend/internal/sessions"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	LoginLimit   gin.HandlerFunc
	RefreshLimit gin.HandlerFunc
	Storage      storage.Storage
}

func New(cfg *config.Config, db *pgxpool.Pool) *gin.Engine {
	return NewWithOptions(cfg, db, Options{
		LoginLimit:   httpx.NewRateLimiter(12*time.Second, 5).Middleware(),
		RefreshLimit: httpx.NewRateLimiter(6*time.Second, 10).Middleware(),
	})
}

func NewWithOptions(cfg *config.Config, db *pgxpool.Pool, opts Options) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	if gin.Mode() != gin.TestMode {
		r.Use(gin.Logger())
	}
	r.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		httpx.Error(c, http.StatusInternalServerError, "internal_error", "Something went wrong", nil)
	}))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.NoRoute(func(c *gin.Context) {
		httpx.Error(c, http.StatusNotFound, "not_found", "Resource not found", nil)
	})

	api := r.Group("/api/v1")
	api.GET("/health", health(db))

	if db != nil {
		identitySvc := identity.NewService(
			identity.NewRepository(db),
			identity.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL),
			cfg.RefreshTokenTTL,
		)
		identity.NewHandler(identitySvc, identity.HandlerOptions{
			RefreshTTL:   cfg.RefreshTokenTTL,
			CookieSecure: cfg.CookieSecure,
			LoginLimit:   opts.LoginLimit,
			RefreshLimit: opts.RefreshLimit,
		}).Register(api)
		learningSvc := learning.NewService(learning.NewRepository(db))
		identitySvc.SetProfileExtender(learningSvc)
		identitySvc.SetUserScoper(learningSvc)
		identity.NewUsersHandler(identitySvc, db).Register(api)
		learning.NewHandler(learningSvc).Register(api, identitySvc.RequireAuth())

		store := opts.Storage
		if store == nil {
			local, err := storage.NewLocal(cfg.StorageDir)
			if err != nil {
				panic(err)
			}
			store = local
		}
		sessionsSvc := sessions.NewService(sessions.NewRepository(db), learningSvc, cfg.Timezone)
		filesSvc := files.NewService(db, store, cfg.MaxUploadBytes)
		filesSvc.AddAccessRule(func(ctx context.Context, r files.Requester, fileID string) (bool, error) {
			return sessionsSvc.FileVisible(ctx, learning.Viewer{UserID: r.UserID, Roles: r.Roles, IsAdmin: r.IsAdmin}, fileID)
		})
		files.NewHandler(filesSvc, requester).Register(api, identitySvc.RequireAuth())
		sessions.NewHandler(sessionsSvc).Register(api, identitySvc.RequireAuth())
		attendanceSvc := attendance.NewService(attendance.NewRepository(db), learningSvc, attendance.Config{TokenTTL: cfg.AttendanceTokenTTL, GraceMinutes: cfg.AttendanceGraceMinutes})
		attendance.NewHandler(attendanceSvc).Register(api, identitySvc.RequireAuth())
		assessmentSvc := assessment.NewService(db, learningSvc, cfg.Timezone)
		filesSvc.AddAccessRule(func(ctx context.Context, r files.Requester, fileID string) (bool, error) {
			return assessmentSvc.SubmissionFileVisible(ctx, learning.Viewer{UserID: r.UserID, Roles: r.Roles, IsAdmin: r.IsAdmin}, fileID)
		})
		assessment.NewHandler(assessmentSvc).Register(api, identitySvc.RequireAuth())
		performanceSvc := performance.NewService(db, learningSvc)
		performance.NewHandler(performanceSvc).Register(api, identitySvc.RequireAuth())
	}

	return r
}

func health(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			httpx.Error(c, http.StatusServiceUnavailable, "database_unavailable", "Database is not reachable", nil)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			httpx.Error(c, http.StatusServiceUnavailable, "database_unavailable", "Database is not reachable", nil)
			return
		}

		httpx.Data(c, http.StatusOK, gin.H{"status": "ok", "database": "up"})
	}
}

func requester(c *gin.Context) files.Requester {
	p := identity.CurrentPrincipal(c)
	roles := p.Roles
	if roles == nil {
		roles = []string{}
	}
	return files.Requester{UserID: p.UserID, Roles: roles, IsAdmin: p.IsAdmin()}
}
