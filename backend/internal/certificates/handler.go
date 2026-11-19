package certificates

import (
	"context"
	"errors"
	"net/http"

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

func (h *Handler) Register(g *gin.RouterGroup, auth, verifyLimit gin.HandlerFunc) {
	g.GET("/certificates/verify/:code", verifyLimit, h.verify)

	a := g.Group("", auth)
	a.GET("/courses/:id/certificate-requirements", h.getRequirements)
	a.PUT("/courses/:id/certificate-requirements", h.putRequirements)
	a.GET("/courses/:id/certificate-eligibility", h.courseEligibility)
	a.GET("/students/:id/certificate-eligibility", h.studentEligibility)
	a.POST("/certificates/issue", h.issue)
	a.GET("/certificates", h.list)
	a.GET("/certificates/:id", h.get)
	a.POST("/certificates/:id/revoke", h.revoke)
	a.POST("/certificates/:id/reinstate", h.reinstate)
	a.GET("/certificates/:id/events", h.events)
}

func viewer(c *gin.Context) learning.Viewer {
	return learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

func (h *Handler) verify(c *gin.Context) {
	p, err := h.svc.Verify(c.Request.Context(), c.Param("code"))
	if writeError(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	httpx.Data(c, http.StatusOK, p)
}

type requirementsRequest struct {
	MinGrade          *int  `json:"minGrade" binding:"required,min=0,max=100"`
	MinAttendance     *int  `json:"minAttendance" binding:"required,min=0,max=100"`
	RequireAllLessons *bool `json:"requireAllLessons" binding:"required"`
	AllowProbation    *bool `json:"allowProbation" binding:"required"`
}

func (h *Handler) getRequirements(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	r, err := h.svc.GetRequirements(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, r)
}

func (h *Handler) putRequirements(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in requirementsRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	r, err := h.svc.SetRequirements(c.Request.Context(), viewer(c), Requirements{
		CourseID: id, MinGrade: *in.MinGrade, MinAttendance: *in.MinAttendance,
		RequireAllLessons: *in.RequireAllLessons, AllowProbation: *in.AllowProbation,
	})
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, r)
}

func (h *Handler) courseEligibility(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	list, err := h.svc.CourseEligibility(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, len(list))
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) studentEligibility(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	courseID := c.Query("courseId")
	if !httpx.IsUUID(courseID) {
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"courseId": "is required"})
		return
	}
	e, err := h.svc.StudentEligibility(c.Request.Context(), viewer(c), courseID, id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, e)
}

type issueRequest struct {
	CourseID  string `json:"courseId" binding:"required,uuid"`
	StudentID string `json:"studentId" binding:"omitempty,uuid"`
}

func (h *Handler) issue(c *gin.Context) {
	var in issueRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	res, err := h.svc.Issue(c.Request.Context(), viewer(c), in.CourseID, in.StudentID)
	if errors.Is(err, ErrNotEligible) && res != nil && len(res.Skipped) == 1 {
		httpx.Error(c, http.StatusConflict, "not_eligible", err.Error(), map[string]any{"reasons": res.Skipped[0].Reasons})
		return
	}
	if writeError(c, err) {
		return
	}
	status := http.StatusOK
	if len(res.Issued) > 0 {
		status = http.StatusCreated
	}
	httpx.Data(c, status, res)
}

func (h *Handler) list(c *gin.Context) {
	f := Filter{Status: c.Query("status")}
	if f.Status != "" && f.Status != StatusIssued && f.Status != StatusRevoked {
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"status": "must be one of: issued revoked"})
		return
	}
	var ok bool
	if f.CourseID, ok = httpx.QueryUUID(c, "courseId"); !ok {
		return
	}
	if f.StudentID, ok = httpx.QueryUUID(c, "studentId"); !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.List(c.Request.Context(), viewer(c), f, page)
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
	cert, err := h.svc.Certificate(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, cert)
}

type reasonRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

func (h *Handler) revoke(c *gin.Context) {
	h.changeStatus(c, h.svc.Revoke)
}

func (h *Handler) reinstate(c *gin.Context) {
	h.changeStatus(c, h.svc.Reinstate)
}

func (h *Handler) changeStatus(c *gin.Context, fn func(context.Context, learning.Viewer, string, string) (*Certificate, error)) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in reasonRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	cert, err := fn(c.Request.Context(), viewer(c), id, in.Reason)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, cert)
}

func (h *Handler) events(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	list, err := h.svc.Events(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, list)
}

func writeError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound), errors.Is(err, learning.ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrForbidden), errors.Is(err, learning.ErrForbidden):
		identity.Forbidden(c)
	case errors.Is(err, ErrAlreadyIssued):
		httpx.Error(c, http.StatusConflict, "already_issued", err.Error(), nil)
	case errors.Is(err, ErrNotEligible):
		httpx.Error(c, http.StatusConflict, "not_eligible", err.Error(), nil)
	case errors.Is(err, ErrBadState):
		httpx.Error(c, http.StatusConflict, "invalid_certificate_state", err.Error(), nil)
	case errors.Is(err, ErrNotEnrolled):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"studentId": err.Error()})
	default:
		httpx.Internal(c, err)
	}
	return true
}
