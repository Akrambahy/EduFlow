package server

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/config"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/storage"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testdb"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

var ginParam = regexp.MustCompile(`:([A-Za-z]+)`)

func TestOpenAPICoversEveryRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := testdb.New(t)
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Env: "test", AllowedOrigins: []string{"http://localhost:5173"}, JWTSecret: []byte("test-secret-test-secret-test-secret!"), AccessTokenTTL: time.Minute,
		RefreshTokenTTL: time.Hour, MaxUploadBytes: 1 << 20, Timezone: time.UTC, AttendanceTokenTTL: 30 * time.Second}
	r := NewWithOptions(cfg, pool, Options{Storage: store})

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	documented := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}
	var missing []string
	routes := map[string]bool{}
	for _, rt := range r.Routes() {
		path := strings.TrimPrefix(rt.Path, "/api/v1")
		key := rt.Method + " " + ginParam.ReplaceAllString(path, "{$1}")
		routes[key] = true
		if !documented[key] {
			missing = append(missing, key)
		}
	}
	var stale []string
	for key := range documented {
		method := strings.SplitN(key, " ", 2)[0]
		switch method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !routes[key] {
				stale = append(stale, key)
			}
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("routes missing from docs/api/openapi.yaml:\n%s", strings.Join(missing, "\n"))
	}
	if len(stale) > 0 {
		t.Errorf("documented operations without a route:\n%s", strings.Join(stale, "\n"))
	}
}
