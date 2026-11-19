package learning

import (
	"context"
	"errors"
	"strconv"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

const enrollmentSelect = `
	SELECT e.id, e.student_id, su.name, e.course_id, c.title, e.status, e.enrolled_at, e.completed_at, e.withdrawn_at,
		(SELECT count(*) FROM lesson_progress lp JOIN lessons l ON l.id = lp.lesson_id JOIN modules m ON m.id = l.module_id
		 WHERE m.course_id = e.course_id AND lp.student_id = e.student_id AND lp.completed_at IS NOT NULL),
		(SELECT count(*) FROM lessons l JOIN modules m ON m.id = l.module_id WHERE m.course_id = e.course_id)
	FROM enrollments e
	JOIN users su ON su.id = e.student_id
	JOIN courses c ON c.id = e.course_id`

const enrollmentVisible = `($1
	OR e.student_id = $2
	OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2)
	OR ('mentor' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_mentors tm WHERE tm.track_id = c.track_id AND tm.user_id = $2))
	OR ('tracker' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_trackers tt WHERE tt.track_id = c.track_id AND tt.user_id = $2)))`

type EnrollmentFilter struct {
	CourseID  string
	StudentID string
	Status    string
}

func scanEnrollment(row pgx.Row) (*Enrollment, error) {
	var e Enrollment
	var dbStatus string
	err := row.Scan(&e.ID, &e.StudentID, &e.StudentName, &e.CourseID, &e.CourseTitle, &dbStatus,
		&e.EnrolledAt, &e.CompletedAt, &e.WithdrawnAt, &e.CompletedLessons, &e.TotalLessons)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.Progress = progressPercent(e.CompletedLessons, e.TotalLessons)
	if dbStatus == EnrollmentCompleted {
		e.Progress = 100
	}
	e.Status = enrollmentAPIStatus(dbStatus, e.CompletedLessons)
	return &e, nil
}

func (r *Repository) ListEnrollments(ctx context.Context, v Viewer, f EnrollmentFilter, page httpx.Page) ([]Enrollment, int, error) {
	statusClause := `e.status IN ('active', 'completed')`
	if f.Status == "all" {
		statusClause = `true`
	} else if f.Status != "" {
		statusClause = `e.status = $6`
	}
	where := ` WHERE ` + enrollmentVisible + `
		AND ($4::uuid IS NULL OR e.course_id = $4::uuid)
		AND ($5::uuid IS NULL OR e.student_id = $5::uuid)
		AND ` + statusClause
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(f.CourseID), nullable(f.StudentID)}
	if statusClause == `e.status = $6` {
		args = append(args, f.Status)
	}
	n := len(args)
	rows, err := r.db.Query(ctx, enrollmentSelect+where+` ORDER BY e.enrolled_at DESC, e.id LIMIT $`+strconv.Itoa(n+1)+` OFFSET $`+strconv.Itoa(n+2),
		append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Enrollment, error) {
		e, err := scanEnrollment(row)
		if err != nil {
			return Enrollment{}, err
		}
		return *e, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM enrollments e JOIN courses c ON c.id = e.course_id`+where, args...).Scan(&total)
	return list, total, err
}

func (r *Repository) Enrollment(ctx context.Context, db database.DBTX, id string) (*Enrollment, error) {
	return scanEnrollment(db.QueryRow(ctx, enrollmentSelect+` WHERE e.id = $1`, id))
}

func (r *Repository) InsertEnrollment(ctx context.Context, db database.DBTX, studentID, courseID, by string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO enrollments (student_id, course_id, enrolled_by) VALUES ($1, $2, $3) RETURNING id`,
		studentID, courseID, by).Scan(&id)
	if database.IsUniqueViolation(err, "enrollments_current_key") {
		return "", ErrAlreadyEnrolled
	}
	return id, err
}

func (r *Repository) WithdrawEnrollment(ctx context.Context, db database.DBTX, studentID, courseID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		UPDATE enrollments SET status = 'withdrawn', withdrawn_at = now()
		WHERE student_id = $1 AND course_id = $2 AND status IN ('active', 'completed')
		RETURNING id`, studentID, courseID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotEnrolled
	}
	return id, err
}

func (r *Repository) lockCurrentEnrollment(ctx context.Context, db database.DBTX, studentID, courseID string) (string, string, error) {
	var id, status string
	err := db.QueryRow(ctx, `
		SELECT id, status FROM enrollments
		WHERE student_id = $1 AND course_id = $2 AND status IN ('active', 'completed') FOR UPDATE`,
		studentID, courseID).Scan(&id, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotEnrolled
	}
	return id, status, err
}

func (r *Repository) IsEnrolled(ctx context.Context, studentID, courseID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM enrollments WHERE student_id = $1 AND course_id = $2 AND status IN ('active', 'completed'))`,
		studentID, courseID).Scan(&ok)
	return ok, err
}

func (r *Repository) markProgress(ctx context.Context, db database.DBTX, studentID, lessonID string, complete bool) error {
	_, err := db.Exec(ctx, `
		INSERT INTO lesson_progress (student_id, lesson_id, completed_at)
		VALUES ($1, $2, CASE WHEN $3 THEN now() END)
		ON CONFLICT (student_id, lesson_id) DO UPDATE
		SET completed_at = COALESCE(lesson_progress.completed_at, EXCLUDED.completed_at)`,
		studentID, lessonID, complete)
	return err
}

func (r *Repository) completeIfFinished(ctx context.Context, db database.DBTX, enrollmentID, studentID, courseID string) error {
	_, err := db.Exec(ctx, `
		UPDATE enrollments SET status = 'completed', completed_at = now()
		WHERE id = $1 AND status = 'active'
		AND (SELECT count(*) FROM lessons l JOIN modules m ON m.id = l.module_id WHERE m.course_id = $3) > 0
		AND NOT EXISTS (
			SELECT 1 FROM lessons l JOIN modules m ON m.id = l.module_id
			LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.student_id = $2
			WHERE m.course_id = $3 AND lp.completed_at IS NULL)`, enrollmentID, studentID, courseID)
	return err
}
