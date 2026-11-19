package communication

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(g *gin.RouterGroup, auth gin.HandlerFunc) {
	a := g.Group("", auth)
	a.GET("/notifications", h.list)
	a.PATCH("/notifications/:id/read", h.read)
	a.POST("/notifications/read-all", h.readAll)
	a.GET("/announcements", h.announcements)
	a.POST("/announcements", h.createAnnouncement)
	a.DELETE("/announcements/:id", h.deleteAnnouncement)
}

func viewer(c *gin.Context) learning.Viewer {
	return learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

func (h *Handler) list(c *gin.Context) {
	unread, ok := httpx.QueryBool(c, "unread")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, unreadCount, err := h.svc.Notifications(c.Request.Context(), identity.CurrentPrincipal(c).UserID, unread != nil && *unread, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	c.Header("X-Unread-Count", strconv.Itoa(unreadCount))
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) read(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	n, err := h.svc.MarkRead(c.Request.Context(), identity.CurrentPrincipal(c).UserID, id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, n)
}

func (h *Handler) readAll(c *gin.Context) {
	n, err := h.svc.MarkAllRead(c.Request.Context(), identity.CurrentPrincipal(c).UserID)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, gin.H{"updated": n})
}

func (h *Handler) announcements(c *gin.Context) {
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.Announcements(c.Request.Context(), viewer(c), courseID, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

type announcementRequest struct {
	Title       string   `json:"title" binding:"required,min=1,max=200"`
	Content     string   `json:"content" binding:"required,min=1,max=20000"`
	CourseID    *string  `json:"courseId" binding:"omitnil,uuid"`
	TargetRoles []string `json:"targetRoles" binding:"omitempty,max=5,dive,oneof=admin instructor mentor tracker student"`
}

func (h *Handler) createAnnouncement(c *gin.Context) {
	var in announcementRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	a, err := h.svc.CreateAnnouncement(c.Request.Context(), viewer(c), AnnouncementInput{
		Title: in.Title, Content: in.Content, CourseID: in.CourseID, TargetRoles: in.TargetRoles,
	})
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, a)
}

func (h *Handler) deleteAnnouncement(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	if writeError(c, h.svc.DeleteAnnouncement(c.Request.Context(), viewer(c), id)) {
		return
	}
	httpx.NoContent(c)
}

func writeError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound), errors.Is(err, learning.ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrForbidden), errors.Is(err, learning.ErrForbidden):
		identity.Forbidden(c)
	default:
		httpx.Internal(c, err)
	}
	return true
}
