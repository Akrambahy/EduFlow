package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type Config struct {
	TokenTTL     time.Duration
	GraceMinutes int
}

type Service struct {
	repo     *Repository
	learning *learning.Service
	cfg      Config
	now      func() time.Time
}

func NewService(repo *Repository, l *learning.Service, cfg Config) *Service {
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = 30 * time.Second
	}
	return &Service{repo: repo, learning: l, cfg: cfg, now: time.Now}
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.repo.db, fn)
}

func record(ctx context.Context, tx pgx.Tx, v learning.Viewer, action, targetType, id string, payload map[string]any) error {
	return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: action, TargetType: targetType, TargetID: id, Context: payload, IP: v.IP})
}

func (s *Service) managedSession(ctx context.Context, v learning.Viewer, sessionID string) (*classSession, error) {
	cs, err := s.repo.classSession(ctx, s.repo.db, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, cs.CourseID); err != nil {
		return nil, err
	}
	return cs, nil
}

func (s *Service) staffSession(ctx context.Context, v learning.Viewer, sessionID string) (*classSession, error) {
	cs, err := s.repo.classSession(ctx, s.repo.db, sessionID)
	if err != nil {
		return nil, err
	}
	err = s.learning.CheckManage(ctx, v, cs.CourseID)
	if err == nil {
		return cs, nil
	}
	if !errors.Is(err, learning.ErrForbidden) {
		return nil, err
	}
	if v.Has(identity.RoleMentor) || v.Has(identity.RoleTracker) {
		ok, verr := s.repo.sessionVisible(ctx, v, sessionID)
		if verr != nil {
			return nil, verr
		}
		if ok {
			return cs, nil
		}
	}
	return nil, ErrForbidden
}

func (s *Service) withToken(ctx context.Context, w *Window) (*Window, error) {
	if w.Status == WindowOpen {
		tok, err := IssueToken(w.ID, w.secret, time.Duration(w.TokenTTLSeconds)*time.Second, s.now())
		if err != nil {
			return nil, err
		}
		w.Token = &tok.Value
		w.TokenExpiresAt = &tok.ExpiresAt
	}
	return s.withSummary(ctx, w)
}

func (s *Service) withSummary(ctx context.Context, w *Window) (*Window, error) {
	sum, err := s.repo.summary(ctx, w)
	if err != nil {
		return nil, err
	}
	w.Summary = sum
	return w, nil
}

func (s *Service) Start(ctx context.Context, v learning.Viewer, sessionID string, grace *int) (*Window, error) {
	cs, err := s.managedSession(ctx, v, sessionID)
	if err != nil {
		return nil, err
	}
	if cs.Cancelled {
		return nil, ErrSessionCancelled
	}
	g := s.cfg.GraceMinutes
	if grace != nil {
		g = *grace
	}
	var w *Window
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		secret, err := NewSecret()
		if err != nil {
			return err
		}
		created, err := s.repo.openWindow(ctx, tx, sessionID, secret, int(s.cfg.TokenTTL/time.Second), g, v.UserID)
		if err != nil {
			return err
		}
		w, err = s.repo.windowBySession(ctx, tx, sessionID, true)
		if err != nil {
			return err
		}
		if w.Status == WindowClosed {
			return ErrWindowClosed
		}
		if !created {
			return nil
		}
		return record(ctx, tx, v, "attendance.started", "session", sessionID, map[string]any{"graceMinutes": w.GraceMinutes})
	})
	if err != nil {
		return nil, err
	}
	return s.withToken(ctx, w)
}

func (s *Service) Token(ctx context.Context, v learning.Viewer, sessionID string) (*Window, error) {
	if _, err := s.managedSession(ctx, v, sessionID); err != nil {
		return nil, err
	}
	w, err := s.repo.windowBySession(ctx, s.repo.db, sessionID, false)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrWindowNotOpen
	}
	if err != nil {
		return nil, err
	}
	if w.Status == WindowClosed {
		return nil, ErrWindowClosed
	}
	return s.withToken(ctx, w)
}

func (s *Service) Close(ctx context.Context, v learning.Viewer, sessionID string) (*Window, error) {
	cs, err := s.managedSession(ctx, v, sessionID)
	if err != nil {
		return nil, err
	}
	var w *Window
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		w, err = s.repo.windowBySession(ctx, tx, sessionID, true)
		if errors.Is(err, ErrNotFound) {
			return ErrWindowNotOpen
		}
		if err != nil {
			return err
		}
		if w.Status == WindowClosed {
			return nil
		}
		if err := s.repo.closeWindow(ctx, tx, w.ID, cs.CourseID, v.UserID); err != nil {
			return err
		}
		if err := record(ctx, tx, v, "attendance.closed", "session", sessionID, nil); err != nil {
			return err
		}
		w, err = s.repo.windowBySession(ctx, tx, sessionID, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.withSummary(ctx, w)
}

func (s *Service) Window(ctx context.Context, v learning.Viewer, sessionID string) (*Window, error) {
	if _, err := s.staffSession(ctx, v, sessionID); err != nil {
		return nil, err
	}
	w, err := s.repo.windowBySession(ctx, s.repo.db, sessionID, false)
	if err != nil {
		return nil, err
	}
	return s.withSummary(ctx, w)
}

func (s *Service) CheckIn(ctx context.Context, v learning.Viewer, token string) (*Record, error) {
	if !v.Has(identity.RoleStudent) {
		return nil, ErrForbidden
	}
	windowID, err := ParseWindowID(token)
	if err != nil {
		return nil, err
	}
	w, err := s.repo.windowByID(ctx, s.repo.db, windowID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	if err := VerifyToken(token, w.secret, time.Duration(w.TokenTTLSeconds)*time.Second, now); err != nil {
		return nil, err
	}
	if w.Status != WindowOpen {
		return nil, ErrWindowClosed
	}
	cs, err := s.repo.classSession(ctx, s.repo.db, w.SessionID)
	if err != nil {
		return nil, err
	}
	if cs.Cancelled {
		return nil, ErrSessionCancelled
	}

	var id string
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		locked, err := s.repo.windowBySession(ctx, tx, w.SessionID, false)
		if err != nil {
			return err
		}
		if locked.Status != WindowOpen {
			return ErrWindowClosed
		}
		ok, err := s.repo.isEnrolled(ctx, tx, v.UserID, cs.CourseID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotEnrolled
		}
		status := checkInStatus(cs.StartsAt, time.Duration(w.GraceMinutes)*time.Minute, now)
		id, err = s.repo.insertRecord(ctx, tx, w.ID, v.UserID, status, MethodQR, &now, v.UserID, nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.repo.record(ctx, s.repo.db, id, false)
}

func (s *Service) Records(ctx context.Context, v learning.Viewer, sessionID, status string, page httpx.Page) ([]Record, int, error) {
	if _, err := s.staffSession(ctx, v, sessionID); err != nil {
		return nil, 0, err
	}
	return s.repo.sessionRecords(ctx, sessionID, status, page)
}

func (s *Service) Manual(ctx context.Context, v learning.Viewer, sessionID, studentID, status, reason string) (*Record, error) {
	cs, err := s.managedSession(ctx, v, sessionID)
	if err != nil {
		return nil, err
	}
	var id string
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		w, err := s.repo.windowBySession(ctx, tx, sessionID, true)
		if errors.Is(err, ErrNotFound) {
			return ErrWindowNotOpen
		}
		if err != nil {
			return err
		}
		ok, err := s.repo.isEnrolled(ctx, tx, studentID, cs.CourseID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotStudent
		}
		var at *time.Time
		if status == StatusPresent || status == StatusLate {
			now := s.now()
			at = &now
		}
		id, err = s.repo.insertRecord(ctx, tx, w.ID, studentID, status, MethodManual, at, v.UserID, &reason)
		if err != nil {
			return err
		}
		return record(ctx, tx, v, "attendance.manual", "attendance_record", id, map[string]any{"sessionId": sessionID, "studentId": studentID, "status": status, "reason": reason})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.record(ctx, s.repo.db, id, false)
}

func (s *Service) Correct(ctx context.Context, v learning.Viewer, recordID, status, reason string) (*Record, error) {
	courseID, err := s.repo.recordCourse(ctx, recordID)
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.record(ctx, tx, recordID, true)
		if err != nil {
			return err
		}
		if err := s.repo.updateRecord(ctx, tx, recordID, status, reason, v.UserID); err != nil {
			return err
		}
		return record(ctx, tx, v, "attendance.corrected", "attendance_record", recordID, map[string]any{
			"studentId": current.StudentID, "before": current.Status, "after": status, "reason": reason,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.record(ctx, s.repo.db, recordID, false)
}

func (s *Service) canViewStudent(ctx context.Context, v learning.Viewer, studentID string) error {
	if v.IsAdmin || v.UserID == studentID {
		return nil
	}
	if !v.Has(identity.RoleInstructor) && !v.Has(identity.RoleMentor) && !v.Has(identity.RoleTracker) {
		return ErrForbidden
	}
	ids, err := s.learning.VisibleUserIDs(ctx, v.UserID, v.Roles)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == studentID {
			return nil
		}
	}
	return ErrForbidden
}

func (s *Service) StudentSummary(ctx context.Context, v learning.Viewer, studentID, courseID string) (StudentSummary, error) {
	if err := s.canViewStudent(ctx, v, studentID); err != nil {
		return StudentSummary{}, err
	}
	return s.repo.studentSummary(ctx, v, studentID, courseID)
}

func (s *Service) StudentRecords(ctx context.Context, v learning.Viewer, studentID, courseID string, page httpx.Page) ([]Record, int, error) {
	if err := s.canViewStudent(ctx, v, studentID); err != nil {
		return nil, 0, err
	}
	return s.repo.studentRecords(ctx, v, studentID, courseID, page)
}
