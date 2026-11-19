package sessions

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/communication"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo     *Repository
	learning *learning.Service
	loc      *time.Location
	now      func() time.Time
}

func NewService(repo *Repository, l *learning.Service, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{repo: repo, learning: l, loc: loc, now: time.Now}
}

func (s *Service) finalize(list []Session) {
	now := s.now()
	for i := range list {
		list[i].finalize(now, s.loc)
	}
}

func (s *Service) List(ctx context.Context, v learning.Viewer, f Filter, page httpx.Page) ([]Session, int, error) {
	list, total, err := s.repo.List(ctx, v, f, page)
	if err != nil {
		return nil, 0, err
	}
	s.finalize(list)
	return list, total, nil
}

func (s *Service) Get(ctx context.Context, v learning.Viewer, id string) (*Session, error) {
	sess, err := s.repo.Get(ctx, v, id)
	if err != nil {
		return nil, err
	}
	sess.finalize(s.now(), s.loc)
	return sess, nil
}

func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, s.repo.db, fn)
}

func record(ctx context.Context, tx pgx.Tx, v learning.Viewer, action, targetType, id string, payload map[string]any) error {
	return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: action, TargetType: targetType, TargetID: id, Context: payload, IP: v.IP})
}

func (s *Service) checkFile(ctx context.Context, db database.DBTX, v learning.Viewer, fileID *string) error {
	if fileID == nil || *fileID == "" {
		return nil
	}
	var ok bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM files WHERE id = $1 AND (owner_id = $2 OR $3))`, *fileID, v.UserID, v.IsAdmin).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrInvalidFile
	}
	return nil
}

func (s *Service) Create(ctx context.Context, v learning.Viewer, courseID string, in SessionInput) (*Session, error) {
	if !in.EndsAt.After(*in.StartsAt) {
		return nil, ErrInvalidTimes
	}
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	t := strings.TrimSpace(*in.Title)
	in.Title = &t
	var id string
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		var err error
		id, err = s.repo.Insert(ctx, tx, courseID, in, v.UserID)
		if err != nil {
			return err
		}
		if err := communication.NotifyCourseStudents(ctx, tx, courseID, communication.Message{
			Type: communication.TypeSession, Title: "New session: " + *in.Title, Message: in.StartsAt.UTC().Format(time.RFC3339),
			Link: "/schedule", Key: "session:created:" + id,
		}); err != nil {
			return err
		}
		return record(ctx, tx, v, "session.created", "session", id, map[string]any{"courseId": courseID})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, v, id)
}

func (s *Service) Update(ctx context.Context, v learning.Viewer, id string, in SessionInput) (*Session, error) {
	courseID, err := s.repo.courseOf(ctx, s.repo.db, id)
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
		if err := s.checkFile(ctx, tx, v, in.RecordingFileID); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, tx, id, in); err != nil {
			return err
		}
		action := "session.updated"
		if in.Cancelled != nil && *in.Cancelled {
			action = "session.cancelled"
		}
		if in.Cancelled != nil || in.StartsAt != nil || in.EndsAt != nil || in.Location != nil {
			title := "Session updated"
			if action == "session.cancelled" {
				title = "Session cancelled"
			}
			if err := communication.NotifyCourseStudents(ctx, tx, courseID, communication.Message{
				Type: communication.TypeSession, Title: title, Link: "/schedule",
				Key: "session:changed:" + id + ":" + strconv.FormatInt(time.Now().UnixNano(), 10),
			}); err != nil {
				return err
			}
		}
		return record(ctx, tx, v, action, "session", id, map[string]any{"courseId": courseID})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, v, id)
}

func (s *Service) Materials(ctx context.Context, v learning.Viewer, courseID, sessionID string, page httpx.Page) ([]Material, int, error) {
	if _, err := s.learning.Course(ctx, v, courseID); err != nil {
		return nil, 0, err
	}
	return s.repo.Materials(ctx, courseID, sessionID, page)
}

func (s *Service) CreateMaterial(ctx context.Context, v learning.Viewer, courseID string, in MaterialInput) (*Material, error) {
	if (in.URL == nil) == (in.FileID == nil) {
		return nil, ErrSourceRequired
	}
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	t := strings.TrimSpace(*in.Title)
	in.Title = &t
	var id string
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.checkFile(ctx, tx, v, in.FileID); err != nil {
			return err
		}
		if in.SessionID != nil {
			ok, err := s.repo.sessionInCourse(ctx, tx, *in.SessionID, courseID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInvalidSession
			}
		}
		var err error
		id, err = s.repo.InsertMaterial(ctx, tx, courseID, in, v.UserID)
		if err != nil {
			return err
		}
		return record(ctx, tx, v, "material.created", "material", id, map[string]any{"courseId": courseID})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Material(ctx, s.repo.db, id)
}

func (s *Service) UpdateMaterial(ctx context.Context, v learning.Viewer, id string, in MaterialInput, clearSession bool) (*Material, error) {
	m, err := s.repo.Material(ctx, s.repo.db, id)
	if err != nil {
		return nil, err
	}
	if err := s.learning.CheckManage(ctx, v, m.CourseID); err != nil {
		return nil, err
	}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if in.SessionID != nil {
			ok, err := s.repo.sessionInCourse(ctx, tx, *in.SessionID, m.CourseID)
			if err != nil {
				return err
			}
			if !ok {
				return ErrInvalidSession
			}
		}
		if err := s.repo.UpdateMaterial(ctx, tx, id, in, clearSession); err != nil {
			return err
		}
		return record(ctx, tx, v, "material.updated", "material", id, map[string]any{"courseId": m.CourseID})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Material(ctx, s.repo.db, id)
}

func (s *Service) DeleteMaterial(ctx context.Context, v learning.Viewer, id string) error {
	m, err := s.repo.Material(ctx, s.repo.db, id)
	if err != nil {
		return err
	}
	if err := s.learning.CheckManage(ctx, v, m.CourseID); err != nil {
		return err
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.DeleteMaterial(ctx, tx, id); err != nil {
			return err
		}
		return record(ctx, tx, v, "material.deleted", "material", id, map[string]any{"courseId": m.CourseID})
	})
}

func (s *Service) FileVisible(ctx context.Context, v learning.Viewer, fileID string) (bool, error) {
	return s.repo.FileVisible(ctx, v, fileID)
}
