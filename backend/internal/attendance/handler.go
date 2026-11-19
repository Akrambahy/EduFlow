package attendance

import (
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

func (h *Handler) Register(g *gin.RouterGroup, auth gin.HandlerFunc) {
	a := g.Group("", auth)
	a.POST("/sessions/:id/attendance/start", h.start)
	a.GET("/sessions/:id/attendance/token", h.token)
	a.POST("/sessions/:id/attendance/close", h.close)
	a.GET("/sessions/:id/attendance", h.window)
	a.GET("/sessions/:id/attendance/records", h.records)
	a.POST("/sessions/:id/attendance/records", h.manual)
	a.PATCH("/attendance/records/:id", h.correct)
	a.POST("/attendance/check-in", h.checkIn)
	a.GET("/students/:id/attendance", h.studentRecords)
	a.GET("/students/:id/attendance/summary", h.studentSummary)
}

func viewer(c *gin.Context) learning.Viewer {
	return learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

type startRequest struct {
	GraceMinutes *int `json:"graceMinutes" binding:"omitnil,min=0,max=240"`
}

type checkInRequest struct {
	Token string `json:"token" binding:"required,max=128"`
}

type manualRequest struct {
	StudentID string `json:"studentId" binding:"required,uuid"`
	Status    string `json:"status" binding:"required,oneof=present late absent excused"`
	Reason    string `json:"reason" binding:"required,min=3,max=500"`
}

type correctRequest struct {
	Status string `json:"status" binding:"required,oneof=present late absent excused"`
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

func (h *Handler) start(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in startRequest
	if c.Request.ContentLength != 0 && !httpx.BindJSON(c, &in) {
		return
	}
	w, err := h.svc.Start(c.Request.Context(), viewer(c), id, in.GraceMinutes)
	if writeError(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	httpx.Data(c, http.StatusOK, w)
}

func (h *Handler) token(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	w, err := h.svc.Token(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	httpx.Data(c, http.StatusOK, w)
}

func (h *Handler) close(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	w, err := h.svc.Close(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, w)
}

func (h *Handler) window(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	w, err := h.svc.Window(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, w)
}

func validStatusQuery(c *gin.Context) (string, bool) {
	status := c.Query("status")
	switch status {
	case "", StatusPresent, StatusLate, StatusAbsent, StatusExcused:
		return status, true
	}
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"status": "must be one of: present late absent excused"})
	return "", false
}

func (h *Handler) records(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	status, ok := validStatusQuery(c)
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.Records(c.Request.Context(), viewer(c), id, status, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) manual(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in manualRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	rec, err := h.svc.Manual(c.Request.Context(), viewer(c), id, in.StudentID, in.Status, in.Reason)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, rec)
}

func (h *Handler) correct(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in correctRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	rec, err := h.svc.Correct(c.Request.Context(), viewer(c), id, in.Status, in.Reason)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, rec)
}

func (h *Handler) checkIn(c *gin.Context) {
	var in checkInRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	rec, err := h.svc.CheckIn(c.Request.Context(), viewer(c), in.Token)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, rec)
}

func (h *Handler) studentRecords(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.StudentRecords(c.Request.Context(), viewer(c), id, courseID, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) studentSummary(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	sum, err := h.svc.StudentSummary(c.Request.Context(), viewer(c), id, courseID)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, sum)
}

func writeError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound), errors.Is(err, learning.ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrForbidden), errors.Is(err, learning.ErrForbidden):
		identity.Forbidden(c)
	case errors.Is(err, ErrTokenExpired):
		httpx.Error(c, http.StatusBadRequest, "attendance_token_expired", "The QR code has expired, scan the current one", nil)
	case errors.Is(err, ErrTokenInvalid):
		httpx.Error(c, http.StatusBadRequest, "attendance_token_invalid", "The QR code is not valid", nil)
	case errors.Is(err, ErrWindowClosed):
		httpx.Error(c, http.StatusConflict, "attendance_closed", "Attendance is closed for this session", nil)
	case errors.Is(err, ErrWindowNotOpen):
		httpx.Error(c, http.StatusConflict, "attendance_not_started", "Attendance has not been started for this session", nil)
	case errors.Is(err, ErrSessionCancelled):
		httpx.Error(c, http.StatusConflict, "session_cancelled", "The session is cancelled", nil)
	case errors.Is(err, ErrAlreadyRecorded):
		httpx.Error(c, http.StatusConflict, "already_checked_in", "Attendance is already recorded for this student", nil)
	case errors.Is(err, ErrNotEnrolled):
		httpx.Error(c, http.StatusForbidden, "not_enrolled", "You are not enrolled in this course", nil)
	case errors.Is(err, ErrNotStudent):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"studentId": err.Error()})
	default:
		httpx.Internal(c, err)
	}
	return true
}
