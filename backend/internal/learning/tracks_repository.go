package learning

import (
	"context"
	"errors"

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

func (r *Repository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, r.db, fn)
}

const trackSelect = `
	SELECT t.id, t.name, t.description, t.status, t.created_at, t.updated_at,
		ARRAY(SELECT c.id::text FROM courses c WHERE c.track_id = t.id ORDER BY c.created_at, c.id),
		(SELECT count(DISTINCT e.student_id) FROM enrollments e JOIN courses c ON c.id = e.course_id
		 WHERE c.track_id = t.id AND e.status IN ('active', 'completed')),
		ARRAY(SELECT tm.user_id::text FROM track_mentors tm WHERE tm.track_id = t.id ORDER BY tm.assigned_at, tm.user_id),
		ARRAY(SELECT tt.user_id::text FROM track_trackers tt WHERE tt.track_id = t.id ORDER BY tt.assigned_at, tt.user_id)
	FROM tracks t`

const trackVisible = `($1 OR t.status = 'active'
	OR EXISTS (SELECT 1 FROM track_mentors tm WHERE tm.track_id = t.id AND tm.user_id = $2)
	OR EXISTS (SELECT 1 FROM track_trackers tt WHERE tt.track_id = t.id AND tt.user_id = $2))`

func scanTrack(row pgx.Row) (*Track, error) {
	var t Track
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Status, &t.CreatedAt, &t.UpdatedAt,
		&t.Courses, &t.TotalStudents, &t.MentorIDs, &t.TrackerIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(t.MentorIDs) > 0 {
		t.MentorID = &t.MentorIDs[0]
	}
	if len(t.TrackerIDs) > 0 {
		t.TrackerID = &t.TrackerIDs[0]
	}
	return &t, nil
}

func (r *Repository) ListTracks(ctx context.Context, v Viewer, status string, page httpx.Page) ([]Track, int, error) {
	where := ` WHERE ` + trackVisible + ` AND ($3 = '' OR t.status = $3)`
	rows, err := r.db.Query(ctx, trackSelect+where+` ORDER BY t.name, t.id LIMIT $4 OFFSET $5`,
		v.IsAdmin, v.UserID, status, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, err
	}
	tracks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Track, error) {
		t, err := scanTrack(row)
		if err != nil {
			return Track{}, err
		}
		return *t, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM tracks t`+where, v.IsAdmin, v.UserID, status).Scan(&total)
	return tracks, total, err
}

func (r *Repository) Track(ctx context.Context, v Viewer, id string) (*Track, error) {
	return scanTrack(r.db.QueryRow(ctx, trackSelect+` WHERE t.id = $3 AND `+trackVisible, v.IsAdmin, v.UserID, id))
}

func (r *Repository) InsertTrack(ctx context.Context, db database.DBTX, name, description, status, createdBy string) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO tracks (name, description, status, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
		name, description, status, createdBy).Scan(&id)
	if database.IsUniqueViolation(err, "tracks_name_key") {
		return "", ErrTrackNameTaken
	}
	return id, err
}

func (r *Repository) UpdateTrack(ctx context.Context, db database.DBTX, id string, name, description, status *string) error {
	tag, err := db.Exec(ctx, `
		UPDATE tracks SET
			name = COALESCE($2, name),
			description = COALESCE($3, description),
			status = COALESCE($4, status)
		WHERE id = $1`, id, name, description, status)
	if database.IsUniqueViolation(err, "tracks_name_key") {
		return ErrTrackNameTaken
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) ReplaceTrackStaff(ctx context.Context, db database.DBTX, table, trackID string, userIDs []string, role, assignedBy string) error {
	if table != "track_mentors" && table != "track_trackers" {
		return errors.New("invalid staff table")
	}
	var valid int
	if err := db.QueryRow(ctx, `
		SELECT count(DISTINCT u.id) FROM users u
		JOIN user_roles ur ON ur.user_id = u.id JOIN roles r ON r.id = ur.role_id
		WHERE u.id = ANY($1::uuid[]) AND u.is_active AND r.name = $2`, userIDs, role).Scan(&valid); err != nil {
		return err
	}
	if valid != len(userIDs) {
		return ErrInvalidAssignee
	}
	if _, err := db.Exec(ctx, `DELETE FROM `+table+` WHERE track_id = $1 AND NOT (user_id = ANY($2::uuid[]))`, trackID, userIDs); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO `+table+` (track_id, user_id, assigned_by)
		SELECT $1, u, $3 FROM unnest($2::uuid[]) WITH ORDINALITY AS x(u, n) ORDER BY n
		ON CONFLICT (track_id, user_id) DO NOTHING`, trackID, userIDs, assignedBy)
	return err
}

func (r *Repository) LockTrack(ctx context.Context, db database.DBTX, id string) error {
	var got string
	err := db.QueryRow(ctx, `SELECT id FROM tracks WHERE id = $1 FOR UPDATE`, id).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
