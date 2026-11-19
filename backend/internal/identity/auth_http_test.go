package identity_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/server"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
	"github.com/gin-gonic/gin"
)

type sessionBody struct {
	Token     string                `json:"token"`
	TokenType string                `json:"tokenType"`
	ExpiresIn int                   `json:"expiresIn"`
	User      identity.UserResponse `json:"user"`
}

func TestLoginSuccessReturnsContract(t *testing.T) {
	app := testkit.NewApp(t)
	id := app.CreateUser("Sara Admin", "admin@eduflow.dev", "admin", "instructor")

	res := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "ADMIN@eduflow.dev", "password": testkit.Password})
	app.Expect(res, http.StatusOK, "")

	var body sessionBody
	res.JSON(&body)
	if body.Token == "" || body.TokenType != "Bearer" || body.ExpiresIn != 900 {
		t.Fatalf("unexpected session %+v", body)
	}
	u := body.User
	if u.ID != id || u.Email != "admin@eduflow.dev" || u.Role != "admin" || len(u.Roles) != 2 || u.Avatar != nil {
		t.Fatalf("unexpected user %+v", u)
	}

	c := res.Cookie(identity.RefreshCookieName)
	if c == nil || !c.HttpOnly || c.Path != "/api/v1/auth" || c.SameSite != http.SameSiteStrictMode || c.Value == "" {
		t.Fatalf("bad refresh cookie %+v", c)
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("login response must not be cached")
	}

	var stored int
	if err := app.DB.QueryRow(context.Background(), `SELECT count(*) FROM refresh_sessions WHERE token_hash = $1`, identity.HashRefreshToken(c.Value)).Scan(&stored); err != nil || stored != 1 {
		t.Fatalf("refresh hash not stored: %d %v", stored, err)
	}
}

func TestLoginRejectsBadCredentialsWithoutLeaking(t *testing.T) {
	app := testkit.NewApp(t)
	app.CreateUser("Student", "student@eduflow.dev", "student")

	wrong := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "student@eduflow.dev", "password": "wrongpass1"})
	unknown := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "ghost@eduflow.dev", "password": "wrongpass1"})

	app.Expect(wrong, http.StatusUnauthorized, "invalid_credentials")
	app.Expect(unknown, http.StatusUnauthorized, "invalid_credentials")
	if wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("responses differ: %s vs %s", wrong.Body.String(), unknown.Body.String())
	}
	if wrong.Cookie(identity.RefreshCookieName) != nil {
		t.Fatal("no cookie on failed login")
	}
}

func TestLoginValidation(t *testing.T) {
	app := testkit.NewApp(t)
	res := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "not-an-email"})
	app.Expect(res, http.StatusBadRequest, "validation_error")

	var body httpx.ErrorResponse
	res.JSON(&body)
	details, _ := body.Details.(map[string]any)
	if details["email"] == nil || details["password"] == nil {
		t.Fatalf("details = %v", body.Details)
	}
}

func TestLoginDeactivatedAccount(t *testing.T) {
	app := testkit.NewApp(t)
	id := app.CreateUser("Gone", "gone@eduflow.dev", "student")
	if _, err := app.DB.Exec(context.Background(), `UPDATE users SET is_active = false WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	res := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "gone@eduflow.dev", "password": testkit.Password})
	app.Expect(res, http.StatusForbidden, "account_disabled")
}

func TestMeRequiresValidToken(t *testing.T) {
	app := testkit.NewApp(t)
	id, token := app.UserWithToken("Marwan", "student@eduflow.dev", "student")

	app.Expect(app.Do(http.MethodGet, "/api/v1/auth/me", "", nil), http.StatusUnauthorized, "unauthorized")
	app.Expect(app.Do(http.MethodGet, "/api/v1/auth/me", "garbage", nil), http.StatusUnauthorized, "unauthorized")

	other := identity.NewTokenIssuer([]byte("another-secret-another-secret-12345"), time.Minute)
	forged, _ := other.Issue(id, []string{"admin"})
	app.Expect(app.Do(http.MethodGet, "/api/v1/auth/me", forged, nil), http.StatusUnauthorized, "unauthorized")

	res := app.Do(http.MethodGet, "/api/v1/auth/me", token, nil)
	app.Expect(res, http.StatusOK, "")
	var u identity.UserResponse
	res.JSON(&u)
	if u.ID != id || u.Role != "student" {
		t.Fatalf("unexpected me %+v", u)
	}

	if _, err := app.DB.Exec(context.Background(), `UPDATE users SET is_active = false WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/auth/me", token, nil), http.StatusUnauthorized, "unauthorized")
}

func TestRefreshRotatesAndDetectsReuse(t *testing.T) {
	app := testkit.NewApp(t)
	app.CreateUser("Omar", "tracker@eduflow.dev", "tracker")

	login := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "tracker@eduflow.dev", "password": testkit.Password})
	first := login.Cookie(identity.RefreshCookieName)

	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/refresh", "", nil), http.StatusUnauthorized, "invalid_refresh_token")

	rotated := app.Do(http.MethodPost, "/api/v1/auth/refresh", "", nil, first)
	app.Expect(rotated, http.StatusOK, "")
	second := rotated.Cookie(identity.RefreshCookieName)
	if second == nil || second.Value == first.Value {
		t.Fatal("refresh token was not rotated")
	}
	var body sessionBody
	rotated.JSON(&body)
	if body.Token == "" || body.User.Role != "tracker" {
		t.Fatalf("unexpected refresh body %+v", body)
	}

	reuse := app.Do(http.MethodPost, "/api/v1/auth/refresh", "", nil, first)
	app.Expect(reuse, http.StatusUnauthorized, "invalid_refresh_token")
	if c := reuse.Cookie(identity.RefreshCookieName); c == nil || c.MaxAge >= 0 {
		t.Fatal("cookie should be cleared on failure")
	}

	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/refresh", "", nil, second), http.StatusUnauthorized, "invalid_refresh_token")
}

func TestRefreshRejectsExpiredSession(t *testing.T) {
	app := testkit.NewApp(t)
	app.CreateUser("Layla", "mentor@eduflow.dev", "mentor")
	login := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "mentor@eduflow.dev", "password": testkit.Password})
	c := login.Cookie(identity.RefreshCookieName)
	if _, err := app.DB.Exec(context.Background(), `UPDATE refresh_sessions SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/refresh", "", nil, c), http.StatusUnauthorized, "invalid_refresh_token")
}

func TestLogoutRevokesRefreshSession(t *testing.T) {
	app := testkit.NewApp(t)
	app.CreateUser("Ahmed", "instructor@eduflow.dev", "instructor")
	login := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "instructor@eduflow.dev", "password": testkit.Password})
	c := login.Cookie(identity.RefreshCookieName)

	out := app.Do(http.MethodPost, "/api/v1/auth/logout", "", nil, c)
	app.Expect(out, http.StatusNoContent, "")
	if cleared := out.Cookie(identity.RefreshCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatal("logout must clear cookie")
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/refresh", "", nil, c), http.StatusUnauthorized, "invalid_refresh_token")
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/logout", "", nil), http.StatusNoContent, "")
}

func TestLoginIsRateLimited(t *testing.T) {
	app := testkit.NewApp(t, server.Options{LoginLimit: httpx.NewRateLimiter(time.Hour, 2).Middleware()})
	body := map[string]string{"email": "x@eduflow.dev", "password": "wrongpass1"}
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/login", "", body), http.StatusUnauthorized, "")
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/login", "", body), http.StatusUnauthorized, "")
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/login", "", body), http.StatusTooManyRequests, "rate_limited")
}

func TestRequireRoles(t *testing.T) {
	app := testkit.NewApp(t)
	_, admin := app.UserWithToken("A", "a@eduflow.dev", "admin")
	_, student := app.UserWithToken("S", "s@eduflow.dev", "student")

	svc := identity.NewService(identity.NewRepository(app.DB), identity.NewTokenIssuer(app.Config.JWTSecret, app.Config.AccessTokenTTL), app.Config.RefreshTokenTTL)
	g := app.Router.Group("/api/v1/_test", svc.RequireAuth(), identity.RequireRoles("admin"))
	g.GET("/admin-only", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	app.Expect(app.Do(http.MethodGet, "/api/v1/_test/admin-only", "", nil), http.StatusUnauthorized, "unauthorized")
	app.Expect(app.Do(http.MethodGet, "/api/v1/_test/admin-only", student, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/_test/admin-only", admin, nil), http.StatusNoContent, "")
}
