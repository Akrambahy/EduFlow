package assessment

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/communication"
	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type Assignment struct {
	ID                 string     `json:"id"`
	CourseID           string     `json:"courseId"`
	CourseTitle        string     `json:"courseTitle"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Status             string     `json:"status"`
	Published          bool       `json:"published"`
	DueAt              time.Time  `json:"dueAt"`
	DueDate            string     `json:"dueDate"`
	MaxGrade           int        `json:"maxGrade"`
	SubmissionTypes    []string   `json:"submissionTypes"`
	LatePolicy         string     `json:"latePolicy"`
	LateUntil          *time.Time `json:"lateUntil"`
	LatePenaltyPercent int        `json:"latePenaltyPercent"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type SubmissionFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type Submission struct {
	ID           string          `json:"id"`
	AssignmentID string          `json:"assignmentId"`
	CourseID     string          `json:"courseId"`
	StudentID    string          `json:"studentId"`
	StudentName  string          `json:"studentName"`
	Version      int             `json:"version"`
	Status       string          `json:"status"`
	SubmittedAt  time.Time       `json:"submittedAt"`
	IsLate       bool            `json:"isLate"`
	File         *SubmissionFile `json:"file"`
	FileName     *string         `json:"fileName"`
	URL          *string         `json:"url"`
	Text         *string         `json:"text"`
	Grade        *float64        `json:"grade"`
	FinalGrade   *float64        `json:"finalGrade"`
	MaxGrade     int             `json:"maxGrade"`
	Feedback     *string         `json:"feedback"`
	GradedAt     *time.Time      `json:"gradedAt"`
}

type AssignmentInput struct {
	Title              *string
	Description        *string
	DueAt              *time.Time
	MaxGrade           *int
	SubmissionTypes    []string
	LatePolicy         *string
	LateUntil          *time.Time
	ClearLateUntil     bool
	LatePenaltyPercent *int
	Status             *string
}

type SubmissionInput struct {
	FileID *string
	URL    *string
	Text   *string
}

const assignmentSelect = `
	SELECT a.id, a.course_id, c.title, a.title, a.description, a.status = 'published', a.due_at, a.max_grade,
		a.submission_types, a.late_policy, a.late_until, a.late_penalty_percent, a.created_at, a.updated_at
	FROM assignments a JOIN courses c ON c.id = a.course_id`

const assignmentVisible = learning.CourseVisibility + `
	AND (a.status = 'published' OR $1 OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2))`

func (s *Service) scanAssignment(row pgx.Row) (*Assignment, error) {
	var a Assignment
	err := row.Scan(&a.ID, &a.CourseID, &a.CourseTitle, &a.Title, &a.Description, &a.Published, &a.DueAt, &a.MaxGrade,
		&a.SubmissionTypes, &a.LatePolicy, &a.LateUntil, &a.LatePenaltyPercent, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.DueDate = *s.dateLabel(&a.DueAt)
	switch {
	case !a.Published:
		a.Status = "draft"
	case s.acceptsSubmissions(&a, s.now()):
		a.Status = "open"
	default:
		a.Status = "closed"
	}
	return &a, nil
}

func (s *Service) acceptsSubmissions(a *Assignment, now time.Time) bool {
	_, err := submissionTiming(a.DueAt, a.LatePolicy, a.LateUntil, now)
	return err == nil
}

func (s *Service) ListAssignments(ctx context.Context, v learning.Viewer, courseID string, page httpx.Page) ([]Assignment, int, error) {
	where := ` WHERE ` + assignmentVisible + ` AND ($4::uuid IS NULL OR a.course_id = $4::uuid)`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(courseID)}
	rows, err := s.db.Query(ctx, assignmentSelect+where+` ORDER BY a.due_at, a.id LIMIT $5 OFFSET $6`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Assignment, error) {
		a, err := s.scanAssignment(row)
		if err != nil {
			return Assignment{}, err
		}
		return *a, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM assignments a JOIN courses c ON c.id = a.course_id`+where, args...).Scan(&total)
	return list, total, err
}

func (s *Service) Assignment(ctx context.Context, v learning.Viewer, id string) (*Assignment, error) {
	return s.scanAssignment(s.db.QueryRow(ctx, assignmentSelect+` WHERE a.id = $4 AND `+assignmentVisible, v.IsAdmin, v.UserID, v.Roles, id))
}

func (s *Service) assignmentCourse(ctx context.Context, id string) (string, error) {
	var courseID string
	err := s.db.QueryRow(ctx, `SELECT course_id FROM assignments WHERE id = $1`, id).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return courseID, err
}

func validLate(policy string, lateUntil *time.Time, dueAt time.Time) bool {
	return policy != LateReject || lateUntil == nil
}

func (s *Service) CreateAssignment(ctx context.Context, v learning.Viewer, courseID string, in AssignmentInput) (*Assignment, error) {
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	if in.LatePolicy != nil && !validLate(*in.LatePolicy, in.LateUntil, *in.DueAt) {
		return nil, ErrInvalidLatePolicy
	}
	var id string
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO assignments (course_id, title, description, status, due_at, max_grade, submission_types,
				late_policy, late_until, late_penalty_percent, created_by)
			VALUES ($1, $2, COALESCE($3, ''), COALESCE($4, 'draft'), $5, COALESCE($6, 100), COALESCE($7, '{file}'::text[]),
				COALESCE($8, 'reject'), $9, COALESCE($10, 0), $11)
			RETURNING id`,
			courseID, strings.TrimSpace(*in.Title), in.Description, in.Status, in.DueAt, in.MaxGrade, emptyToNil(in.SubmissionTypes),
			in.LatePolicy, in.LateUntil, in.LatePenaltyPercent, v.UserID).Scan(&id)
		if database.IsCheckViolation(err) {
			return ErrInvalidLatePolicy
		}
		if err != nil {
			return err
		}
		if err := s.announceAssignment(ctx, tx, id); err != nil {
			return err
		}
		return record(ctx, tx, v, "assignment.created", "assignment", id, map[string]any{"courseId": courseID})
	})
	if err != nil {
		return nil, err
	}
	return s.Assignment(ctx, v, id)
}

func (s *Service) UpdateAssignment(ctx context.Context, v learning.Viewer, id string, in AssignmentInput) (*Assignment, error) {
	courseID, err := s.assignmentCourse(ctx, id)
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
		_, err := tx.Exec(ctx, `
			UPDATE assignments SET
				title = COALESCE($2, title),
				description = COALESCE($3, description),
				due_at = COALESCE($4, due_at),
				max_grade = COALESCE($5, max_grade),
				submission_types = COALESCE($6, submission_types),
				late_policy = COALESCE($7, late_policy),
				late_until = CASE WHEN $8 THEN NULL ELSE COALESCE($9, late_until) END,
				late_penalty_percent = COALESCE($10, late_penalty_percent),
				status = COALESCE($11, status)
			WHERE id = $1`, id, in.Title, in.Description, in.DueAt, in.MaxGrade, emptyToNil(in.SubmissionTypes),
			in.LatePolicy, in.ClearLateUntil, in.LateUntil, in.LatePenaltyPercent, in.Status)
		if database.IsCheckViolation(err) {
			return ErrInvalidLatePolicy
		}
		if err != nil {
			return err
		}
		var policy string
		var until *time.Time
		if err := tx.QueryRow(ctx, `SELECT late_policy, late_until FROM assignments WHERE id = $1`, id).Scan(&policy, &until); err != nil {
			return err
		}
		if policy == LateReject && until != nil {
			return ErrInvalidLatePolicy
		}
		if err := s.announceAssignment(ctx, tx, id); err != nil {
			return err
		}
		return record(ctx, tx, v, "assignment.updated", "assignment", id, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.Assignment(ctx, v, id)
}

const submissionSelect = `
	SELECT s.id, s.assignment_id, a.course_id, s.student_id, u.name, s.version, s.status, s.submitted_at, s.is_late,
		f.id, f.original_name, f.size_bytes, s.url, s.text_body, s.grade::float8, s.final_grade::float8, a.max_grade,
		s.feedback, s.graded_at
	FROM assignment_submissions s
	JOIN assignments a ON a.id = s.assignment_id
	JOIN courses c ON c.id = a.course_id
	JOIN users u ON u.id = s.student_id
	LEFT JOIN files f ON f.id = s.file_id`

func scanSubmission(row pgx.Row) (*Submission, error) {
	var sub Submission
	var fileID, fileName *string
	var fileSize *int64
	err := row.Scan(&sub.ID, &sub.AssignmentID, &sub.CourseID, &sub.StudentID, &sub.StudentName, &sub.Version, &sub.Status,
		&sub.SubmittedAt, &sub.IsLate, &fileID, &fileName, &fileSize, &sub.URL, &sub.Text, &sub.Grade, &sub.FinalGrade,
		&sub.MaxGrade, &sub.Feedback, &sub.GradedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if fileID != nil {
		sub.File = &SubmissionFile{ID: *fileID, Name: *fileName, Size: *fileSize}
		sub.FileName = fileName
	}
	return &sub, nil
}

func allowedType(types []string, t string) bool {
	for _, x := range types {
		if x == t {
			return true
		}
	}
	return false
}

func (s *Service) Submit(ctx context.Context, v learning.Viewer, assignmentID string, in SubmissionInput) (*Submission, error) {
	if !v.Has(identity.RoleStudent) {
		return nil, ErrForbidden
	}
	a, err := s.Assignment(ctx, v, assignmentID)
	if err != nil {
		return nil, err
	}
	if !a.Published {
		return nil, ErrAssignmentNotOpen
	}
	if (in.FileID != nil && !allowedType(a.SubmissionTypes, "file")) ||
		(in.URL != nil && !allowedType(a.SubmissionTypes, "url")) ||
		(in.Text != nil && !allowedType(a.SubmissionTypes, "text")) {
		return nil, ErrSubmissionType
	}
	if in.FileID == nil && in.URL == nil && in.Text == nil {
		return nil, ErrSubmissionType
	}
	now := s.now()
	late, err := submissionTiming(a.DueAt, a.LatePolicy, a.LateUntil, now)
	if err != nil {
		return nil, err
	}
	var id string
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		enrolled, err := s.isEnrolledStudent(ctx, tx, v.UserID, a.CourseID)
		if err != nil {
			return err
		}
		if !enrolled {
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
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 1))`, assignmentID, v.UserID); err != nil {
			return err
		}
		var currentID, currentStatus string
		var version int
		err = tx.QueryRow(ctx, `
			SELECT id, status, version FROM assignment_submissions
			WHERE assignment_id = $1 AND student_id = $2 AND status <> 'superseded'`, assignmentID, v.UserID).Scan(&currentID, &currentStatus, &version)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		case currentStatus == SubmissionGraded:
			return ErrAlreadyGraded
		default:
			if _, err := tx.Exec(ctx, `UPDATE assignment_submissions SET status = 'superseded' WHERE id = $1`, currentID); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, `
			INSERT INTO assignment_submissions (assignment_id, student_id, version, submitted_at, is_late, file_id, url, text_body)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			assignmentID, v.UserID, version+1, now, late, in.FileID, in.URL, in.Text).Scan(&id)
	})
	if err != nil {
		return nil, err
	}
	return scanSubmission(s.db.QueryRow(ctx, submissionSelect+` WHERE s.id = $1`, id))
}

type SubmissionFilter struct {
	AssignmentID string
	CourseID     string
	StudentID    string
	Status       string
	AllVersions  bool
}

func (s *Service) ListSubmissions(ctx context.Context, v learning.Viewer, f SubmissionFilter, page httpx.Page) ([]Submission, int, error) {
	where := ` WHERE ` + learning.CourseVisibility + `
		AND (s.student_id = $2 OR $1 OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2)
			OR ('mentor' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_mentors tm WHERE tm.track_id = c.track_id AND tm.user_id = $2))
			OR ('tracker' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_trackers tt WHERE tt.track_id = c.track_id AND tt.user_id = $2)))
		AND ($4::uuid IS NULL OR s.assignment_id = $4::uuid)
		AND ($5::uuid IS NULL OR a.course_id = $5::uuid)
		AND ($6::uuid IS NULL OR s.student_id = $6::uuid)
		AND ($7 = '' OR s.status = $7)
		AND ($8 OR s.status <> 'superseded')`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(f.AssignmentID), nullable(f.CourseID), nullable(f.StudentID), f.Status, f.AllVersions}
	rows, err := s.db.Query(ctx, submissionSelect+where+` ORDER BY s.submitted_at DESC, s.id LIMIT $9 OFFSET $10`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Submission, error) {
		sub, err := scanSubmission(row)
		if err != nil {
			return Submission{}, err
		}
		return *sub, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM assignment_submissions s JOIN assignments a ON a.id = s.assignment_id
		JOIN courses c ON c.id = a.course_id`+where, args...).Scan(&total)
	return list, total, err
}

func (s *Service) Submission(ctx context.Context, v learning.Viewer, id string) (*Submission, error) {
	sub, err := scanSubmission(s.db.QueryRow(ctx, submissionSelect+` WHERE s.id = $1`, id))
	if err != nil {
		return nil, err
	}
	if sub.StudentID == v.UserID {
		return sub, nil
	}
	staff, err := s.isStaff(ctx, v, sub.CourseID)
	if err != nil {
		return nil, err
	}
	if !staff {
		return nil, ErrNotFound
	}
	return sub, nil
}

type GradeInput struct {
	Grade             float64
	Feedback          *string
	ReturnForRevision bool
	Reason            string
}

func (s *Service) Grade(ctx context.Context, v learning.Viewer, id string, in GradeInput) (*Submission, error) {
	sub, err := scanSubmission(s.db.QueryRow(ctx, submissionSelect+` WHERE s.id = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, sub.CourseID); err != nil {
		if errors.Is(err, learning.ErrForbidden) && sub.StudentID != v.UserID {
			if staff, serr := s.isStaff(ctx, v, sub.CourseID); serr == nil && !staff {
				return nil, ErrNotFound
			}
		}
		return nil, err
	}
	if in.Grade < 0 || in.Grade > float64(sub.MaxGrade) {
		return nil, ErrGradeRange
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		var status string
		var previous *float64
		var late bool
		var policy string
		var penalty int
		err := tx.QueryRow(ctx, `
			SELECT s.status, s.grade::float8, s.is_late, a.late_policy, a.late_penalty_percent
			FROM assignment_submissions s JOIN assignments a ON a.id = s.assignment_id
			WHERE s.id = $1 FOR UPDATE OF s`, id).Scan(&status, &previous, &late, &policy, &penalty)
		if err != nil {
			return err
		}
		if status == SubmissionSuperseded {
			return ErrNotLatest
		}
		if previous != nil && *previous != in.Grade && strings.TrimSpace(in.Reason) == "" {
			return ErrReasonRequired
		}
		newStatus := SubmissionGraded
		if in.ReturnForRevision {
			newStatus = SubmissionReturned
		}
		final := finalGrade(in.Grade, late, policy, penalty)
		if _, err := tx.Exec(ctx, `
			UPDATE assignment_submissions SET grade = $2, final_grade = $3, feedback = COALESCE($4, feedback),
				status = $5, graded_by = $6, graded_at = now()
			WHERE id = $1`, id, in.Grade, final, in.Feedback, newStatus, v.UserID); err != nil {
			return err
		}
		action := "submission.graded"
		payload := map[string]any{"assignmentId": sub.AssignmentID, "studentId": sub.StudentID, "grade": in.Grade, "finalGrade": final, "status": newStatus}
		if previous != nil {
			action = "grade.changed"
			payload["before"] = *previous
			payload["reason"] = in.Reason
		}
		title := "Your submission was graded"
		if in.ReturnForRevision {
			title = "Your submission was returned for revision"
		}
		if err := communication.NotifyUsers(ctx, tx, []string{sub.StudentID}, communication.Message{
			Type: communication.TypeGrade, Title: title, Message: sub.StudentName, Link: "/assignments",
			Key: "grade:submission:" + id + ":" + strconv.FormatInt(time.Now().UnixNano(), 10),
		}); err != nil {
			return err
		}
		return record(ctx, tx, v, action, "submission", id, payload)
	})
	if err != nil {
		return nil, err
	}
	return s.Submission(ctx, v, id)
}

func (s *Service) SubmissionFileVisible(ctx context.Context, v learning.Viewer, fileID string) (bool, error) {
	rows, err := s.db.Query(ctx, `
		SELECT s.student_id, a.course_id FROM assignment_submissions s JOIN assignments a ON a.id = s.assignment_id
		WHERE s.file_id = $1`, fileID)
	if err != nil {
		return false, err
	}
	type ref struct{ student, course string }
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ref, error) {
		var r ref
		err := row.Scan(&r.student, &r.course)
		return r, err
	})
	if err != nil {
		return false, err
	}
	for _, r := range refs {
		if r.student == v.UserID {
			return true, nil
		}
		ok, err := s.isStaff(ctx, v, r.course)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func emptyToNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func (s *Service) announceAssignment(ctx context.Context, tx pgx.Tx, id string) error {
	var courseID, title string
	var published bool
	if err := tx.QueryRow(ctx, `SELECT course_id, title, status = 'published' FROM assignments WHERE id = $1`, id).Scan(&courseID, &title, &published); err != nil {
		return err
	}
	if !published {
		return nil
	}
	return communication.NotifyCourseStudents(ctx, tx, courseID, communication.Message{
		Type: communication.TypeAssignment, Title: "New assignment: " + title, Link: "/assignments", Key: "assignment:published:" + id,
	})
}
