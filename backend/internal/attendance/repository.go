package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

type classSession struct {
	ID        string
	CourseID  string
	StartsAt  time.Time
	Cancelled bool
}

func (r *Repository) classSession(ctx context.Context, db database.DBTX, id string) (*classSession, error) {
	var s classSession
	err := db.QueryRow(ctx, `SELECT id, course_id, starts_at, cancelled_at IS NOT NULL FROM class_sessions WHERE id = $1`, id).
		Scan(&s.ID, &s.CourseID, &s.StartsAt, &s.Cancelled)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (r *Repository) sessionVisible(ctx context.Context, v learning.Viewer, sessionID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM class_sessions s JOIN courses c ON c.id = s.course_id
		WHERE s.id = $4 AND `+learning.CourseVisibility+`)`, v.IsAdmin, v.UserID, v.Roles, sessionID).Scan(&ok)
	return ok, err
}

const windowSelect = `
	SELECT w.id, w.session_id, s.course_id, w.status, w.grace_minutes, w.token_ttl_seconds, w.opened_at, w.closed_at,
		w.token_secret, s.starts_at
	FROM attendance_sessions w JOIN class_sessions s ON s.id = w.session_id`

func scanWindow(row pgx.Row) (*Window, error) {
	var w Window
	err := row.Scan(&w.ID, &w.SessionID, &w.CourseID, &w.Status, &w.GraceMinutes, &w.TokenTTLSeconds, &w.OpenedAt, &w.ClosedAt, &w.secret, &w.sessionStart)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &w, err
}

func (r *Repository) windowBySession(ctx context.Context, db database.DBTX, sessionID string, lock bool) (*Window, error) {
	q := windowSelect + ` WHERE w.session_id = $1`
	if lock {
		q += ` FOR UPDATE OF w`
	}
	return scanWindow(db.QueryRow(ctx, q, sessionID))
}

func (r *Repository) windowByID(ctx context.Context, db database.DBTX, id string) (*Window, error) {
	return scanWindow(db.QueryRow(ctx, windowSelect+` WHERE w.id = $1`, id))
}

func (r *Repository) openWindow(ctx context.Context, db database.DBTX, sessionID string, secret []byte, ttl, grace int, by string) (bool, error) {
	tag, err := db.Exec(ctx, `
		INSERT INTO attendance_sessions (session_id, token_secret, token_ttl_seconds, grace_minutes, opened_by)
		VALUES ($1, $2, $3, $4, $5) ON CONFLICT (session_id) DO NOTHING`, sessionID, secret, ttl, grace, by)
	return err == nil && tag.RowsAffected() == 1, err
}

func (r *Repository) closeWindow(ctx context.Context, db database.DBTX, windowID, courseID, by string) error {
	if _, err := db.Exec(ctx, `
		UPDATE attendance_sessions SET status = 'closed', closed_at = now(), closed_by = $2
		WHERE id = $1 AND status = 'open'`, windowID, by); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO attendance_records (attendance_session_id, student_id, status, method, recorded_by)
		SELECT $1, e.student_id, 'absent', 'auto', $3
		FROM enrollments e WHERE e.course_id = $2 AND e.status IN ('active', 'completed')
		ON CONFLICT (attendance_session_id, student_id) DO NOTHING`, windowID, courseID, by)
	return err
}

func (r *Repository) summary(ctx context.Context, w *Window) (Summary, error) {
	var s Summary
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM enrollments e WHERE e.course_id = $2 AND e.status IN ('active', 'completed')),
			count(*) FILTER (WHERE status = 'present'),
			count(*) FILTER (WHERE status = 'late'),
			count(*) FILTER (WHERE status = 'absent'),
			count(*) FILTER (WHERE status = 'excused')
		FROM attendance_records WHERE attendance_session_id = $1`, w.ID, w.CourseID).
		Scan(&s.Enrolled, &s.Present, &s.Late, &s.Absent, &s.Excused)
	s.Pending = s.Enrolled - s.Present - s.Late - s.Absent - s.Excused
	if s.Pending < 0 {
		s.Pending = 0
	}
	return s, err
}

func (r *Repository) isEnrolled(ctx context.Context, db database.DBTX, studentID, courseID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM enrollments e JOIN user_roles ur ON ur.user_id = e.student_id
		JOIN roles ro ON ro.id = ur.role_id AND ro.name = 'student'
		WHERE e.student_id = $1 AND e.course_id = $2 AND e.status IN ('active', 'completed'))`, studentID, courseID).Scan(&ok)
	return ok, err
}

func (r *Repository) insertRecord(ctx context.Context, db database.DBTX, windowID, studentID, status, method string, checkedIn *time.Time, by string, reason *string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO attendance_records (attendance_session_id, student_id, status, method, checked_in_at, recorded_by, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (attendance_session_id, student_id) DO NOTHING
		RETURNING id`, windowID, studentID, status, method, checkedIn, by, reason).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrAlreadyRecorded
	}
	return id, err
}

const recordSelect = `
	SELECT r.id, w.session_id, r.student_id, u.name, r.status, r.method, r.checked_in_at, r.reason, r.updated_at
	FROM attendance_records r
	JOIN attendance_sessions w ON w.id = r.attendance_session_id
	JOIN users u ON u.id = r.student_id`

func scanRecord(row pgx.Row) (*Record, error) {
	var rec Record
	err := row.Scan(&rec.ID, &rec.SessionID, &rec.StudentID, &rec.StudentName, &rec.Status, &rec.Method, &rec.CheckedInAt, &rec.Reason, &rec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &rec, err
}

func collectRecords(rows pgx.Rows) ([]Record, error) {
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Record, error) {
		rec, err := scanRecord(row)
		if err != nil {
			return Record{}, err
		}
		return *rec, nil
	})
}

func (r *Repository) record(ctx context.Context, db database.DBTX, id string, lock bool) (*Record, error) {
	q := recordSelect + ` WHERE r.id = $1`
	if lock {
		q += ` FOR UPDATE OF r`
	}
	return scanRecord(db.QueryRow(ctx, q, id))
}

func (r *Repository) recordCourse(ctx context.Context, id string) (string, error) {
	var courseID string
	err := r.db.QueryRow(ctx, `
		SELECT s.course_id FROM attendance_records r JOIN attendance_sessions w ON w.id = r.attendance_session_id
		JOIN class_sessions s ON s.id = w.session_id WHERE r.id = $1`, id).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return courseID, err
}

func (r *Repository) sessionRecords(ctx context.Context, sessionID, status string, page httpx.Page) ([]Record, int, error) {
	where := ` WHERE w.session_id = $1 AND ($2 = '' OR r.status = $2)`
	rows, err := r.db.Query(ctx, recordSelect+where+` ORDER BY u.name, r.id LIMIT $3 OFFSET $4`, sessionID, status, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, err
	}
	list, err := collectRecords(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM attendance_records r JOIN attendance_sessions w ON w.id = r.attendance_session_id`+where, sessionID, status).Scan(&total)
	return list, total, err
}

func (r *Repository) updateRecord(ctx context.Context, db database.DBTX, id, status, reason, by string) error {
	_, err := db.Exec(ctx, `
		UPDATE attendance_records SET status = $2, reason = $3, recorded_by = $4,
			method = CASE WHEN method = 'auto' THEN 'manual' ELSE method END
		WHERE id = $1`, id, status, reason, by)
	return err
}

const studentScope = `
	FROM attendance_records r
	JOIN attendance_sessions w ON w.id = r.attendance_session_id
	JOIN class_sessions s ON s.id = w.session_id
	JOIN courses c ON c.id = s.course_id
	JOIN users u ON u.id = r.student_id
	WHERE r.student_id = $4 AND ` + learning.CourseVisibility + `
	AND ($5::uuid IS NULL OR c.id = $5::uuid)`

func (r *Repository) studentSummary(ctx context.Context, v learning.Viewer, studentID, courseID string) (StudentSummary, error) {
	var s StudentSummary
	err := r.db.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE r.status = 'present'), count(*) FILTER (WHERE r.status = 'late'),
			count(*) FILTER (WHERE r.status = 'absent'), count(*) FILTER (WHERE r.status = 'excused')`+studentScope,
		v.IsAdmin, v.UserID, v.Roles, studentID, nullable(courseID)).
		Scan(&s.TotalSessions, &s.Present, &s.Late, &s.Absent, &s.Excused)
	s.Rate = attendanceRate(s)
	return s, err
}

func (r *Repository) studentRecords(ctx context.Context, v learning.Viewer, studentID, courseID string, page httpx.Page) ([]Record, int, error) {
	args := []any{v.IsAdmin, v.UserID, v.Roles, studentID, nullable(courseID)}
	rows, err := r.db.Query(ctx, `
		SELECT r.id, w.session_id, r.student_id, u.name, r.status, r.method, r.checked_in_at, r.reason, r.updated_at`+studentScope+`
		ORDER BY s.starts_at DESC, r.id LIMIT $6 OFFSET $7`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := collectRecords(rows)
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.db.QueryRow(ctx, `SELECT count(*)`+studentScope, args...).Scan(&total)
	return list, total, err
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
