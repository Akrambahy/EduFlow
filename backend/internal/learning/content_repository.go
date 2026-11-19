package learning

import (
	"context"
	"errors"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
)

const outlineQuery = `
	SELECT m.id, m.title, m.position, l.id, l.title, l.type, l.duration_minutes, l.position,
		lp.student_id IS NOT NULL, lp.completed_at IS NOT NULL,
		COALESCE(bool_and(l.id IS NULL OR lp.completed_at IS NOT NULL) OVER (
			ORDER BY m.position, l.position NULLS LAST
			ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING), true)
	FROM modules m
	LEFT JOIN lessons l ON l.module_id = m.id
	LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.student_id = $2
	WHERE m.course_id = $1
	ORDER BY m.position, l.position NULLS LAST`

func (r *Repository) Outline(ctx context.Context, db database.DBTX, courseID string, sequential bool, studentID string) ([]Module, error) {
	var student any
	if studentID != "" {
		student = studentID
	}
	rows, err := db.Query(ctx, outlineQuery, courseID, student)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modules := []Module{}
	for rows.Next() {
		var (
			mID, mTitle                 string
			mPos                        int
			lID, lTitle, lType          *string
			lMinutes, lPos              *int
			started, completed, prevAll bool
		)
		if err := rows.Scan(&mID, &mTitle, &mPos, &lID, &lTitle, &lType, &lMinutes, &lPos, &started, &completed, &prevAll); err != nil {
			return nil, err
		}
		if len(modules) == 0 || modules[len(modules)-1].ID != mID {
			modules = append(modules, Module{ID: mID, CourseID: courseID, Title: mTitle, Order: mPos, Lessons: []Lesson{}})
		}
		if lID == nil {
			continue
		}
		status := LessonAvailable
		if studentID != "" {
			switch {
			case completed:
				status = LessonCompleted
			case sequential && !prevAll:
				status = LessonLocked
			case started:
				status = LessonInProgress
			}
		}
		m := &modules[len(modules)-1]
		m.Lessons = append(m.Lessons, Lesson{
			ID: *lID, ModuleID: mID, CourseID: courseID, Title: *lTitle, Type: *lType,
			DurationMinutes: *lMinutes, Duration: minutesLabel(*lMinutes), Order: *lPos, Status: status,
		})
	}
	return modules, rows.Err()
}

type lessonRef struct {
	ModuleID string
	CourseID string
	Content  string
	VideoURL *string
}

func (r *Repository) lessonRef(ctx context.Context, db database.DBTX, lessonID string) (*lessonRef, error) {
	var ref lessonRef
	err := db.QueryRow(ctx, `
		SELECT l.module_id, m.course_id, l.content, l.video_url
		FROM lessons l JOIN modules m ON m.id = l.module_id WHERE l.id = $1`, lessonID).
		Scan(&ref.ModuleID, &ref.CourseID, &ref.Content, &ref.VideoURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &ref, err
}

func (r *Repository) moduleCourse(ctx context.Context, db database.DBTX, moduleID string) (string, error) {
	var courseID string
	err := db.QueryRow(ctx, `SELECT course_id FROM modules WHERE id = $1`, moduleID).Scan(&courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return courseID, err
}

func (r *Repository) insertPositioned(ctx context.Context, db database.DBTX, table, parentCol, parentID string, position *int) (int, error) {
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE `+parentCol+` = $1`, parentID).Scan(&count); err != nil {
		return 0, err
	}
	pos := count + 1
	if position != nil && *position >= 1 && *position <= count {
		pos = *position
		if _, err := db.Exec(ctx, `UPDATE `+table+` SET position = position + 1 WHERE `+parentCol+` = $1 AND position >= $2`, parentID, pos); err != nil {
			return 0, err
		}
	}
	return pos, nil
}

func (r *Repository) reposition(ctx context.Context, db database.DBTX, table, parentCol, parentID, id string, position int) error {
	rows, err := db.Query(ctx, `SELECT id FROM `+table+` WHERE `+parentCol+` = $1 ORDER BY position, id`, parentID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	ordered := make([]string, 0, len(ids))
	for _, x := range ids {
		if x != id {
			ordered = append(ordered, x)
		}
	}
	if position < 1 {
		position = 1
	}
	if position > len(ordered)+1 {
		position = len(ordered) + 1
	}
	ordered = append(ordered[:position-1], append([]string{id}, ordered[position-1:]...)...)
	_, err = db.Exec(ctx, `
		UPDATE `+table+` t SET position = x.n
		FROM unnest($1::uuid[]) WITH ORDINALITY AS x(id, n)
		WHERE t.id = x.id AND t.position <> x.n`, ordered)
	return err
}

func (r *Repository) compact(ctx context.Context, db database.DBTX, table, parentCol, parentID string) error {
	_, err := db.Exec(ctx, `
		UPDATE `+table+` t SET position = x.n
		FROM (SELECT id, row_number() OVER (ORDER BY position, id) AS n FROM `+table+` WHERE `+parentCol+` = $1) x
		WHERE t.id = x.id AND t.position <> x.n`, parentID)
	return err
}

func (r *Repository) InsertModule(ctx context.Context, db database.DBTX, courseID, title string, position *int) (string, error) {
	pos, err := r.insertPositioned(ctx, db, "modules", "course_id", courseID, position)
	if err != nil {
		return "", err
	}
	var id string
	err = db.QueryRow(ctx, `INSERT INTO modules (course_id, title, position) VALUES ($1, $2, $3) RETURNING id`, courseID, title, pos).Scan(&id)
	return id, err
}

func (r *Repository) UpdateModule(ctx context.Context, db database.DBTX, id, courseID string, title *string, position *int) error {
	if title != nil {
		if _, err := db.Exec(ctx, `UPDATE modules SET title = $2 WHERE id = $1`, id, *title); err != nil {
			return err
		}
	}
	if position != nil {
		return r.reposition(ctx, db, "modules", "course_id", courseID, id, *position)
	}
	return nil
}

func (r *Repository) DeleteModule(ctx context.Context, db database.DBTX, id, courseID string) error {
	var lessons int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM lessons WHERE module_id = $1`, id).Scan(&lessons); err != nil {
		return err
	}
	if lessons > 0 {
		return ErrModuleNotEmpty
	}
	if _, err := db.Exec(ctx, `DELETE FROM modules WHERE id = $1`, id); err != nil {
		return err
	}
	return r.compact(ctx, db, "modules", "course_id", courseID)
}

type LessonInput struct {
	Title           *string
	Type            *string
	DurationMinutes *int
	Content         *string
	VideoURL        *string
	Position        *int
}

func (r *Repository) InsertLesson(ctx context.Context, db database.DBTX, moduleID string, in LessonInput) (string, error) {
	pos, err := r.insertPositioned(ctx, db, "lessons", "module_id", moduleID, in.Position)
	if err != nil {
		return "", err
	}
	var id string
	err = db.QueryRow(ctx, `
		INSERT INTO lessons (module_id, title, type, duration_minutes, content, video_url, position)
		VALUES ($1, $2, COALESCE($3, 'text'), COALESCE($4, 10), COALESCE($5, ''), NULLIF($6, ''), $7) RETURNING id`,
		moduleID, in.Title, in.Type, in.DurationMinutes, in.Content, in.VideoURL, pos).Scan(&id)
	return id, err
}

func (r *Repository) UpdateLesson(ctx context.Context, db database.DBTX, id, moduleID string, in LessonInput) error {
	if _, err := db.Exec(ctx, `
		UPDATE lessons SET
			title = COALESCE($2, title),
			type = COALESCE($3, type),
			duration_minutes = COALESCE($4, duration_minutes),
			content = COALESCE($5, content),
			video_url = CASE WHEN $6::text IS NULL THEN video_url ELSE NULLIF($6, '') END
		WHERE id = $1`, id, in.Title, in.Type, in.DurationMinutes, in.Content, in.VideoURL); err != nil {
		return err
	}
	if in.Position != nil {
		return r.reposition(ctx, db, "lessons", "module_id", moduleID, id, *in.Position)
	}
	return nil
}

func (r *Repository) DeleteLesson(ctx context.Context, db database.DBTX, id, moduleID string) error {
	var progress bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM lesson_progress WHERE lesson_id = $1)`, id).Scan(&progress); err != nil {
		return err
	}
	if progress {
		return ErrLessonHasProgress
	}
	if _, err := db.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, id); err != nil {
		return err
	}
	return r.compact(ctx, db, "lessons", "module_id", moduleID)
}
