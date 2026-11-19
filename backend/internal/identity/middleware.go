package identity

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const principalKey = "identity.principal"

type Principal struct {
	UserID string
	Roles  []string
}

func (p *Principal) HasRole(roles ...string) bool {
	for _, want := range roles {
		for _, have := range p.Roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

func (p *Principal) IsAdmin() bool {
	return p.HasRole(RoleAdmin)
}

func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			unauthorized(c)
			return
		}
		user, err := s.Authenticate(c.Request.Context(), token)
		switch {
		case errors.Is(err, ErrInvalidToken), errors.Is(err, ErrAccountDisabled):
			unauthorized(c)
			return
		case err != nil:
			httpx.Internal(c, err)
			return
		}
		c.Set(principalKey, &Principal{UserID: user.ID, Roles: user.Roles})
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := CurrentPrincipal(c)
		if p == nil {
			unauthorized(c)
			return
		}
		if !p.HasRole(roles...) {
			Forbidden(c)
			return
		}
		c.Next()
	}
}

func CurrentPrincipal(c *gin.Context) *Principal {
	v, ok := c.Get(principalKey)
	if !ok {
		return nil
	}
	p, _ := v.(*Principal)
	return p
}

func Forbidden(c *gin.Context) {
	httpx.Error(c, http.StatusForbidden, "forbidden", "You do not have permission to perform this action", nil)
}

func unauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", `Bearer realm="eduflow"`)
	httpx.Error(c, http.StatusUnauthorized, "unauthorized", "Authentication required", nil)
}
