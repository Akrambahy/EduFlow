package projects

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound               = errors.New("not found")
	ErrForbidden              = errors.New("forbidden")
	ErrNotEnrolled            = errors.New("you are not enrolled in this course")
	ErrProjectNotOpen         = errors.New("project is not published")
	ErrAlreadyInTeam          = errors.New("student is already in a team for this project")
	ErrTeamFull               = errors.New("team is full")
	ErrTeamNameTaken          = errors.New("team name is already used in this project")
	ErrTeamRequired           = errors.New("join or create a team before submitting")
	ErrTeamLocked             = errors.New("team membership cannot change after a submission")
	ErrAlreadyGraded          = errors.New("submission is already graded")
	ErrResubmissionNotAllowed = errors.New("this project does not allow resubmission")
	ErrNotLatest              = errors.New("only the latest submission can be graded")
	ErrGradeRange             = errors.New("grade must be between 0 and the maximum grade")
	ErrCriteria               = errors.New("criteria scores must match the project criteria and stay within their maximum")
	ErrReasonRequired         = errors.New("a reason is required when changing an existing grade")
	ErrInvalidFile            = errors.New("file must exist and be uploaded by you")
	ErrPresentationNotNeeded  = errors.New("this project does not require a presentation")
	ErrSourceRequired         = errors.New("provide a repoUrl or a fileId")
)

type Criterion struct {
	Title     string `json:"title"`
	MaxPoints int    `json:"maxPoints"`
}

type Project struct {
	ID                   string      `json:"id"`
	CourseID             string      `json:"courseId"`
	CourseTitle          string      `json:"courseTitle"`
	TrackID              *string     `json:"trackId"`
	Title                string      `json:"title"`
	Description          string      `json:"description"`
	Requirements         string      `json:"requirements"`
	Status               string      `json:"status"`
	Published            bool        `json:"published"`
	DueAt                time.Time   `json:"dueAt"`
	DueDate              string      `json:"dueDate"`
	MaxGrade             int         `json:"maxGrade"`
	TeamSize             int         `json:"teamSize"`
	RequiresPresentation bool        `json:"requiresPresentation"`
	AllowResubmission    bool        `json:"allowResubmission"`
	Criteria             []Criterion `json:"criteria"`
	MyTeamID             *string     `json:"myTeamId,omitempty"`
	GithubURL            *string     `json:"githubUrl"`
	CreatedAt            time.Time   `json:"createdAt"`
}

type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Repository struct {
	URL          string        `json:"url"`
	Owner        string        `json:"owner"`
	Name         string        `json:"name"`
	LinkedAt     time.Time     `json:"linkedAt"`
	Metadata     *RepoMetadata `json:"metadata"`
	LastSyncedAt *time.Time    `json:"lastSyncedAt"`
	SyncError    *string       `json:"syncError"`
}

type Presentation struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"projectId"`
	TeamID           string    `json:"teamId"`
	ScheduledAt      time.Time `json:"scheduledAt"`
	PresentationDate string    `json:"presentationDate"`
	PresentationTime string    `json:"presentationTime"`
	Location         string    `json:"location"`
	Status           string    `json:"status"`
	Feedback         *string   `json:"feedback"`
}

type Team struct {
	ID           string        `json:"id"`
	ProjectID    string        `json:"projectId"`
	Name         string        `json:"name"`
	Members      []Member      `json:"members"`
	Repository   *Repository   `json:"repository"`
	Presentation *Presentation `json:"presentation"`
}

type SubmissionFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Submission struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"projectId"`
	TeamID           string          `json:"teamId"`
	TeamName         string          `json:"teamName"`
	StudentID        string          `json:"studentId"`
	StudentName      string          `json:"studentName"`
	Version          int             `json:"version"`
	Status           string          `json:"status"`
	SubmittedAt      time.Time       `json:"submittedAt"`
	IsLate           bool            `json:"isLate"`
	RepoURL          *string         `json:"repoUrl"`
	File             *SubmissionFile `json:"file"`
	Notes            *string         `json:"notes"`
	Grade            *float64        `json:"grade"`
	MaxGrade         int             `json:"maxGrade"`
	CriteriaScores   map[string]int  `json:"criteriaScores"`
	Feedback         *string         `json:"feedback"`
	GradedAt         *time.Time      `json:"gradedAt"`
	PresentationDate *string         `json:"presentationDate"`
	PresentationTime *string         `json:"presentationTime"`
	courseID         string
}

type Service struct {
	db       *pgxpool.Pool
	learning *learning.Service
	github   GitHubClient
	loc      *time.Location
	now      func() time.Time
}

func NewService(db *pgxpool.Pool, l *learning.Service, gh GitHubClient, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{db: db, learning: l, github: gh, loc: loc, now: time.Now}
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.db, fn)
}

func record(ctx context.Context, tx pgx.Tx, v learning.Viewer, action, targetType, id string, payload map[string]any) error {
	return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: action, TargetType: targetType, TargetID: id, Context: payload, IP: v.IP})
}

func (s *Service) canManage(ctx context.Context, v learning.Viewer, courseID string) (bool, error) {
	err := s.learning.CheckManage(ctx, v, courseID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, learning.ErrForbidden) || errors.Is(err, learning.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (s *Service) isStaff(ctx context.Context, v learning.Viewer, courseID string) (bool, error) {
	ok, err := s.canManage(ctx, v, courseID)
	if err != nil || ok {
		return ok, err
	}
	if !v.Has(identity.RoleMentor) && !v.Has(identity.RoleTracker) {
		return false, nil
	}
	_, err = s.learning.Course(ctx, v, courseID)
	if errors.Is(err, learning.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) enrolled(ctx context.Context, db database.DBTX, studentID, courseID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM enrollments e JOIN courses c ON c.id = e.course_id
		JOIN user_roles ur ON ur.user_id = e.student_id JOIN roles r ON r.id = ur.role_id AND r.name = 'student'
		WHERE e.student_id = $1 AND e.course_id = $2 AND e.status IN ('active', 'completed') AND c.status = 'published')`,
		studentID, courseID).Scan(&ok)
	return ok, err
}

func (s *Service) local(t time.Time) (string, string) {
	l := t.In(s.loc)
	return l.Format("2006-01-02"), l.Format("15:04")
}

const projectSelect = `
	SELECT p.id, p.course_id, c.title, c.track_id, p.title, p.description, p.requirements, p.status = 'published', p.due_at,
		p.max_grade, p.team_size, p.requires_presentation, p.allow_resubmission, p.criteria, p.created_at
	FROM projects p JOIN courses c ON c.id = p.course_id`

const projectVisible = learning.CourseVisibility + `
	AND (p.status = 'published' OR $1 OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2))`

func (s *Service) scanProject(row pgx.Row) (*Project, error) {
	var p Project
	var criteria []byte
	err := row.Scan(&p.ID, &p.CourseID, &p.CourseTitle, &p.TrackID, &p.Title, &p.Description, &p.Requirements, &p.Published,
		&p.DueAt, &p.MaxGrade, &p.TeamSize, &p.RequiresPresentation, &p.AllowResubmission, &criteria, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Criteria = []Criterion{}
	_ = json.Unmarshal(criteria, &p.Criteria)
	p.DueDate, _ = s.local(p.DueAt)
	switch {
	case !p.Published:
		p.Status = "draft"
	case s.now().After(p.DueAt):
		p.Status = "closed"
	default:
		p.Status = "in_progress"
	}
	return &p, nil
}

func (s *Service) withMine(ctx context.Context, v learning.Viewer, p *Project) error {
	if !v.Has(identity.RoleStudent) {
		return nil
	}
	var teamID string
	var repo *string
	err := s.db.QueryRow(ctx, `
		SELECT m.team_id, g.url FROM project_team_members m LEFT JOIN github_repositories g ON g.team_id = m.team_id
		WHERE m.project_id = $1 AND m.student_id = $2`, p.ID, v.UserID).Scan(&teamID, &repo)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	p.MyTeamID = &teamID
	p.GithubURL = repo
	return nil
}

func (s *Service) ListProjects(ctx context.Context, v learning.Viewer, courseID string, page httpx.Page) ([]Project, int, error) {
	where := ` WHERE ` + projectVisible + ` AND ($4::uuid IS NULL OR p.course_id = $4::uuid)`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(courseID)}
	rows, err := s.db.Query(ctx, projectSelect+where+` ORDER BY p.due_at, p.id LIMIT $5 OFFSET $6`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Project, error) {
		p, err := s.scanProject(row)
		if err != nil {
			return Project{}, err
		}
		return *p, nil
	})
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		if err := s.withMine(ctx, v, &list[i]); err != nil {
			return nil, 0, err
		}
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM projects p JOIN courses c ON c.id = p.course_id`+where, args...).Scan(&total)
	return list, total, err
}

func (s *Service) Project(ctx context.Context, v learning.Viewer, id string) (*Project, error) {
	p, err := s.scanProject(s.db.QueryRow(ctx, projectSelect+` WHERE p.id = $4 AND `+projectVisible, v.IsAdmin, v.UserID, v.Roles, id))
	if err != nil {
		return nil, err
	}
	return p, s.withMine(ctx, v, p)
}

type ProjectInput struct {
	Title                *string
	Description          *string
	Requirements         *string
	Status               *string
	DueAt                *time.Time
	MaxGrade             *int
	TeamSize             *int
	RequiresPresentation *bool
	AllowResubmission    *bool
	Criteria             []Criterion
}

func criteriaJSON(c []Criterion) []byte {
	if c == nil {
		return nil
	}
	b, _ := json.Marshal(c)
	return b
}

func (s *Service) CreateProject(ctx context.Context, v learning.Viewer, courseID string, in ProjectInput) (*Project, error) {
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	var id string
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO projects (course_id, title, description, requirements, status, due_at, max_grade, team_size,
				requires_presentation, allow_resubmission, criteria, created_by)
			VALUES ($1, $2, COALESCE($3, ''), COALESCE($4, ''), COALESCE($5, 'draft'), $6, COALESCE($7, 100), COALESCE($8, 1),
				COALESCE($9, false), COALESCE($10, true), COALESCE($11::jsonb, '[]'::jsonb), $12)
			RETURNING id`,
			courseID, strings.TrimSpace(*in.Title), in.Description, in.Requirements, in.Status, in.DueAt, in.MaxGrade, in.TeamSize,
			in.RequiresPresentation, in.AllowResubmission, criteriaJSON(in.Criteria), v.UserID).Scan(&id)
		if err != nil {
			return err
		}
		return record(ctx, tx, v, "project.created", "project", id, map[string]any{"courseId": courseID})
	})
	if err != nil {
		return nil, err
	}
	return s.Project(ctx, v, id)
}

func (s *Service) projectCourse(ctx context.Context, db database.DBTX, id string) (string, error) {
	var courseID string
	err := db.QueryRow(ctx, `SELECT course_id FROM projects WHERE id = $1`, id).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return courseID, err
}

func (s *Service) UpdateProject(ctx context.Context, v learning.Viewer, id string, in ProjectInput) (*Project, error) {
	courseID, err := s.projectCourse(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		in.Title = &t
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE projects SET
				title = COALESCE($2, title), description = COALESCE($3, description), requirements = COALESCE($4, requirements),
				status = COALESCE($5, status), due_at = COALESCE($6, due_at), max_grade = COALESCE($7, max_grade),
				team_size = COALESCE($8, team_size), requires_presentation = COALESCE($9, requires_presentation),
				allow_resubmission = COALESCE($10, allow_resubmission), criteria = COALESCE($11::jsonb, criteria)
			WHERE id = $1`, id, in.Title, in.Description, in.Requirements, in.Status, in.DueAt, in.MaxGrade, in.TeamSize,
			in.RequiresPresentation, in.AllowResubmission, criteriaJSON(in.Criteria)); err != nil {
			return err
		}
		return record(ctx, tx, v, "project.updated", "project", id, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.Project(ctx, v, id)
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
