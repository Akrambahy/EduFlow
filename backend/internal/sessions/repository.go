package sessions

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

const sessionSelect = `
	SELECT s.id, s.course_id, c.title, s.title, s.description, s.type, s.location, s.starts_at, s.ends_at,
		s.instructor_id, iu.name, s.recording_url, s.recording_file_id, s.cancelled_at
	FROM class_sessions s
	JOIN courses c ON c.id = s.course_id
	LEFT JOIN users iu ON iu.id = s.instructor_id`

type Filter struct {
	CourseID string
	From     *time.Time
	To       *time.Time
	Status   string
}

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	err := row.Scan(&s.ID, &s.CourseID, &s.CourseTitle, &s.Title, &s.Description, &s.Type, &s.Location,
		&s.StartsAt, &s.EndsAt, &s.InstructorID, &s.InstructorName, &s.RecordingURL, &s.RecordingFileID, &s.cancelledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (r *Repository) List(ctx context.Context, v learning.Viewer, f Filter, page httpx.Page) ([]Session, int, error) {
	where := ` WHERE ` + learning.CourseVisibility + `
		AND ($4::uuid IS NULL OR s.course_id = $4::uuid)
		AND ($5::timestamptz IS NULL OR s.ends_at >= $5)
		AND ($6::timestamptz IS NULL OR s.starts_at < $6)
		AND (CASE $7
			WHEN 'upcoming' THEN s.cancelled_at IS NULL AND s.ends_at > now()
			WHEN 'completed' THEN s.cancelled_at IS NULL AND s.ends_at <= now()
			WHEN 'cancelled' THEN s.cancelled_at IS NOT NULL
			ELSE true END)`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(f.CourseID), f.From, f.To, f.Status}
	order := ` ORDER BY s.starts_at, s.id`
	if f.Status == StatusCompleted {
		order = ` ORDER BY s.starts_at DESC, s.id`
	}
	rows, err := r.db.Query(ctx, sessionSelect+where+order+` LIMIT $8 OFFSET $9`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Session, error) {
		s, err := scanSession(row)
		if err != nil {
			return Session{}, err
		}
		return *s, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM class_sessions s JOIN courses c ON c.id = s.course_id`+where, args...).Scan(&total)
	return list, total, err
}

func (r *Repository) Get(ctx context.Context, v learning.Viewer, id string) (*Session, error) {
	return scanSession(r.db.QueryRow(ctx, sessionSelect+` WHERE s.id = $4 AND `+learning.CourseVisibility, v.IsAdmin, v.UserID, v.Roles, id))
}

func (r *Repository) courseOf(ctx context.Context, db database.DBTX, sessionID string) (string, error) {
	var courseID string
	err := db.QueryRow(ctx, `SELECT course_id FROM class_sessions WHERE id = $1`, sessionID).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return courseID, err
}

type SessionInput struct {
	Title           *string
	Description     *string
	Type            *string
	Location        *string
	StartsAt        *time.Time
	EndsAt          *time.Time
	RecordingURL    *string
	RecordingFileID *string
	Cancelled       *bool
}

func mapCheck(err error) error {
	if database.IsCheckViolation(err) {
		return ErrInvalidTimes
	}
	return err
}

func (r *Repository) Insert(ctx context.Context, db database.DBTX, courseID string, in SessionInput, createdBy string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO class_sessions (course_id, instructor_id, title, description, type, location, starts_at, ends_at,
			recording_url, created_by)
		SELECT $1, c.instructor_id, $2, COALESCE($3, ''), COALESCE($4, 'lecture'), COALESCE($5, ''), $6, $7, NULLIF($8, ''), $9
		FROM courses c WHERE c.id = $1
		RETURNING id`,
		courseID, in.Title, in.Description, in.Type, in.Location, in.StartsAt, in.EndsAt, in.RecordingURL, createdBy).Scan(&id)
	return id, mapCheck(err)
}

func (r *Repository) Update(ctx context.Context, db database.DBTX, id string, in SessionInput) error {
	_, err := db.Exec(ctx, `
		UPDATE class_sessions SET
			title = COALESCE($2, title),
			description = COALESCE($3, description),
			type = COALESCE($4, type),
			location = COALESCE($5, location),
			starts_at = COALESCE($6, starts_at),
			ends_at = COALESCE($7, ends_at),
			recording_url = CASE WHEN $8::text IS NULL THEN recording_url ELSE NULLIF($8, '') END,
			recording_file_id = CASE WHEN $9::text IS NULL THEN recording_file_id ELSE NULLIF($9, '')::uuid END,
			cancelled_at = CASE
				WHEN $10::boolean IS NULL THEN cancelled_at
				WHEN $10 THEN COALESCE(cancelled_at, now())
				ELSE NULL END
		WHERE id = $1`,
		id, in.Title, in.Description, in.Type, in.Location, in.StartsAt, in.EndsAt, in.RecordingURL, in.RecordingFileID, in.Cancelled)
	return mapCheck(err)
}

const materialSelect = `
	SELECT m.id, m.course_id, m.session_id, m.title, m.description, m.url, m.created_at, m.updated_at,
		f.id, f.original_name, f.content_type, f.size_bytes
	FROM learning_materials m
	LEFT JOIN files f ON f.id = m.file_id`

func scanMaterial(row pgx.Row) (*Material, error) {
	var (
		m                          Material
		fileID, fileName, fileType *string
		fileSize                   *int64
	)
	err := row.Scan(&m.ID, &m.CourseID, &m.SessionID, &m.Title, &m.Description, &m.URL, &m.CreatedAt, &m.UpdatedAt,
		&fileID, &fileName, &fileType, &fileSize)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.Kind = "link"
	if fileID != nil {
		m.Kind = "file"
		m.File = &MaterialFile{ID: *fileID, Name: *fileName, ContentType: *fileType, Size: *fileSize}
	}
	return &m, nil
}

func (r *Repository) Materials(ctx context.Context, courseID, sessionID string, page httpx.Page) ([]Material, int, error) {
	where := ` WHERE m.course_id = $1 AND ($2::uuid IS NULL OR m.session_id = $2::uuid)`
	rows, err := r.db.Query(ctx, materialSelect+where+` ORDER BY m.created_at, m.id LIMIT $3 OFFSET $4`,
		courseID, nullable(sessionID), page.Limit, page.Offset)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Material, error) {
		m, err := scanMaterial(row)
		if err != nil {
			return Material{}, err
		}
		return *m, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM learning_materials m`+where, courseID, nullable(sessionID)).Scan(&total)
	return list, total, err
}

func (r *Repository) Material(ctx context.Context, db database.DBTX, id string) (*Material, error) {
	return scanMaterial(db.QueryRow(ctx, materialSelect+` WHERE m.id = $1`, id))
}

type MaterialInput struct {
	Title       *string
	Description *string
	URL         *string
	FileID      *string
	SessionID   *string
}

func (r *Repository) InsertMaterial(ctx context.Context, db database.DBTX, courseID string, in MaterialInput, by string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO learning_materials (course_id, session_id, title, description, file_id, url, created_by)
		VALUES ($1, $2, $3, COALESCE($4, ''), $5, $6, $7) RETURNING id`,
		courseID, in.SessionID, in.Title, in.Description, in.FileID, in.URL, by).Scan(&id)
	return id, err
}

func (r *Repository) UpdateMaterial(ctx context.Context, db database.DBTX, id string, in MaterialInput, clearSession bool) error {
	_, err := db.Exec(ctx, `
		UPDATE learning_materials SET
			title = COALESCE($2, title),
			description = COALESCE($3, description),
			session_id = CASE WHEN $5 THEN NULL ELSE COALESCE($4, session_id) END
		WHERE id = $1`, id, in.Title, in.Description, in.SessionID, clearSession)
	return err
}

func (r *Repository) DeleteMaterial(ctx context.Context, db database.DBTX, id string) error {
	_, err := db.Exec(ctx, `DELETE FROM learning_materials WHERE id = $1`, id)
	return err
}

func (r *Repository) sessionInCourse(ctx context.Context, db database.DBTX, sessionID, courseID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM class_sessions WHERE id = $1 AND course_id = $2)`, sessionID, courseID).Scan(&ok)
	return ok, err
}

func (r *Repository) FileVisible(ctx context.Context, v learning.Viewer, fileID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM learning_materials m JOIN courses c ON c.id = m.course_id
			WHERE m.file_id = $4 AND `+learning.CourseVisibility+`
		) OR EXISTS (
			SELECT 1 FROM class_sessions s JOIN courses c ON c.id = s.course_id
			WHERE s.recording_file_id = $4 AND `+learning.CourseVisibility+`
		)`, v.IsAdmin, v.UserID, v.Roles, fileID).Scan(&ok)
	return ok, err
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
