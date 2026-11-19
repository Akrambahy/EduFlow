package sessions

import (
	"errors"
	"net/http"
	"time"

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
	a.GET("/sessions", h.list)
	a.GET("/sessions/:id", h.get)
	a.PATCH("/sessions/:id", h.update)
	a.POST("/courses/:id/sessions", h.create)

	a.GET("/courses/:id/materials", h.materials)
	a.POST("/courses/:id/materials", h.createMaterial)
	a.PATCH("/materials/:id", h.updateMaterial)
	a.DELETE("/materials/:id", h.deleteMaterial)
}

func viewer(c *gin.Context) learning.Viewer {
	return learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

type sessionRequest struct {
	Title           *string    `json:"title" binding:"omitnil,min=1,max=160"`
	Description     *string    `json:"description" binding:"omitnil,max=5000"`
	Type            *string    `json:"type" binding:"omitnil,oneof=lecture lab workshop review"`
	Location        *string    `json:"location" binding:"omitnil,max=200"`
	StartsAt        *time.Time `json:"startsAt"`
	EndsAt          *time.Time `json:"endsAt"`
	RecordingURL    *string    `json:"recordingUrl" binding:"omitnil,max=2048,url_or_empty"`
	RecordingFileID *string    `json:"recordingFileId" binding:"omitnil,uuid_or_empty"`
	Cancelled       *bool      `json:"cancelled"`
}

func (r sessionRequest) input() SessionInput {
	return SessionInput{
		Title: r.Title, Description: r.Description, Type: r.Type, Location: r.Location,
		StartsAt: r.StartsAt, EndsAt: r.EndsAt, RecordingURL: r.RecordingURL,
		RecordingFileID: r.RecordingFileID, Cancelled: r.Cancelled,
	}
}

type materialRequest struct {
	Title         *string `json:"title" binding:"omitnil,min=1,max=160"`
	Description   *string `json:"description" binding:"omitnil,max=5000"`
	URL           *string `json:"url" binding:"omitnil,url,max=2048"`
	FileID        *string `json:"fileId" binding:"omitnil,uuid"`
	SessionID     *string `json:"sessionId" binding:"omitnil,uuid"`
	RemoveSession bool    `json:"removeSession"`
}

func required(c *gin.Context, fields map[string]bool) bool {
	details := map[string]string{}
	for name, present := range fields {
		if !present {
			details[name] = "is required"
		}
	}
	if len(details) > 0 {
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", details)
		return false
	}
	return true
}

func parseTimeQuery(c *gin.Context, name string) (*time.Time, bool) {
	v := c.Query(name)
	if v == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		if d, derr := time.Parse("2006-01-02", v); derr == nil {
			t = d
		} else {
			httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{name: "must be an RFC 3339 timestamp or YYYY-MM-DD"})
			return nil, false
		}
	}
	return &t, true
}

func (h *Handler) list(c *gin.Context) {
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	from, ok := parseTimeQuery(c, "from")
	if !ok {
		return
	}
	to, ok := parseTimeQuery(c, "to")
	if !ok {
		return
	}
	status := c.Query("status")
	switch status {
	case "", StatusUpcoming, StatusCompleted, StatusCancelled:
	default:
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"status": "must be one of: upcoming completed cancelled"})
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.List(c.Request.Context(), viewer(c), Filter{CourseID: courseID, From: from, To: to, Status: status}, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) get(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	s, err := h.svc.Get(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, s)
}

func (h *Handler) create(c *gin.Context) {
	courseID, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in sessionRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if !required(c, map[string]bool{"title": in.Title != nil, "startsAt": in.StartsAt != nil, "endsAt": in.EndsAt != nil}) {
		return
	}
	s, err := h.svc.Create(c.Request.Context(), viewer(c), courseID, in.input())
	if writeError(c, err) {
		return
	}
	c.Header("Location", "/api/v1/sessions/"+s.ID)
	httpx.Data(c, http.StatusCreated, s)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in sessionRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	s, err := h.svc.Update(c.Request.Context(), viewer(c), id, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, s)
}

func (h *Handler) materials(c *gin.Context) {
	courseID, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	sessionID, ok := httpx.QueryUUID(c, "sessionId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.Materials(c.Request.Context(), viewer(c), courseID, sessionID, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) createMaterial(c *gin.Context) {
	courseID, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in materialRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if !required(c, map[string]bool{"title": in.Title != nil}) {
		return
	}
	m, err := h.svc.CreateMaterial(c.Request.Context(), viewer(c), courseID, MaterialInput{
		Title: in.Title, Description: in.Description, URL: in.URL, FileID: in.FileID, SessionID: in.SessionID,
	})
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, m)
}

func (h *Handler) updateMaterial(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in materialRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	m, err := h.svc.UpdateMaterial(c.Request.Context(), viewer(c), id, MaterialInput{
		Title: in.Title, Description: in.Description, SessionID: in.SessionID,
	}, in.RemoveSession)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, m)
}

func (h *Handler) deleteMaterial(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	if writeError(c, h.svc.DeleteMaterial(c.Request.Context(), viewer(c), id)) {
		return
	}
	httpx.NoContent(c)
}

func invalid(c *gin.Context, field string, err error) {
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{field: err.Error()})
}

func writeError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound), errors.Is(err, learning.ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, learning.ErrForbidden):
		identity.Forbidden(c)
	case errors.Is(err, ErrInvalidTimes):
		invalid(c, "endsAt", err)
	case errors.Is(err, ErrInvalidFile):
		invalid(c, "fileId", err)
	case errors.Is(err, ErrInvalidSession):
		invalid(c, "sessionId", err)
	case errors.Is(err, ErrSourceRequired):
		invalid(c, "url", err)
	default:
		httpx.Internal(c, err)
	}
	return true
}
