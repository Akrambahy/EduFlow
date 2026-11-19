package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/joho/godotenv"
)

type Config struct {
	Env                    string
	Port                   string
	DatabaseURL            string
	AllowedOrigins         []string
	JWTSecret              []byte
	AccessTokenTTL         time.Duration
	RefreshTokenTTL        time.Duration
	CookieSecure           bool
	StorageDir             string
	MaxUploadBytes         int64
	Timezone               *time.Location
	AttendanceTokenTTL     time.Duration
	AttendanceGraceMinutes int
	GitHubToken            string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	env := getEnv("APP_ENV", "development")
	cfg := &Config{
		Env:            env,
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		AllowedOrigins: splitList(getEnv("ALLOWED_ORIGINS", "http://localhost:5173")),
		JWTSecret:      []byte(os.Getenv("JWT_SECRET")),
		StorageDir:     getEnv("STORAGE_DIR", "./storage"),
		GitHubToken:    os.Getenv("GITHUB_TOKEN"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	var err error
	if cfg.AccessTokenTTL, err = getDuration("ACCESS_TOKEN_TTL", 2*time.Hour); err != nil {
		return nil, err
	}
	if cfg.RefreshTokenTTL, err = getDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour); err != nil {
		return nil, err
	}
	if cfg.CookieSecure, err = getBool("COOKIE_SECURE", env == "production"); err != nil {
		return nil, err
	}
	mb, err := strconv.Atoi(getEnv("MAX_UPLOAD_MB", "20"))
	if err != nil || mb < 1 || mb > 1024 {
		return nil, errors.New("MAX_UPLOAD_MB must be between 1 and 1024")
	}
	cfg.MaxUploadBytes = int64(mb) << 20
	if cfg.AttendanceTokenTTL, err = getDuration("ATTENDANCE_TOKEN_TTL", 30*time.Second); err != nil {
		return nil, err
	}
	if cfg.AttendanceTokenTTL < 10*time.Second || cfg.AttendanceTokenTTL > 5*time.Minute {
		return nil, errors.New("ATTENDANCE_TOKEN_TTL must be between 10s and 5m")
	}
	if cfg.AttendanceGraceMinutes, err = strconv.Atoi(getEnv("ATTENDANCE_GRACE_MINUTES", "10")); err != nil || cfg.AttendanceGraceMinutes < 0 || cfg.AttendanceGraceMinutes > 240 {
		return nil, errors.New("ATTENDANCE_GRACE_MINUTES must be between 0 and 240")
	}
	if cfg.Timezone, err = time.LoadLocation(getEnv("APP_TIMEZONE", "Africa/Cairo")); err != nil {
		return nil, fmt.Errorf("APP_TIMEZONE: %w", err)
	}

	return cfg, nil
}

func (c *Config) ValidateAuth() error {
	if len(c.JWTSecret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 bytes")
	}
	if c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= c.AccessTokenTTL {
		return errors.New("REFRESH_TOKEN_TTL must be longer than a positive ACCESS_TOKEN_TTL")
	}
	if c.Env == "production" && !c.CookieSecure {
		return errors.New("COOKIE_SECURE must be true in production")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func getBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
