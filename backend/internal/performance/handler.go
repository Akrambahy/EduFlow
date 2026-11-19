package performance

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
	a.GET("/courses/:id/grade-policy", h.getPolicy)
	a.PUT("/courses/:id/grade-policy", h.putPolicy)
	a.GET("/courses/:id/grade-policy/versions", h.versions)
	a.GET("/courses/:id/standing", h.courseStanding)
	a.POST("/courses/:id/standing/snapshots", h.snapshot)
	a.GET("/students/:id/grades", h.grades)
	a.GET("/students/:id/standing", h.standing)
	a.GET("/at-risk", h.atRisk)
}

func viewer(c *gin.Context) learning.Viewer {
	return learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

type policyRequest struct {
	QuizWeight          *int    `json:"quizWeight" binding:"required,min=0,max=100"`
	AssignmentWeight    *int    `json:"assignmentWeight" binding:"required,min=0,max=100"`
	AttendanceWeight    *int    `json:"attendanceWeight" binding:"required,min=0,max=100"`
	ProjectWeight       *int    `json:"projectWeight" binding:"required,min=0,max=100"`
	WarningGrade        *int    `json:"warningGrade" binding:"omitnil,min=0,max=100"`
	ProbationGrade      *int    `json:"probationGrade" binding:"omitnil,min=0,max=100"`
	WarningAttendance   *int    `json:"warningAttendance" binding:"omitnil,min=0,max=100"`
	ProbationAttendance *int    `json:"probationAttendance" binding:"omitnil,min=0,max=100"`
	WarningMissing      *int    `json:"warningMissing" binding:"omitnil,min=1,max=100"`
	Reason              *string `json:"reason" binding:"omitnil,max=500"`
}

func pick(v *int, d int) int {
	if v == nil {
		return d
	}
	return *v
}

func (h *Handler) getPolicy(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.Policy(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, p)
}

func (h *Handler) putPolicy(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in policyRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	d := DefaultPolicy(id)
	p := Policy{
		CourseID: id, QuizWeight: *in.QuizWeight, AssignmentWeight: *in.AssignmentWeight,
		AttendanceWeight: *in.AttendanceWeight, ProjectWeight: *in.ProjectWeight,
		WarningGrade: pick(in.WarningGrade, d.WarningGrade), ProbationGrade: pick(in.ProbationGrade, d.ProbationGrade),
		WarningAttendance: pick(in.WarningAttendance, d.WarningAttendance), ProbationAttendance: pick(in.ProbationAttendance, d.ProbationAttendance),
		WarningMissing: pick(in.WarningMissing, d.WarningMissing), Reason: in.Reason,
	}
	out, err := h.svc.SetPolicy(c.Request.Context(), viewer(c), id, p)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, out)
}

func (h *Handler) versions(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	list, err := h.svc.PolicyVersions(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, len(list))
	httpx.Data(c, http.StatusOK, list)
}

func standingQuery(c *gin.Context) (string, bool) {
	s := c.Query("standing")
	switch s {
	case "", StandingGood, StandingWarning, StandingProbation, "at_risk":
		return s, true
	}
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"standing": "must be one of: good warning probation at_risk"})
	return "", false
}

func paginate(c *gin.Context, list []StudentStanding) {
	page := httpx.ParsePage(c)
	httpx.SetTotal(c, len(list))
	start := page.Offset
	if start > len(list) {
		start = len(list)
	}
	end := start + page.Limit
	if end > len(list) {
		end = len(list)
	}
	out := list[start:end]
	if out == nil {
		out = []StudentStanding{}
	}
	httpx.Data(c, http.StatusOK, out)
}

func (h *Handler) courseStanding(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	standing, ok := standingQuery(c)
	if !ok {
		return
	}
	list, err := h.svc.CourseStanding(c.Request.Context(), viewer(c), id, standing)
	if writeError(c, err) {
		return
	}
	paginate(c, list)
}

func (h *Handler) snapshot(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	n, err := h.svc.Snapshot(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, gin.H{"courseId": id, "snapshots": n})
}

func (h *Handler) grades(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	g, err := h.svc.StudentGrades(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, g)
}

func (h *Handler) standing(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	g, err := h.svc.StudentGrades(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	type courseStanding struct {
		CourseID   string   `json:"courseId"`
		CourseName string   `json:"courseName"`
		Standing   string   `json:"standing"`
		Overall    *float64 `json:"overall"`
		Factors    []Factor `json:"factors"`
	}
	courses := make([]courseStanding, 0, len(g.Courses))
	for _, cg := range g.Courses {
		courses = append(courses, courseStanding{cg.CourseID, cg.CourseName, cg.Standing, cg.Overall, cg.Factors})
	}
	httpx.Data(c, http.StatusOK, gin.H{"studentId": id, "standing": g.Standing, "courses": courses})
}

func (h *Handler) atRisk(c *gin.Context) {
	list, err := h.svc.AtRisk(c.Request.Context(), viewer(c))
	if writeError(c, err) {
		return
	}
	paginate(c, list)
}

func writeError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound), errors.Is(err, learning.ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrForbidden), errors.Is(err, learning.ErrForbidden):
		identity.Forbidden(c)
	case errors.Is(err, ErrInvalidPolicy):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"weights": err.Error()})
	default:
		httpx.Internal(c, err)
	}
	return true
}
