package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidRepoURL  = errors.New("repository URL must look like https://github.com/{owner}/{repo}")
	ErrRepoUnavailable = errors.New("repository is unavailable")
)

var ownerRepo = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
var repoName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

func ParseRepoURL(raw string) (owner, name, canonical string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") {
		return "", "", "", ErrInvalidRepoURL
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", "", ErrInvalidRepoURL
	}
	owner, name = parts[0], strings.TrimSuffix(parts[1], ".git")
	if !ownerRepo.MatchString(owner) || !repoName.MatchString(name) || name == "." || name == ".." {
		return "", "", "", ErrInvalidRepoURL
	}
	return owner, name, "https://github.com/" + owner + "/" + name, nil
}

type RepoMetadata struct {
	FullName      string    `json:"fullName"`
	Description   string    `json:"description"`
	DefaultBranch string    `json:"defaultBranch"`
	Private       bool      `json:"private"`
	Stars         int       `json:"stars"`
	OpenIssues    int       `json:"openIssues"`
	PushedAt      time.Time `json:"pushedAt"`
	Language      string    `json:"language"`
}

type GitHubClient interface {
	Repository(ctx context.Context, owner, name string) (*RepoMetadata, error)
}

type HTTPGitHub struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewHTTPGitHub(token string) *HTTPGitHub {
	return &HTTPGitHub{BaseURL: "https://api.github.com", Token: token, Client: &http.Client{Timeout: 5 * time.Second}}
}

func (g *HTTPGitHub) Repository(ctx context.Context, owner, name string) (*RepoMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", g.BaseURL, url.PathEscape(owner), url.PathEscape(name)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "eduflow-backend")
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	res, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRepoUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: github returned %d", ErrRepoUnavailable, res.StatusCode)
	}
	var body struct {
		FullName      string    `json:"full_name"`
		Description   *string   `json:"description"`
		DefaultBranch string    `json:"default_branch"`
		Private       bool      `json:"private"`
		Stars         int       `json:"stargazers_count"`
		OpenIssues    int       `json:"open_issues_count"`
		PushedAt      time.Time `json:"pushed_at"`
		Language      *string   `json:"language"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRepoUnavailable, err)
	}
	m := &RepoMetadata{FullName: body.FullName, DefaultBranch: body.DefaultBranch, Private: body.Private,
		Stars: body.Stars, OpenIssues: body.OpenIssues, PushedAt: body.PushedAt}
	if body.Description != nil {
		m.Description = *body.Description
	}
	if body.Language != nil {
		m.Language = *body.Language
	}
	return m, nil
}
