package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type bindSample struct {
	EmailAddress string `json:"emailAddress" binding:"required,email"`
}

func TestBindJSONValidationDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", func(c *gin.Context) {
		var in bindSample
		if BindJSON(c, &in) {
			c.Status(http.StatusNoContent)
		}
	})

	cases := map[string]struct {
		body string
		code string
	}{
		"empty":   {"", "invalid_body"},
		"broken":  {"{", "invalid_body"},
		"invalid": {`{"emailAddress":"nope"}`, "validation_error"},
	}
	for name, tc := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tc.body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status %d", name, w.Code)
		}
		var body ErrorResponse
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body.Code != tc.code {
			t.Fatalf("%s: code %q", name, body.Code)
		}
		if name == "invalid" {
			details, _ := body.Details.(map[string]any)
			if details["emailAddress"] == nil {
				t.Fatalf("details missing camelCase field: %v", body.Details)
			}
		}
	}
}
