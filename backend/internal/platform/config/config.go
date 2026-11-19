package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env             string
	Port            string
	DatabaseURL     string
	AllowedOrigins  []string
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	CookieSecure    bool
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
