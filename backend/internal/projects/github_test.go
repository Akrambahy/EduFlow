package projects

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRepoURL(t *testing.T) {
	ok := map[string]string{
		"https://github.com/alice/api":      "https://github.com/alice/api",
		"https://github.com/alice/api.git":  "https://github.com/alice/api",
		"https://GitHub.com/a-b/my.repo_1/": "https://github.com/a-b/my.repo_1",
	}
	for in, want := range ok {
		_, _, got, err := ParseRepoURL(in)
		if err != nil || got != want {
			t.Errorf("ParseRepoURL(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"http://github.com/a/b", "https://gitlab.com/a/b", "https://github.com/a", "https://github.com/a/b/c",
		"https://github.com/-bad/b", "https://github.com/a/..", "javascript:alert(1)", ""} {
		if _, _, _, err := ParseRepoURL(bad); !errors.Is(err, ErrInvalidRepoURL) {
			t.Errorf("ParseRepoURL(%q) accepted", bad)
		}
	}
}

func TestHTTPGitHub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/alice/api" {
			w.Write([]byte(`{"full_name":"alice/api","default_branch":"main","stargazers_count":3,"pushed_at":"2026-11-01T10:00:00Z","language":"Go"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	g := NewHTTPGitHub("")
	g.BaseURL = srv.URL
	m, err := g.Repository(context.Background(), "alice", "api")
	if err != nil || m.FullName != "alice/api" || m.Stars != 3 || m.Language != "Go" {
		t.Fatalf("%+v %v", m, err)
	}
	if _, err := g.Repository(context.Background(), "alice", "missing"); !errors.Is(err, ErrRepoUnavailable) {
		t.Fatalf("missing repo: %v", err)
	}
}
