package learning

import (
	"errors"
	"net/http"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
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

	a.GET("/tracks", h.listTracks)
	a.POST("/tracks", h.createTrack)
	a.GET("/tracks/:id", h.getTrack)
	a.PATCH("/tracks/:id", h.updateTrack)
	a.PUT("/tracks/:id/mentors", h.setStaff(identity.RoleMentor))
	a.PUT("/tracks/:id/trackers", h.setStaff(identity.RoleTracker))

	a.GET("/courses", h.listCourses)
	a.POST("/courses", h.createCourse)
	a.GET("/courses/:id", h.getCourse)
	a.PATCH("/courses/:id", h.updateCourse)
	a.GET("/courses/:id/modules", h.outline)
	a.POST("/courses/:id/modules", h.createModule)
	a.POST("/courses/:id/enrollments", h.enroll)
	a.DELETE("/courses/:id/enrollments/:studentId", h.withdraw)

	a.PATCH("/modules/:id", h.updateModule)
	a.DELETE("/modules/:id", h.deleteModule)
	a.POST("/modules/:id/lessons", h.createLesson)

	a.GET("/lessons/:id", h.getLesson)
	a.PATCH("/lessons/:id", h.updateLesson)
	a.DELETE("/lessons/:id", h.deleteLesson)
	a.POST("/lessons/:id/progress", h.progress)

	a.GET("/enrollments", h.listEnrollments)
}

func viewer(c *gin.Context) Viewer {
	return NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

type trackRequest struct {
	Name        *string `json:"name" binding:"omitnil,min=1,max=120"`
	Description *string `json:"description" binding:"omitnil,max=2000"`
	Status      *string `json:"status" binding:"omitnil,oneof=draft active archived"`
}

type staffRequest struct {
	UserIDs []string `json:"userIds" binding:"required,max=20,dive,uuid"`
}

type courseRequest struct {
	TrackID          *string `json:"trackId" binding:"omitnil,uuid"`
	InstructorID     *string `json:"instructorId" binding:"omitnil,uuid"`
	Title            *string `json:"title" binding:"omitnil,min=1,max=160"`
	Description      *string `json:"description" binding:"omitnil,max=5000"`
	Category         *string `json:"category" binding:"omitnil,min=1,max=50"`
	Color            *string `json:"color" binding:"omitnil,oneof=teal emerald sky amber violet rose"`
	DurationWeeks    *int    `json:"durationWeeks" binding:"omitnil,min=1,max=104"`
	SequentialUnlock *bool   `json:"sequentialUnlock"`
	Status           *string `json:"status" binding:"omitnil,oneof=draft published archived"`
	RemoveTrack      bool    `json:"removeTrack"`
}

type moduleRequest struct {
	Title    *string `json:"title" binding:"omitnil,min=1,max=160"`
	Position *int    `json:"position" binding:"omitnil,min=1"`
}

type lessonRequest struct {
	Title           *string `json:"title" binding:"omitnil,min=1,max=160"`
	Type            *string `json:"type" binding:"omitnil,oneof=video text project"`
	DurationMinutes *int    `json:"durationMinutes" binding:"omitnil,min=1,max=1440"`
	Content         *string `json:"content" binding:"omitnil,max=100000"`
	VideoURL        *string `json:"videoUrl" binding:"omitnil,max=2048,url_or_empty"`
	Position        *int    `json:"position" binding:"omitnil,min=1"`
}

type enrollRequest struct {
	StudentID string `json:"studentId" binding:"required,uuid"`
}

type progressRequest struct {
	Status string `json:"status" binding:"required,oneof=in_progress completed"`
}

func (h *Handler) listTracks(c *gin.Context) {
	status := c.Query("status")
	if status != "" && status != TrackDraft && status != TrackActive && status != TrackArchived {
		invalidQuery(c, "status", "must be one of: draft active archived")
		return
	}
	page := httpx.ParsePage(c)
	tracks, total, err := h.svc.ListTracks(c.Request.Context(), viewer(c), status, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, tracks)
}

func (h *Handler) getTrack(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	t, err := h.svc.Track(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

func (h *Handler) createTrack(c *gin.Context) {
	var in trackRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if in.Name == nil {
		invalidQuery(c, "name", "is required")
		return
	}
	status := TrackDraft
	if in.Status != nil {
		status = *in.Status
	}
	desc := ""
	if in.Description != nil {
		desc = *in.Description
	}
	t, err := h.svc.CreateTrack(c.Request.Context(), viewer(c), *in.Name, desc, status)
	if writeError(c, err) {
		return
	}
	c.Header("Location", "/api/v1/tracks/"+t.ID)
	httpx.Data(c, http.StatusCreated, t)
}

func (h *Handler) updateTrack(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in trackRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	t, err := h.svc.UpdateTrack(c.Request.Context(), viewer(c), id, in.Name, in.Description, in.Status)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

func (h *Handler) setStaff(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := httpx.PathUUID(c, "id")
		if !ok {
			return
		}
		var in staffRequest
		if !httpx.BindJSON(c, &in) {
			return
		}
		t, err := h.svc.SetTrackStaff(c.Request.Context(), viewer(c), id, role, in.UserIDs)
		if writeError(c, err) {
			return
		}
		httpx.Data(c, http.StatusOK, t)
	}
}

func (h *Handler) listCourses(c *gin.Context) {
	status := c.Query("status")
	if status != "" && status != CourseDraft && status != CoursePublished && status != CourseArchived {
		invalidQuery(c, "status", "must be one of: draft published archived")
		return
	}
	trackID, ok := httpx.QueryUUID(c, "trackId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	courses, total, err := h.svc.ListCourses(c.Request.Context(), viewer(c), CourseFilter{
		Status: status, TrackID: trackID, Category: c.Query("category"), Query: c.Query("q"),
	}, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, courses)
}

func (h *Handler) getCourse(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	course, err := h.svc.Course(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, course)
}

func (r courseRequest) input() CourseInput {
	return CourseInput{
		TrackID: r.TrackID, InstructorID: r.InstructorID, Title: r.Title, Description: r.Description,
		Category: r.Category, Color: r.Color, DurationWeeks: r.DurationWeeks,
		SequentialUnlock: r.SequentialUnlock, Status: r.Status,
	}
}

func (h *Handler) createCourse(c *gin.Context) {
	var in courseRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if in.Title == nil {
		invalidQuery(c, "title", "is required")
		return
	}
	course, err := h.svc.CreateCourse(c.Request.Context(), viewer(c), in.input())
	if writeError(c, err) {
		return
	}
	c.Header("Location", "/api/v1/courses/"+course.ID)
	httpx.Data(c, http.StatusCreated, course)
}

func (h *Handler) updateCourse(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in courseRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	course, err := h.svc.UpdateCourse(c.Request.Context(), viewer(c), id, in.input(), in.RemoveTrack)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, course)
}

func (h *Handler) outline(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	modules, err := h.svc.Outline(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, modules)
}

func (h *Handler) createModule(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in moduleRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if in.Title == nil {
		invalidQuery(c, "title", "is required")
		return
	}
	modules, err := h.svc.CreateModule(c.Request.Context(), viewer(c), id, *in.Title, in.Position)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, modules)
}

func (h *Handler) updateModule(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in moduleRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	modules, err := h.svc.UpdateModule(c.Request.Context(), viewer(c), id, in.Title, in.Position)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, modules)
}

func (h *Handler) deleteModule(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	if writeError(c, h.svc.DeleteModule(c.Request.Context(), viewer(c), id)) {
		return
	}
	httpx.NoContent(c)
}

func (r lessonRequest) input() LessonInput {
	return LessonInput{Title: r.Title, Type: r.Type, DurationMinutes: r.DurationMinutes, Content: r.Content, VideoURL: r.VideoURL, Position: r.Position}
}

func (h *Handler) createLesson(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in lessonRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	if in.Title == nil {
		invalidQuery(c, "title", "is required")
		return
	}
	modules, err := h.svc.CreateLesson(c.Request.Context(), viewer(c), id, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, modules)
}

func (h *Handler) getLesson(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	lesson, err := h.svc.Lesson(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, lesson)
}

func (h *Handler) updateLesson(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in lessonRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	modules, err := h.svc.UpdateLesson(c.Request.Context(), viewer(c), id, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, modules)
}

func (h *Handler) deleteLesson(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	if writeError(c, h.svc.DeleteLesson(c.Request.Context(), viewer(c), id)) {
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) progress(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in progressRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	e, err := h.svc.RecordProgress(c.Request.Context(), viewer(c), id, in.Status == LessonCompleted)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, e)
}

func (h *Handler) listEnrollments(c *gin.Context) {
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	studentID, ok := httpx.QueryUUID(c, "studentId")
	if !ok {
		return
	}
	status := c.Query("status")
	switch status {
	case "", "all", EnrollmentActive, EnrollmentCompleted, EnrollmentWithdrawn:
	default:
		invalidQuery(c, "status", "must be one of: active completed withdrawn all")
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.ListEnrollments(c.Request.Context(), viewer(c), EnrollmentFilter{CourseID: courseID, StudentID: studentID, Status: status}, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) enroll(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in enrollRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	e, err := h.svc.Enroll(c.Request.Context(), viewer(c), id, in.StudentID)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, e)
}

func (h *Handler) withdraw(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	studentID, ok := httpx.PathUUID(c, "studentId")
	if !ok {
		return
	}
	if writeError(c, h.svc.Withdraw(c.Request.Context(), viewer(c), id, studentID)) {
		return
	}
	httpx.NoContent(c)
}

func invalidQuery(c *gin.Context, field, msg string) {
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{field: msg})
}

func writeError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrForbidden):
		identity.Forbidden(c)
	case errors.Is(err, ErrTrackNameTaken):
		httpx.Error(c, http.StatusConflict, "track_name_taken", "A track with this name already exists", nil)
	case errors.Is(err, ErrAlreadyEnrolled):
		httpx.Error(c, http.StatusConflict, "already_enrolled", "The student is already enrolled in this course", nil)
	case errors.Is(err, ErrNotEnrolled):
		httpx.Error(c, http.StatusConflict, "not_enrolled", "The student is not enrolled in this course", nil)
	case errors.Is(err, ErrCourseArchived):
		httpx.Error(c, http.StatusConflict, "course_archived", "The course is archived", nil)
	case errors.Is(err, ErrLessonLocked):
		httpx.Error(c, http.StatusForbidden, "lesson_locked", "Complete the previous lessons first", nil)
	case errors.Is(err, ErrModuleNotEmpty):
		httpx.Error(c, http.StatusConflict, "module_not_empty", "Remove the module's lessons first", nil)
	case errors.Is(err, ErrLessonHasProgress):
		httpx.Error(c, http.StatusConflict, "lesson_has_progress", "Students have progress on this lesson", nil)
	case errors.Is(err, ErrInvalidAssignee):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"userIds": ErrInvalidAssignee.Error()})
	case errors.Is(err, ErrInvalidInstructor):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"instructorId": ErrInvalidInstructor.Error()})
	case errors.Is(err, ErrNotStudent):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"studentId": ErrNotStudent.Error()})
	case errors.Is(err, ErrInvalidReference):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"trackId": ErrInvalidReference.Error()})
	default:
		httpx.Internal(c, err)
	}
	return true
}
