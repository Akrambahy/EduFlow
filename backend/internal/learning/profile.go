package learning

import (
	"context"
	"errors"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/jackc/pgx/v5"
)

func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func (r *Repository) ids(ctx context.Context, sql string, args ...any) ([]string, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Service) ProfileExtras(ctx context.Context, userID string, roles []string) (identity.ProfileExtras, error) {
	var out identity.ProfileExtras
	var err error

	if hasRole(roles, identity.RoleStudent) {
		out.EnrolledCourses, err = s.repo.ids(ctx, `
			SELECT e.course_id::text FROM enrollments e JOIN courses c ON c.id = e.course_id
			WHERE e.student_id = $1 AND e.status IN ('active', 'completed') AND c.status <> 'draft'
			ORDER BY e.enrolled_at, e.course_id`, userID)
		if err != nil {
			return out, err
		}
		var name string
		err = s.repo.db.QueryRow(ctx, `
			SELECT t.name FROM enrollments e JOIN courses c ON c.id = e.course_id JOIN tracks t ON t.id = c.track_id
			WHERE e.student_id = $1 AND e.status IN ('active', 'completed')
			GROUP BY t.id, t.name ORDER BY count(*) DESC, min(e.enrolled_at) LIMIT 1`, userID).Scan(&name)
		if err == nil {
			out.Track = &name
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
	}
	if hasRole(roles, identity.RoleInstructor) {
		out.Courses, err = s.repo.ids(ctx, `
			SELECT id::text FROM courses WHERE instructor_id = $1 ORDER BY created_at, id`, userID)
		if err != nil {
			return out, err
		}
	}
	if hasRole(roles, identity.RoleMentor) || hasRole(roles, identity.RoleTracker) {
		out.Tracks, err = s.repo.ids(ctx, `
			SELECT DISTINCT track_id::text FROM (
				SELECT track_id FROM track_mentors WHERE user_id = $1 AND $2
				UNION SELECT track_id FROM track_trackers WHERE user_id = $1 AND $3
			) x ORDER BY 1`, userID, hasRole(roles, identity.RoleMentor), hasRole(roles, identity.RoleTracker))
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func (s *Service) VisibleUserIDs(ctx context.Context, viewerID string, roles []string) ([]string, error) {
	return s.repo.ids(ctx, `
		WITH my_courses AS (
			SELECT c.id, c.track_id FROM courses c
			WHERE ($2 AND c.instructor_id = $1)
			   OR ($3 AND c.track_id IN (SELECT track_id FROM track_mentors WHERE user_id = $1))
			   OR ($4 AND c.track_id IN (SELECT track_id FROM track_trackers WHERE user_id = $1))
		),
		my_enrolled AS (
			SELECT c.id, c.track_id, c.instructor_id FROM enrollments e JOIN courses c ON c.id = e.course_id
			WHERE e.student_id = $1 AND e.status IN ('active', 'completed') AND c.status <> 'draft'
		)
		SELECT $1::uuid::text
		UNION SELECT e.student_id::text FROM enrollments e JOIN my_courses mc ON mc.id = e.course_id
			WHERE e.status IN ('active', 'completed')
		UNION SELECT c.instructor_id::text FROM courses c JOIN my_courses mc ON mc.track_id = c.track_id
			WHERE c.instructor_id IS NOT NULL
		UNION SELECT tm.user_id::text FROM track_mentors tm JOIN my_courses mc ON mc.track_id = tm.track_id
		UNION SELECT tt.user_id::text FROM track_trackers tt JOIN my_courses mc ON mc.track_id = tt.track_id
		UNION SELECT me.instructor_id::text FROM my_enrolled me WHERE me.instructor_id IS NOT NULL
		UNION SELECT tm.user_id::text FROM track_mentors tm JOIN my_enrolled me ON me.track_id = tm.track_id
		UNION SELECT tt.user_id::text FROM track_trackers tt JOIN my_enrolled me ON me.track_id = tt.track_id`,
		viewerID, hasRole(roles, identity.RoleInstructor), hasRole(roles, identity.RoleMentor), hasRole(roles, identity.RoleTracker))
}
