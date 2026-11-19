package assessment

import (
	"context"
	"errors"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db       *pgxpool.Pool
	learning *learning.Service
	loc      *time.Location
	now      func() time.Time
}

func NewService(db *pgxpool.Pool, l *learning.Service, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{db: db, learning: l, loc: loc, now: time.Now}
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.db, fn)
}

func record(ctx context.Context, tx pgx.Tx, v learning.Viewer, action, targetType, id string, payload map[string]any) error {
	return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: action, TargetType: targetType, TargetID: id, Context: payload, IP: v.IP})
}

func (s *Service) canManage(ctx context.Context, v learning.Viewer, courseID string) (bool, error) {
	err := s.learning.CheckManage(ctx, v, courseID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, learning.ErrForbidden), errors.Is(err, learning.ErrNotFound):
		return false, nil
	default:
		return false, err
	}
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

func (s *Service) isEnrolledStudent(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, studentID, courseID string) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM enrollments e JOIN courses c ON c.id = e.course_id
		JOIN user_roles ur ON ur.user_id = e.student_id JOIN roles r ON r.id = ur.role_id AND r.name = 'student'
		WHERE e.student_id = $1 AND e.course_id = $2 AND e.status IN ('active', 'completed') AND c.status = 'published')`,
		studentID, courseID).Scan(&ok)
	return ok, err
}

func (s *Service) dateLabel(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.In(s.loc).Format("2006-01-02")
	return &v
}
