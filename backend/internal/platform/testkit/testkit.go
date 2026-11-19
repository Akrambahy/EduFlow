package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/config"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/server"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testdb"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const Password = "testpass123"

var passwordHash = func() string {
	h, err := bcrypt.GenerateFromPassword([]byte(Password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	return string(h)
}()

type App struct {
	T      *testing.T
	DB     *pgxpool.Pool
	Router *gin.Engine
	Config *config.Config
}

func NewApp(t *testing.T, opts ...server.Options) *App {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testdb.New(t)
	cfg := &config.Config{
		Env:             "test",
		AllowedOrigins:  []string{"http://localhost:5173"},
		JWTSecret:       []byte("test-secret-test-secret-test-secret!"),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: time.Hour,
	}
	var o server.Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return &App{T: t, DB: pool, Router: server.NewWithOptions(cfg, pool, o), Config: cfg}
}

func (a *App) CreateUser(name, email string, roles ...string) string {
	a.T.Helper()
	ctx := context.Background()
	var id string
	if err := a.DB.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		name, email, passwordHash).Scan(&id); err != nil {
		a.T.Fatalf("create user: %v", err)
	}
	for _, r := range roles {
		if _, err := a.DB.Exec(ctx,
			`INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = $2`, id, r); err != nil {
			a.T.Fatalf("assign role: %v", err)
		}
	}
	return id
}

func (a *App) Login(email string) string {
	a.T.Helper()
	res := a.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": email, "password": Password})
	if res.Code != http.StatusOK {
		a.T.Fatalf("login %s: %d %s", email, res.Code, res.Body.String())
	}
	var body struct {
		Token string `json:"token"`
	}
	res.JSON(&body)
	return body.Token
}

func (a *App) UserWithToken(name, email string, roles ...string) (string, string) {
	a.T.Helper()
	id := a.CreateUser(name, email, roles...)
	return id, a.Login(email)
}

type Response struct {
	*httptest.ResponseRecorder
	t *testing.T
}

func (r Response) JSON(dst any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body.Bytes(), dst); err != nil {
		r.t.Fatalf("decode %q: %v", r.Body.String(), err)
	}
}

func (r Response) ErrorCode() string {
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(r.Body.Bytes(), &body)
	return body.Code
}

func (r Response) Cookie(name string) *http.Cookie {
	for _, c := range r.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (a *App) Do(method, path, token string, body any, cookies ...*http.Cookie) Response {
	a.T.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			a.T.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, req)
	return Response{ResponseRecorder: w, t: a.T}
}

func (a *App) Expect(res Response, status int, code string) {
	a.T.Helper()
	if res.Code != status {
		a.T.Fatalf("status %d, want %d: %s", res.Code, status, res.Body.String())
	}
	if code != "" && res.ErrorCode() != code {
		a.T.Fatalf("code %q, want %q: %s", res.ErrorCode(), code, res.Body.String())
	}
}
