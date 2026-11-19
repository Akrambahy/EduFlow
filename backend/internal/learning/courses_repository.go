package learning

import (
	"context"
	"errors"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

const courseSelect = `
	SELECT c.id, c.track_id, t.name, c.instructor_id, iu.name, c.title, c.description, c.category, c.color,
		c.duration_weeks, c.sequential_unlock, c.status, c.published_at, c.created_at, c.updated_at,
		(SELECT count(*) FROM modules m WHERE m.course_id = c.id),
		(SELECT count(*) FROM lessons l JOIN modules m ON m.id = l.module_id WHERE m.course_id = c.id),
		(SELECT count(*) FROM enrollments e WHERE e.course_id = c.id AND e.status IN ('active', 'completed'))
	FROM courses c
	LEFT JOIN tracks t ON t.id = c.track_id
	LEFT JOIN users iu ON iu.id = c.instructor_id`

const courseVisible = `($1
	OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2)
	OR (c.status <> 'draft' AND EXISTS (
		SELECT 1 FROM enrollments e WHERE e.course_id = c.id AND e.student_id = $2 AND e.status IN ('active', 'completed')))
	OR (c.status <> 'draft' AND 'mentor' = ANY($3::text[]) AND EXISTS (
		SELECT 1 FROM track_mentors tm WHERE tm.track_id = c.track_id AND tm.user_id = $2))
	OR (c.status <> 'draft' AND 'tracker' = ANY($3::text[]) AND EXISTS (
		SELECT 1 FROM track_trackers tt WHERE tt.track_id = c.track_id AND tt.user_id = $2)))`

type CourseFilter struct {
	Status   string
	TrackID  string
	Category string
	Query    string
}

func scanCourse(row pgx.Row) (*Course, error) {
	var c Course
	err := row.Scan(&c.ID, &c.TrackID, &c.TrackName, &c.InstructorID, &c.InstructorName, &c.Title, &c.Description,
		&c.Category, &c.Color, &c.DurationWeeks, &c.SequentialUnlock, &c.Status, &c.PublishedAt,
		&c.CreatedAt, &c.UpdatedAt, &c.TotalModules, &c.TotalLessons, &c.EnrolledCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.Duration = weeksLabel(c.DurationWeeks)
	return &c, nil
}

func (r *Repository) ListCourses(ctx context.Context, v Viewer, f CourseFilter, page httpx.Page) ([]Course, int, error) {
	pattern := ""
	if q := strings.TrimSpace(f.Query); q != "" {
		pattern = "%" + escapeLike(q) + "%"
	}
	where := ` WHERE ` + courseVisible + `
		AND ($4 = '' OR c.status = $4)
		AND ($5::uuid IS NULL OR c.track_id = $5::uuid)
		AND ($6 = '' OR lower(c.category) = lower($6))
		AND ($7 = '' OR c.title ILIKE $7 ESCAPE '\')`
	args := []any{v.IsAdmin, v.UserID, v.Roles, f.Status, nullable(f.TrackID), f.Category, pattern}

	rows, err := r.db.Query(ctx, courseSelect+where+` ORDER BY c.title, c.id LIMIT $8 OFFSET $9`,
		append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	courses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Course, error) {
		c, err := scanCourse(row)
		if err != nil {
			return Course{}, err
		}
		return *c, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM courses c`+where, args...).Scan(&total)
	return courses, total, err
}

func (r *Repository) Course(ctx context.Context, v Viewer, id string) (*Course, error) {
	return scanCourse(r.db.QueryRow(ctx, courseSelect+` WHERE c.id = $4 AND `+courseVisible, v.IsAdmin, v.UserID, v.Roles, id))
}

func (r *Repository) CourseUnscoped(ctx context.Context, id string) (*Course, error) {
	return scanCourse(r.db.QueryRow(ctx, courseSelect+` WHERE c.id = $1`, id))
}

type courseOwner struct {
	InstructorID *string
	Status       string
	Sequential   bool
}

func (r *Repository) lockCourse(ctx context.Context, db database.DBTX, id string) (*courseOwner, error) {
	var o courseOwner
	err := db.QueryRow(ctx, `SELECT instructor_id, status, sequential_unlock FROM courses WHERE id = $1 FOR UPDATE`, id).
		Scan(&o.InstructorID, &o.Status, &o.Sequential)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &o, err
}

type CourseInput struct {
	TrackID          *string
	InstructorID     *string
	Title            *string
	Description      *string
	Category         *string
	Color            *string
	DurationWeeks    *int
	SequentialUnlock *bool
	Status           *string
}

func (r *Repository) InsertCourse(ctx context.Context, db database.DBTX, in CourseInput, createdBy string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO courses (track_id, instructor_id, title, description, category, color, duration_weeks,
			sequential_unlock, status, published_at, created_by)
		VALUES ($1, $2, $3, COALESCE($4, ''), COALESCE($5, 'General'), COALESCE($6, 'teal'), COALESCE($7, 1),
			COALESCE($8, false), COALESCE($9, 'draft'), CASE WHEN $9 = 'published' THEN now() END, $10)
		RETURNING id`,
		in.TrackID, in.InstructorID, in.Title, in.Description, in.Category, in.Color, in.DurationWeeks,
		in.SequentialUnlock, in.Status, createdBy).Scan(&id)
	if database.IsForeignKeyViolation(err) {
		return "", ErrInvalidReference
	}
	return id, err
}

func (r *Repository) UpdateCourse(ctx context.Context, db database.DBTX, id string, in CourseInput, clearTrack bool) error {
	_, err := db.Exec(ctx, `
		UPDATE courses SET
			track_id = CASE WHEN $11 THEN NULL ELSE COALESCE($2, track_id) END,
			instructor_id = COALESCE($3, instructor_id),
			title = COALESCE($4, title),
			description = COALESCE($5, description),
			category = COALESCE($6, category),
			color = COALESCE($7, color),
			duration_weeks = COALESCE($8, duration_weeks),
			sequential_unlock = COALESCE($9, sequential_unlock),
			status = COALESCE($10, status),
			published_at = CASE WHEN $10 = 'published' AND published_at IS NULL THEN now() ELSE published_at END
		WHERE id = $1`,
		id, in.TrackID, in.InstructorID, in.Title, in.Description, in.Category, in.Color, in.DurationWeeks,
		in.SequentialUnlock, in.Status, clearTrack)
	if database.IsForeignKeyViolation(err) {
		return ErrInvalidReference
	}
	return err
}

func (r *Repository) userHasRole(ctx context.Context, db database.DBTX, userID, role string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id = u.id JOIN roles r ON r.id = ur.role_id
		WHERE u.id = $1 AND u.is_active AND r.name = $2)`, userID, role).Scan(&ok)
	return ok, err
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
