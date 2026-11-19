package projects

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
	a.GET("/projects", h.list)
	a.POST("/courses/:id/projects", h.create)
	a.GET("/projects/:id", h.get)
	a.PATCH("/projects/:id", h.update)
	a.GET("/projects/:id/teams", h.teams)
	a.POST("/projects/:id/teams", h.createTeam)
	a.POST("/projects/:id/submissions", h.submit)
	a.GET("/teams/:id", h.team)
	a.POST("/teams/:id/members", h.addMember)
	a.DELETE("/teams/:id/members/:studentId", h.removeMember)
	a.PUT("/teams/:id/repository", h.linkRepo)
	a.POST("/teams/:id/repository/sync", h.syncRepo)
	a.PUT("/teams/:id/presentation", h.schedule)
	a.PATCH("/presentations/:id", h.updatePresentation)
	a.GET("/project-submissions", h.submissions)
	a.POST("/project-submissions/:id/grade", h.grade)
}

func viewer(c *gin.Context) learning.Viewer {
	return learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

type criterionRequest struct {
	Title     string `json:"title" binding:"required,min=1,max=120"`
	MaxPoints int    `json:"maxPoints" binding:"required,min=1,max=1000"`
}

type projectRequest struct {
	Title                *string            `json:"title" binding:"omitnil,min=1,max=160"`
	Description          *string            `json:"description" binding:"omitnil,max=10000"`
	Requirements         *string            `json:"requirements" binding:"omitnil,max=20000"`
	Status               *string            `json:"status" binding:"omitnil,oneof=draft published"`
	DueAt                *time.Time         `json:"dueAt"`
	MaxGrade             *int               `json:"maxGrade" binding:"omitnil,min=1,max=1000"`
	TeamSize             *int               `json:"teamSize" binding:"omitnil,min=1,max=10"`
	RequiresPresentation *bool              `json:"requiresPresentation"`
	AllowResubmission    *bool              `json:"allowResubmission"`
	Criteria             []criterionRequest `json:"criteria" binding:"omitempty,max=20,dive"`
}

func (r projectRequest) input() ProjectInput {
	var criteria []Criterion
	if r.Criteria != nil {
		criteria = make([]Criterion, 0, len(r.Criteria))
		for _, c := range r.Criteria {
			criteria = append(criteria, Criterion{Title: c.Title, MaxPoints: c.MaxPoints})
		}
	}
	return ProjectInput{Title: r.Title, Description: r.Description, Requirements: r.Requirements, Status: r.Status, DueAt: r.DueAt,
		MaxGrade: r.MaxGrade, TeamSize: r.TeamSize, RequiresPresentation: r.RequiresPresentation, AllowResubmission: r.AllowResubmission,
		Criteria: criteria}
}

func required(c *gin.Context, fields map[string]bool) bool {
	details := map[string]string{}
	for k, ok := range fields {
		if !ok {
			details[k] = "is required"
		}
	}
	if len(details) == 0 {
		return true
	}
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", details)
	return false
}

func (h *Handler) list(c *gin.Context) {
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.ListProjects(c.Request.Context(), viewer(c), courseID, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) create(c *gin.Context) {
	courseID, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in projectRequest
	if !httpx.BindJSON(c, &in) || !required(c, map[string]bool{"title": in.Title != nil, "dueAt": in.DueAt != nil}) {
		return
	}
	p, err := h.svc.CreateProject(c.Request.Context(), viewer(c), courseID, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, p)
}

func (h *Handler) get(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.Project(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, p)
}

func (h *Handler) update(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in projectRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	p, err := h.svc.UpdateProject(c.Request.Context(), viewer(c), id, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, p)
}

func (h *Handler) teams(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	list, err := h.svc.Teams(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, len(list))
	httpx.Data(c, http.StatusOK, list)
}

type teamRequest struct {
	Name string `json:"name" binding:"required,min=1,max=80"`
}

func (h *Handler) createTeam(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in teamRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	t, err := h.svc.CreateTeam(c.Request.Context(), viewer(c), id, in.Name)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, t)
}

func (h *Handler) team(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	t, err := h.svc.TeamForProject(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

type memberRequest struct {
	StudentID string `json:"studentId" binding:"omitempty,uuid"`
}

func (h *Handler) addMember(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in memberRequest
	if c.Request.ContentLength != 0 && !httpx.BindJSON(c, &in) {
		return
	}
	t, err := h.svc.AddMember(c.Request.Context(), viewer(c), id, in.StudentID)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

func (h *Handler) removeMember(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	studentID, ok := httpx.PathUUID(c, "studentId")
	if !ok {
		return
	}
	if writeError(c, h.svc.RemoveMember(c.Request.Context(), viewer(c), id, studentID)) {
		return
	}
	httpx.NoContent(c)
}

type repoRequest struct {
	URL string `json:"url" binding:"required,max=300"`
}

func (h *Handler) linkRepo(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in repoRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	t, err := h.svc.LinkRepository(c.Request.Context(), viewer(c), id, in.URL)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

func (h *Handler) syncRepo(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	t, err := h.svc.SyncRepository(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

type presentationRequest struct {
	ScheduledAt *time.Time `json:"scheduledAt" binding:"required"`
	Location    string     `json:"location" binding:"max=200"`
}

func (h *Handler) schedule(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in presentationRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	t, err := h.svc.SchedulePresentation(c.Request.Context(), viewer(c), id, *in.ScheduledAt, in.Location)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

type presentationUpdate struct {
	Status   *string `json:"status" binding:"omitnil,oneof=scheduled done cancelled"`
	Feedback *string `json:"feedback" binding:"omitnil,max=10000"`
}

func (h *Handler) updatePresentation(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in presentationUpdate
	if !httpx.BindJSON(c, &in) {
		return
	}
	t, err := h.svc.UpdatePresentation(c.Request.Context(), viewer(c), id, in.Status, in.Feedback)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, t)
}

type submitRequest struct {
	RepoURL *string `json:"repoUrl" binding:"omitnil,url,max=2048"`
	FileID  *string `json:"fileId" binding:"omitnil,uuid"`
	Notes   *string `json:"notes" binding:"omitnil,max=10000"`
}

func (h *Handler) submit(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in submitRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	sub, err := h.svc.Submit(c.Request.Context(), viewer(c), id, SubmissionInput{RepoURL: in.RepoURL, FileID: in.FileID, Notes: in.Notes})
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, sub)
}

func (h *Handler) submissions(c *gin.Context) {
	f := SubmissionFilter{AllVersions: c.Query("allVersions") == "true"}
	var ok bool
	if f.ProjectID, ok = httpx.QueryUUID(c, "projectId"); !ok {
		return
	}
	if f.TeamID, ok = httpx.QueryUUID(c, "teamId"); !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.ListSubmissions(c.Request.Context(), viewer(c), f, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

type gradeRequest struct {
	Grade             *float64       `json:"grade" binding:"required"`
	CriteriaScores    map[string]int `json:"criteriaScores"`
	Feedback          *string        `json:"feedback" binding:"omitnil,max=10000"`
	ReturnForRevision bool           `json:"returnForRevision"`
	Reason            string         `json:"reason" binding:"max=500"`
}

func (h *Handler) grade(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in gradeRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	sub, err := h.svc.Grade(c.Request.Context(), viewer(c), id, GradeInput{Grade: *in.Grade, CriteriaScores: in.CriteriaScores,
		Feedback: in.Feedback, ReturnForRevision: in.ReturnForRevision, Reason: in.Reason})
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, sub)
}

func invalid(c *gin.Context, field string, err error) {
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{field: err.Error()})
}

func conflict(c *gin.Context, code string, err error) {
	httpx.Error(c, http.StatusConflict, code, err.Error(), nil)
}

func writeError(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound), errors.Is(err, learning.ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrForbidden), errors.Is(err, learning.ErrForbidden):
		identity.Forbidden(c)
	case errors.Is(err, ErrNotEnrolled):
		httpx.Error(c, http.StatusForbidden, "not_enrolled", err.Error(), nil)
	case errors.Is(err, ErrProjectNotOpen):
		conflict(c, "project_not_open", err)
	case errors.Is(err, ErrAlreadyInTeam):
		conflict(c, "already_in_team", err)
	case errors.Is(err, ErrTeamFull):
		conflict(c, "team_full", err)
	case errors.Is(err, ErrTeamNameTaken):
		conflict(c, "team_name_taken", err)
	case errors.Is(err, ErrTeamRequired):
		conflict(c, "team_required", err)
	case errors.Is(err, ErrTeamLocked):
		conflict(c, "team_locked", err)
	case errors.Is(err, ErrAlreadyGraded):
		conflict(c, "already_graded", err)
	case errors.Is(err, ErrResubmissionNotAllowed):
		conflict(c, "resubmission_not_allowed", err)
	case errors.Is(err, ErrNotLatest):
		conflict(c, "not_latest_submission", err)
	case errors.Is(err, ErrPresentationNotNeeded):
		conflict(c, "presentation_not_required", err)
	case errors.Is(err, ErrInvalidRepoURL):
		invalid(c, "url", err)
	case errors.Is(err, ErrGradeRange):
		invalid(c, "grade", err)
	case errors.Is(err, ErrCriteria):
		invalid(c, "criteriaScores", err)
	case errors.Is(err, ErrReasonRequired):
		invalid(c, "reason", err)
	case errors.Is(err, ErrInvalidFile):
		invalid(c, "fileId", err)
	case errors.Is(err, ErrSourceRequired):
		invalid(c, "repoUrl", err)
	default:
		httpx.Internal(c, err)
	}
	return true
}
