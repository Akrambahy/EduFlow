package identity

import (
	"errors"
	"net/http"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const (
	RefreshCookieName = "eduflow_refresh"
	RefreshCookiePath = "/api/v1/auth"
)

type HandlerOptions struct {
	RefreshTTL   time.Duration
	CookieSecure bool
	LoginLimit   gin.HandlerFunc
	RefreshLimit gin.HandlerFunc
}

type Handler struct {
	svc  *Service
	opts HandlerOptions
}

func NewHandler(svc *Service, opts HandlerOptions) *Handler {
	pass := func(c *gin.Context) { c.Next() }
	if opts.LoginLimit == nil {
		opts.LoginLimit = pass
	}
	if opts.RefreshLimit == nil {
		opts.RefreshLimit = pass
	}
	return &Handler{svc: svc, opts: opts}
}

func (h *Handler) Register(g *gin.RouterGroup) {
	auth := g.Group("/auth")
	auth.POST("/login", h.opts.LoginLimit, h.login)
	auth.POST("/refresh", h.opts.RefreshLimit, h.refresh)
	auth.POST("/logout", h.logout)
	auth.GET("/me", h.svc.RequireAuth(), h.me)
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,max=72"`
}

type UserResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Roles      []string  `json:"roles"`
	Avatar     *string   `json:"avatar"`
	University *string   `json:"university"`
	IsActive   bool      `json:"isActive"`
	CreatedAt  time.Time `json:"createdAt"`
}

type sessionResponse struct {
	Token     string       `json:"token"`
	TokenType string       `json:"tokenType"`
	ExpiresIn int          `json:"expiresIn"`
	User      UserResponse `json:"user"`
}

func ToUserResponse(u *User) UserResponse {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	return UserResponse{
		ID:         u.ID,
		Name:       u.Name,
		Email:      u.Email,
		Role:       PrimaryRole(roles),
		Roles:      roles,
		Avatar:     u.AvatarURL,
		University: u.University,
		IsActive:   u.IsActive,
		CreatedAt:  u.CreatedAt,
	}
}

func (h *Handler) login(c *gin.Context) {
	var in loginRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	s, err := h.svc.Login(c.Request.Context(), in.Email, in.Password, meta(c))
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.Error(c, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password", nil)
	case errors.Is(err, ErrAccountDisabled):
		httpx.Error(c, http.StatusForbidden, "account_disabled", "This account has been deactivated", nil)
	case err != nil:
		httpx.Internal(c, err)
	default:
		h.respondSession(c, s)
	}
}

func (h *Handler) refresh(c *gin.Context) {
	raw, _ := c.Cookie(RefreshCookieName)
	s, err := h.svc.Refresh(c.Request.Context(), raw, meta(c))
	switch {
	case errors.Is(err, ErrSessionInvalid), errors.Is(err, ErrAccountDisabled):
		h.setCookie(c, "", -1)
		httpx.Error(c, http.StatusUnauthorized, "invalid_refresh_token", "Session expired, please sign in again", nil)
	case err != nil:
		httpx.Internal(c, err)
	default:
		h.respondSession(c, s)
	}
}

func (h *Handler) logout(c *gin.Context) {
	raw, _ := c.Cookie(RefreshCookieName)
	if err := h.svc.Logout(c.Request.Context(), raw); err != nil {
		httpx.Internal(c, err)
		return
	}
	h.setCookie(c, "", -1)
	httpx.NoContent(c)
}

func (h *Handler) me(c *gin.Context) {
	user, err := h.svc.User(c.Request.Context(), CurrentPrincipal(c).UserID)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	httpx.Data(c, http.StatusOK, ToUserResponse(user))
}

func (h *Handler) respondSession(c *gin.Context, s *Session) {
	h.setCookie(c, s.RefreshToken, int(h.opts.RefreshTTL.Seconds()))
	c.Header("Cache-Control", "no-store")
	httpx.Data(c, http.StatusOK, sessionResponse{
		Token:     s.AccessToken,
		TokenType: "Bearer",
		ExpiresIn: int(h.svc.tokens.TTL().Seconds()),
		User:      ToUserResponse(s.User),
	})
}

func (h *Handler) setCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    value,
		Path:     RefreshCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.opts.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func meta(c *gin.Context) SessionMeta {
	return SessionMeta{UserAgent: c.Request.UserAgent(), IP: c.ClientIP()}
}
