package identity

import (
	"errors"
	"net/http"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UsersHandler struct {
	svc   *Service
	audit *audit.Store
}

func NewUsersHandler(svc *Service, db *pgxpool.Pool) *UsersHandler {
	return &UsersHandler{svc: svc, audit: audit.NewStore(db)}
}

func (h *UsersHandler) Register(g *gin.RouterGroup) {
	authed := g.Group("", h.svc.RequireAuth())
	admin := authed.Group("", RequireRoles(RoleAdmin))

	authed.GET("/users/me", h.getMe)
	authed.PATCH("/users/me", h.updateMe)
	authed.POST("/users/me/password", h.changePassword)
	authed.GET("/users/:id", h.getUser)

	admin.GET("/users", h.listUsers)
	admin.POST("/users", h.createUser)
	admin.PATCH("/users/:id", h.updateUser)
	admin.PUT("/users/:id/roles", h.setRoles)
	admin.POST("/users/:id/password", h.resetPassword)
	admin.GET("/audit-logs", h.listAudit)
}

type createUserRequest struct {
	Name       string   `json:"name" binding:"required,min=1,max=120"`
	Email      string   `json:"email" binding:"required,email,max=254"`
	Password   string   `json:"password" binding:"required,min=8,max=72"`
	Roles      []string `json:"roles" binding:"required,min=1,max=5,dive,oneof=admin instructor mentor tracker student"`
	University *string  `json:"university" binding:"omitnil,max=200"`
	Avatar     *string  `json:"avatar" binding:"omitnil,max=2048,url_or_empty"`
}

type updateUserRequest struct {
	Name       *string `json:"name" binding:"omitnil,min=1,max=120"`
	Email      *string `json:"email" binding:"omitnil,email,max=254"`
	University *string `json:"university" binding:"omitnil,max=200"`
	Avatar     *string `json:"avatar" binding:"omitnil,max=2048,url_or_empty"`
	IsActive   *bool   `json:"isActive"`
}

type updateProfileRequest struct {
	Name       *string `json:"name" binding:"omitnil,min=1,max=120"`
	University *string `json:"university" binding:"omitnil,max=200"`
	Avatar     *string `json:"avatar" binding:"omitnil,max=2048,url_or_empty"`
}

type setRolesRequest struct {
	Roles []string `json:"roles" binding:"required,min=1,max=5,dive,oneof=admin instructor mentor tracker student"`
}

type resetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required,max=72"`
	NewPassword     string `json:"newPassword" binding:"required,min=8,max=72"`
}

func (h *UsersHandler) listUsers(c *gin.Context) {
	role := c.Query("role")
	if role != "" && !IsValidRole(role) {
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"role": "must be a known role"})
		return
	}
	active, ok := httpx.QueryBool(c, "isActive")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	users, total, err := h.svc.ListUsers(c.Request.Context(), UserFilter{Role: role, Query: c.Query("q"), IsActive: active}, page)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	out := make([]UserResponse, 0, len(users))
	for i := range users {
		out = append(out, ToUserResponse(&users[i]))
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, out)
}

func (h *UsersHandler) createUser(c *gin.Context) {
	var in createUserRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	u, err := h.svc.CreateUser(c.Request.Context(), actor(c), CreateUserInput{
		Name: in.Name, Email: in.Email, Password: in.Password, Roles: in.Roles,
		University: in.University, AvatarURL: in.Avatar,
	})
	if writeUserError(c, err) {
		return
	}
	c.Header("Location", "/api/v1/users/"+u.ID)
	httpx.Data(c, http.StatusCreated, ToUserResponse(u))
}

func (h *UsersHandler) getUser(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	p := CurrentPrincipal(c)
	if p.UserID != id && !p.IsAdmin() {
		Forbidden(c)
		return
	}
	u, err := h.svc.User(c.Request.Context(), id)
	if writeUserError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, ToUserResponse(u))
}

func (h *UsersHandler) getMe(c *gin.Context) {
	u, err := h.svc.User(c.Request.Context(), CurrentPrincipal(c).UserID)
	if writeUserError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, ToUserResponse(u))
}

func (h *UsersHandler) updateMe(c *gin.Context) {
	var in updateProfileRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	u, err := h.svc.UpdateProfile(c.Request.Context(), actor(c), UserPatch{Name: in.Name, University: in.University, AvatarURL: in.Avatar})
	if writeUserError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, ToUserResponse(u))
}

func (h *UsersHandler) updateUser(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in updateUserRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	u, err := h.svc.UpdateUser(c.Request.Context(), actor(c), id, UserPatch{
		Name: in.Name, Email: in.Email, University: in.University, AvatarURL: in.Avatar, IsActive: in.IsActive,
	})
	if writeUserError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, ToUserResponse(u))
}

func (h *UsersHandler) setRoles(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in setRolesRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	u, err := h.svc.SetRoles(c.Request.Context(), actor(c), id, in.Roles)
	if writeUserError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, ToUserResponse(u))
}

func (h *UsersHandler) resetPassword(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in resetPasswordRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if writeUserError(c, h.svc.ResetPassword(c.Request.Context(), actor(c), id, in.Password)) {
		return
	}
	httpx.NoContent(c)
}

func (h *UsersHandler) changePassword(c *gin.Context) {
	var in changePasswordRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if writeUserError(c, h.svc.ChangePassword(c.Request.Context(), actor(c), in.CurrentPassword, in.NewPassword)) {
		return
	}
	httpx.NoContent(c)
}

func (h *UsersHandler) listAudit(c *gin.Context) {
	targetID, ok := httpx.QueryUUID(c, "targetId")
	if !ok {
		return
	}
	actorID, ok := httpx.QueryUUID(c, "actorId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	entries, total, err := h.audit.List(c.Request.Context(), audit.Filter{
		Action: c.Query("action"), TargetType: c.Query("targetType"), TargetID: targetID, ActorID: actorID,
	}, page)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, entries)
}

func actor(c *gin.Context) Actor {
	return Actor{ID: CurrentPrincipal(c).UserID, IP: c.ClientIP()}
}

func writeUserError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrEmailTaken):
		httpx.Error(c, http.StatusConflict, "email_taken", "A user with this email already exists", nil)
	case errors.Is(err, ErrSelfModification):
		httpx.Error(c, http.StatusForbidden, "self_modification", "You cannot change your own roles or activation status", nil)
	case errors.Is(err, ErrInvalidRoles):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"roles": "must be a non-empty list of known roles"})
	case errors.Is(err, ErrWeakPassword):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"password": ErrWeakPassword.Error()})
	case errors.Is(err, ErrWrongPassword):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"currentPassword": "is incorrect"})
	case errors.Is(err, ErrPasswordUnchanged):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"newPassword": "must differ from the current password"})
	default:
		httpx.Internal(c, err)
	}
	return true
}
