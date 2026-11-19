package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/config"
)

func TestUnversionedPrefixIsNotFound(t *testing.T) {
	r := New(&config.Config{Env: "test", AllowedOrigins: []string{"http://localhost:5173"}}, nil)

	for _, path := range []string{"/api/health", "/api/v1/nope"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d, want 404", path, w.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: invalid json: %v", path, err)
		}
		if body["code"] != "not_found" || body["message"] == "" {
			t.Fatalf("%s: unexpected body %v", path, body)
		}
	}
}

func TestCORSExposesTotalCount(t *testing.T) {
	r := New(&config.Config{Env: "test", AllowedOrigins: []string{"http://localhost:5173"}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Expose-Headers"); got == "" {
		t.Fatal("expected X-Total-Count to be exposed")
	}
}
