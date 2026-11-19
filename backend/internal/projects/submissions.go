package projects

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/performance"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const submissionSelect = `
	SELECT s.id, s.project_id, s.team_id, t.name, s.submitted_by, u.name, s.version, s.status, s.submitted_at, s.is_late,
		s.repo_url, f.id, f.original_name, s.notes, s.grade::float8, p.max_grade, s.criteria_scores, s.feedback, s.graded_at,
		pr.scheduled_at, p.course_id
	FROM project_submissions s
	JOIN projects p ON p.id = s.project_id
	JOIN courses c ON c.id = p.course_id
	JOIN project_teams t ON t.id = s.team_id
	JOIN users u ON u.id = s.submitted_by
	LEFT JOIN files f ON f.id = s.file_id
	LEFT JOIN presentations pr ON pr.team_id = s.team_id AND pr.status <> 'cancelled'`

func (s *Service) scanSubmission(row pgx.Row) (*Submission, error) {
	var sub Submission
	var fileID, fileName *string
	var criteria []byte
	var presentAt *time.Time
	err := row.Scan(&sub.ID, &sub.ProjectID, &sub.TeamID, &sub.TeamName, &sub.StudentID, &sub.StudentName, &sub.Version, &sub.Status,
		&sub.SubmittedAt, &sub.IsLate, &sub.RepoURL, &fileID, &fileName, &sub.Notes, &sub.Grade, &sub.MaxGrade, &criteria,
		&sub.Feedback, &sub.GradedAt, &presentAt, &sub.courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if fileID != nil {
		sub.File = &SubmissionFile{ID: *fileID, Name: *fileName}
	}
	if len(criteria) > 0 {
		_ = json.Unmarshal(criteria, &sub.CriteriaScores)
	}
	if presentAt != nil {
		d, t := s.local(*presentAt)
		sub.PresentationDate, sub.PresentationTime = &d, &t
	}
	return &sub, nil
}

type SubmissionInput struct {
	RepoURL *string
	FileID  *string
	Notes   *string
}

func (s *Service) Submit(ctx context.Context, v learning.Viewer, projectID string, in SubmissionInput) (*Submission, error) {
	if !v.Has(identity.RoleStudent) {
		return nil, ErrForbidden
	}
	if in.RepoURL == nil && in.FileID == nil {
		return nil, ErrSourceRequired
	}
	p, err := s.Project(ctx, v, projectID)
	if err != nil {
		return nil, err
	}
	if !p.Published {
		return nil, ErrProjectNotOpen
	}
	now := s.now()
	var id string
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		ok, err := s.enrolled(ctx, tx, v.UserID, p.CourseID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotEnrolled
		}
		if in.FileID != nil {
			var owned bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM files WHERE id = $1 AND owner_id = $2)`, *in.FileID, v.UserID).Scan(&owned); err != nil {
				return err
			}
			if !owned {
				return ErrInvalidFile
			}
		}
		var teamID string
		err = tx.QueryRow(ctx, `SELECT team_id FROM project_team_members WHERE project_id = $1 AND student_id = $2`, projectID, v.UserID).Scan(&teamID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if p.TeamSize > 1 {
				return ErrTeamRequired
			}
			var name string
			if err := tx.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, v.UserID).Scan(&name); err != nil {
				return err
			}
			if teamID, err = s.createTeam(ctx, tx, projectID, name+" ("+v.UserID[:8]+")", v.UserID); err != nil {
				return err
			}
		case err != nil:
			return err
		}
		if _, err := s.teamRef(ctx, tx, teamID, true); err != nil {
			return err
		}
		var currentID, status string
		var version int
		err = tx.QueryRow(ctx, `SELECT id, status, version FROM project_submissions WHERE team_id = $1 AND status <> 'superseded'`, teamID).
			Scan(&currentID, &status, &version)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		case status == "graded":
			return ErrAlreadyGraded
		case status == "submitted" && !p.AllowResubmission:
			return ErrResubmissionNotAllowed
		default:
			if _, err := tx.Exec(ctx, `UPDATE project_submissions SET status = 'superseded' WHERE id = $1`, currentID); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, `
			INSERT INTO project_submissions (project_id, team_id, version, submitted_by, submitted_at, is_late, repo_url, file_id, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
			projectID, teamID, version+1, v.UserID, now, now.After(p.DueAt), in.RepoURL, in.FileID, in.Notes).Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	return s.scanSubmission(s.db.QueryRow(ctx, submissionSelect+` WHERE s.id = $1`, id))
}

type SubmissionFilter struct {
	ProjectID   string
	TeamID      string
	AllVersions bool
}

func (s *Service) ListSubmissions(ctx context.Context, v learning.Viewer, f SubmissionFilter, page httpx.Page) ([]Submission, int, error) {
	where := ` WHERE ` + learning.CourseVisibility + `
		AND (EXISTS (SELECT 1 FROM project_team_members m WHERE m.team_id = s.team_id AND m.student_id = $2)
			OR $1 OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2)
			OR ('mentor' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_mentors tm WHERE tm.track_id = c.track_id AND tm.user_id = $2))
			OR ('tracker' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_trackers tt WHERE tt.track_id = c.track_id AND tt.user_id = $2)))
		AND ($4::uuid IS NULL OR s.project_id = $4::uuid)
		AND ($5::uuid IS NULL OR s.team_id = $5::uuid)
		AND ($6 OR s.status <> 'superseded')`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(f.ProjectID), nullable(f.TeamID), f.AllVersions}
	rows, err := s.db.Query(ctx, submissionSelect+where+` ORDER BY s.submitted_at DESC, s.id LIMIT $7 OFFSET $8`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Submission, error) {
		sub, err := s.scanSubmission(row)
		if err != nil {
			return Submission{}, err
		}
		return *sub, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM project_submissions s JOIN projects p ON p.id = s.project_id
		JOIN courses c ON c.id = p.course_id`+where, args...).Scan(&total)
	return list, total, err
}

type GradeInput struct {
	Grade             float64
	CriteriaScores    map[string]int
	Feedback          *string
	ReturnForRevision bool
	Reason            string
}

func (s *Service) Grade(ctx context.Context, v learning.Viewer, id string, in GradeInput) (*Submission, error) {
	sub, err := s.scanSubmission(s.db.QueryRow(ctx, submissionSelect+` WHERE s.id = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, sub.courseID); err != nil {
		if errors.Is(err, learning.ErrForbidden) {
			member, merr := s.isMember(ctx, s.db, sub.TeamID, v.UserID)
			staff, serr := s.isStaff(ctx, v, sub.courseID)
			if merr == nil && serr == nil && !member && !staff {
				return nil, ErrNotFound
			}
		}
		return nil, err
	}
	if in.Grade < 0 || in.Grade > float64(sub.MaxGrade) {
		return nil, ErrGradeRange
	}
	var criteriaRaw []byte
	if in.CriteriaScores != nil {
		var criteria []byte
		if err := s.db.QueryRow(ctx, `SELECT criteria FROM projects WHERE id = $1`, sub.ProjectID).Scan(&criteria); err != nil {
			return nil, err
		}
		var defs []Criterion
		_ = json.Unmarshal(criteria, &defs)
		max := map[string]int{}
		for _, d := range defs {
			max[d.Title] = d.MaxPoints
		}
		for k, val := range in.CriteriaScores {
			limit, ok := max[k]
			if !ok || val < 0 || val > limit {
				return nil, ErrCriteria
			}
		}
		criteriaRaw, _ = json.Marshal(in.CriteriaScores)
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		var status string
		var previous *float64
		if err := tx.QueryRow(ctx, `SELECT status, grade::float8 FROM project_submissions WHERE id = $1 FOR UPDATE`, id).Scan(&status, &previous); err != nil {
			return err
		}
		if status == "superseded" {
			return ErrNotLatest
		}
		if previous != nil && *previous != in.Grade && strings.TrimSpace(in.Reason) == "" {
			return ErrReasonRequired
		}
		newStatus := "graded"
		if in.ReturnForRevision {
			newStatus = "returned"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE project_submissions SET grade = $2, criteria_scores = COALESCE($3::jsonb, criteria_scores),
				feedback = COALESCE($4, feedback), status = $5, graded_by = $6, graded_at = now()
			WHERE id = $1`, id, in.Grade, criteriaRaw, in.Feedback, newStatus, v.UserID); err != nil {
			return err
		}
		action := "project.graded"
		payload := map[string]any{"projectId": sub.ProjectID, "teamId": sub.TeamID, "grade": in.Grade, "status": newStatus}
		if previous != nil {
			action = "grade.changed"
			payload["before"] = *previous
			payload["reason"] = in.Reason
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO notifications (user_id, type, title, message, link, dedupe_key)
			SELECT m.student_id, 'grade', 'Your project was graded', '', '/performance', $2
			FROM project_team_members m WHERE m.team_id = $1
			ON CONFLICT (user_id, dedupe_key) DO NOTHING`, sub.TeamID, "grade:project:"+id+":"+strconv.FormatInt(time.Now().UnixNano(), 10)); err != nil {
			return err
		}
		return record(ctx, tx, v, action, "project_submission", id, payload)
	})
	if err != nil {
		return nil, err
	}
	return s.scanSubmission(s.db.QueryRow(ctx, submissionSelect+` WHERE s.id = $1`, id))
}

func (s *Service) FileVisible(ctx context.Context, v learning.Viewer, fileID string) (bool, error) {
	rows, err := s.db.Query(ctx, `
		SELECT s.team_id, p.course_id FROM project_submissions s JOIN projects p ON p.id = s.project_id WHERE s.file_id = $1`, fileID)
	if err != nil {
		return false, err
	}
	type ref struct{ team, course string }
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ref, error) {
		var r ref
		err := row.Scan(&r.team, &r.course)
		return r, err
	})
	if err != nil {
		return false, err
	}
	for _, r := range refs {
		member, err := s.isMember(ctx, s.db, r.team, v.UserID)
		if err != nil || member {
			return member, err
		}
		staff, err := s.isStaff(ctx, v, r.course)
		if err != nil || staff {
			return staff, err
		}
	}
	return false, nil
}

func GradeItems(ctx context.Context, db *pgxpool.Pool, courseID, studentID string) (map[string][]performance.Item, error) {
	var student any
	if studentID != "" {
		student = studentID
	}
	rows, err := db.Query(ctx, `
		SELECT e.student_id, (s.grade / p.max_grade * 100)::float8, s.status, now() > p.due_at
		FROM enrollments e
		JOIN projects p ON p.course_id = e.course_id AND p.status = 'published'
		LEFT JOIN project_team_members m ON m.project_id = p.id AND m.student_id = e.student_id
		LEFT JOIN project_submissions s ON s.team_id = m.team_id AND s.status <> 'superseded'
		WHERE e.course_id = $1 AND e.status IN ('active', 'completed') AND ($2::uuid IS NULL OR e.student_id = $2)`, courseID, student)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]performance.Item{}
	for rows.Next() {
		var sid string
		var pct *float64
		var status *string
		var closed bool
		if err := rows.Scan(&sid, &pct, &status, &closed); err != nil {
			return nil, err
		}
		var it performance.Item
		switch {
		case pct != nil:
			it.Percent = pct
		case status != nil:
			it.Pending = true
		case closed:
			it.Missing = true
		default:
			it.Pending = true
		}
		out[sid] = append(out[sid], it)
	}
	return out, rows.Err()
}
