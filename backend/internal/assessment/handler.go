package assessment

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
	a.GET("/quizzes", h.listQuizzes)
	a.POST("/courses/:id/quizzes", h.createQuiz)
	a.GET("/quizzes/:id", h.getQuiz)
	a.PATCH("/quizzes/:id", h.updateQuiz)
	a.GET("/quizzes/:id/questions", h.getQuestions)
	a.PUT("/quizzes/:id/questions", h.putQuestions)
	a.POST("/quizzes/:id/attempts", h.startAttempt)
	a.GET("/quizzes/:id/attempts", h.listAttempts)
	a.GET("/attempts/:id", h.getAttempt)
	a.POST("/attempts/:id/submit", h.submitAttempt)

	a.GET("/assignments", h.listAssignments)
	a.POST("/courses/:id/assignments", h.createAssignment)
	a.GET("/assignments/:id", h.getAssignment)
	a.PATCH("/assignments/:id", h.updateAssignment)
	a.POST("/assignments/:id/submissions", h.submit)
	a.GET("/submissions", h.listSubmissions)
	a.GET("/submissions/:id", h.getSubmission)
	a.POST("/submissions/:id/grade", h.grade)
}

func viewer(c *gin.Context) learning.Viewer {
	return learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP())
}

func missing(c *gin.Context, fields map[string]bool) bool {
	details := map[string]string{}
	for k, ok := range fields {
		if !ok {
			details[k] = "is required"
		}
	}
	if len(details) == 0 {
		return false
	}
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", details)
	return true
}

type quizRequest struct {
	Title         *string    `json:"title" binding:"omitnil,min=1,max=160"`
	Description   *string    `json:"description" binding:"omitnil,max=5000"`
	TimeLimit     *int       `json:"timeLimit" binding:"omitnil,min=1,max=600"`
	RemoveLimit   bool       `json:"removeTimeLimit"`
	OpensAt       *time.Time `json:"opensAt"`
	DueAt         *time.Time `json:"dueAt"`
	MaxAttempts   *int       `json:"maxAttempts" binding:"omitnil,min=1,max=20"`
	PassPercent   *int       `json:"passPercent" binding:"omitnil,min=0,max=100"`
	RevealAnswers *string    `json:"revealAnswers" binding:"omitnil,oneof=after_submission after_close never"`
	Status        *string    `json:"status" binding:"omitnil,oneof=draft published"`
}

func (r quizRequest) input() QuizInput {
	return QuizInput{Title: r.Title, Description: r.Description, TimeLimit: r.TimeLimit, ClearLimit: r.RemoveLimit,
		OpensAt: r.OpensAt, DueAt: r.DueAt, MaxAttempts: r.MaxAttempts, PassPercent: r.PassPercent,
		RevealAnswers: r.RevealAnswers, Status: r.Status}
}

type optionRequest struct {
	Text    string `json:"text" binding:"required,min=1,max=500"`
	Correct bool   `json:"correct"`
}

type questionRequest struct {
	Text    string          `json:"text" binding:"required,min=1,max=2000"`
	Points  int             `json:"points" binding:"omitempty,min=1,max=100"`
	Options []optionRequest `json:"options" binding:"required,min=2,max=10,dive"`
}

type questionsRequest struct {
	Questions []questionRequest `json:"questions" binding:"required,min=1,max=200,dive"`
}

type answerRequest struct {
	QuestionID string `json:"questionId" binding:"required,uuid"`
	OptionID   string `json:"optionId" binding:"required,uuid"`
}

type submitAttemptRequest struct {
	Answers []answerRequest `json:"answers" binding:"max=200,dive"`
}

func (h *Handler) listQuizzes(c *gin.Context) {
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.ListQuizzes(c.Request.Context(), viewer(c), courseID, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) createQuiz(c *gin.Context) {
	courseID, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in quizRequest
	if !httpx.BindJSON(c, &in) || missing(c, map[string]bool{"title": in.Title != nil}) {
		return
	}
	in.Status = nil
	q, err := h.svc.CreateQuiz(c.Request.Context(), viewer(c), courseID, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, q)
}

func (h *Handler) getQuiz(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	q, err := h.svc.Quiz(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, q)
}

func (h *Handler) updateQuiz(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in quizRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	q, err := h.svc.UpdateQuiz(c.Request.Context(), viewer(c), id, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, q)
}

func (h *Handler) getQuestions(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	qs, err := h.svc.Questions(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, qs)
}

func (h *Handler) putQuestions(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in questionsRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	qs := make([]QuestionInput, 0, len(in.Questions))
	for _, q := range in.Questions {
		opts := make([]OptionInput, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, OptionInput{Text: o.Text, Correct: o.Correct})
		}
		qs = append(qs, QuestionInput{Text: q.Text, Points: q.Points, Options: opts})
	}
	out, err := h.svc.ReplaceQuestions(c.Request.Context(), viewer(c), id, qs)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, out)
}

func (h *Handler) startAttempt(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	a, created, err := h.svc.StartAttempt(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.Header("Cache-Control", "no-store")
	httpx.Data(c, status, a)
}

func (h *Handler) listAttempts(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.Attempts(c.Request.Context(), viewer(c), id, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) getAttempt(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	a, err := h.svc.Attempt(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, a)
}

func (h *Handler) submitAttempt(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in submitAttemptRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	answers := make(map[string]string, len(in.Answers))
	for _, a := range in.Answers {
		if _, dup := answers[a.QuestionID]; dup {
			writeError(c, ErrInvalidAnswer)
			return
		}
		answers[a.QuestionID] = a.OptionID
	}
	a, err := h.svc.SubmitAttempt(c.Request.Context(), viewer(c), id, answers)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, a)
}

type assignmentRequest struct {
	Title              *string    `json:"title" binding:"omitnil,min=1,max=160"`
	Description        *string    `json:"description" binding:"omitnil,max=10000"`
	DueAt              *time.Time `json:"dueAt"`
	MaxGrade           *int       `json:"maxGrade" binding:"omitnil,min=1,max=1000"`
	SubmissionTypes    []string   `json:"submissionTypes" binding:"omitempty,min=1,max=3,dive,oneof=file url text"`
	LatePolicy         *string    `json:"latePolicy" binding:"omitnil,oneof=reject accept penalty"`
	LateUntil          *time.Time `json:"lateUntil"`
	RemoveLateUntil    bool       `json:"removeLateUntil"`
	LatePenaltyPercent *int       `json:"latePenaltyPercent" binding:"omitnil,min=0,max=100"`
	Status             *string    `json:"status" binding:"omitnil,oneof=draft published"`
}

func (r assignmentRequest) input() AssignmentInput {
	return AssignmentInput{Title: r.Title, Description: r.Description, DueAt: r.DueAt, MaxGrade: r.MaxGrade,
		SubmissionTypes: r.SubmissionTypes, LatePolicy: r.LatePolicy, LateUntil: r.LateUntil, ClearLateUntil: r.RemoveLateUntil,
		LatePenaltyPercent: r.LatePenaltyPercent, Status: r.Status}
}

type submissionRequest struct {
	FileID *string `json:"fileId" binding:"omitnil,uuid"`
	URL    *string `json:"url" binding:"omitnil,url,max=2048"`
	Text   *string `json:"text" binding:"omitnil,min=1,max=50000"`
}

type gradeRequest struct {
	Grade             *float64 `json:"grade" binding:"required"`
	Feedback          *string  `json:"feedback" binding:"omitnil,max=10000"`
	ReturnForRevision bool     `json:"returnForRevision"`
	Reason            string   `json:"reason" binding:"max=500"`
}

func (h *Handler) listAssignments(c *gin.Context) {
	courseID, ok := httpx.QueryUUID(c, "courseId")
	if !ok {
		return
	}
	page := httpx.ParsePage(c)
	list, total, err := h.svc.ListAssignments(c.Request.Context(), viewer(c), courseID, page)
	if writeError(c, err) {
		return
	}
	httpx.SetTotal(c, total)
	httpx.Data(c, http.StatusOK, list)
}

func (h *Handler) createAssignment(c *gin.Context) {
	courseID, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in assignmentRequest
	if !httpx.BindJSON(c, &in) || missing(c, map[string]bool{"title": in.Title != nil, "dueAt": in.DueAt != nil}) {
		return
	}
	a, err := h.svc.CreateAssignment(c.Request.Context(), viewer(c), courseID, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, a)
}

func (h *Handler) getAssignment(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	a, err := h.svc.Assignment(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, a)
}

func (h *Handler) updateAssignment(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in assignmentRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	a, err := h.svc.UpdateAssignment(c.Request.Context(), viewer(c), id, in.input())
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, a)
}

func (h *Handler) submit(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	var in submissionRequest
	if !httpx.BindJSON(c, &in) {
		return
	}
	sub, err := h.svc.Submit(c.Request.Context(), viewer(c), id, SubmissionInput{FileID: in.FileID, URL: in.URL, Text: in.Text})
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusCreated, sub)
}

func (h *Handler) listSubmissions(c *gin.Context) {
	f := SubmissionFilter{Status: c.Query("status"), AllVersions: c.Query("allVersions") == "true"}
	switch f.Status {
	case "", SubmissionSubmitted, SubmissionGraded, SubmissionReturned:
	default:
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"status": "must be one of: submitted graded returned"})
		return
	}
	var ok bool
	if f.AssignmentID, ok = httpx.QueryUUID(c, "assignmentId"); !ok {
		return
	}
	if f.CourseID, ok = httpx.QueryUUID(c, "courseId"); !ok {
		return
	}
	if f.StudentID, ok = httpx.QueryUUID(c, "studentId"); !ok {
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

func (h *Handler) getSubmission(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	sub, err := h.svc.Submission(c.Request.Context(), viewer(c), id)
	if writeError(c, err) {
		return
	}
	httpx.Data(c, http.StatusOK, sub)
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
	sub, err := h.svc.Grade(c.Request.Context(), viewer(c), id, GradeInput{
		Grade: *in.Grade, Feedback: in.Feedback, ReturnForRevision: in.ReturnForRevision, Reason: in.Reason,
	})
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
	case errors.Is(err, ErrQuizNotOpen):
		conflict(c, "quiz_not_open", err)
	case errors.Is(err, ErrAttemptLimit):
		conflict(c, "attempt_limit_reached", err)
	case errors.Is(err, ErrAttemptClosed):
		conflict(c, "attempt_closed", err)
	case errors.Is(err, ErrAttemptExpired):
		conflict(c, "attempt_expired", err)
	case errors.Is(err, ErrQuizHasAttempts):
		conflict(c, "quiz_has_attempts", err)
	case errors.Is(err, ErrPastDeadline):
		conflict(c, "deadline_passed", err)
	case errors.Is(err, ErrAssignmentNotOpen):
		conflict(c, "assignment_not_open", err)
	case errors.Is(err, ErrAlreadyGraded):
		conflict(c, "already_graded", err)
	case errors.Is(err, ErrNotLatest):
		conflict(c, "not_latest_submission", err)
	case errors.Is(err, ErrQuizInvalid):
		invalid(c, "questions", err)
	case errors.Is(err, ErrInvalidAnswer):
		invalid(c, "answers", err)
	case errors.Is(err, ErrSubmissionType):
		invalid(c, "submission", err)
	case errors.Is(err, ErrInvalidFile):
		invalid(c, "fileId", err)
	case errors.Is(err, ErrGradeRange):
		invalid(c, "grade", err)
	case errors.Is(err, ErrReasonRequired):
		invalid(c, "reason", err)
	case errors.Is(err, ErrInvalidLatePolicy):
		invalid(c, "latePolicy", err)
	case errors.Is(err, ErrInvalidSchedule):
		invalid(c, "dueAt", err)
	default:
		httpx.Internal(c, err)
	}
	return true
}
