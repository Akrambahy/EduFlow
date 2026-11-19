package testdb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"
)

func URL(t *testing.T) string {
	t.Helper()
	loadEnv()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return url
}

func loadEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
			return
		}
		dir = filepath.Dir(dir)
	}
}
