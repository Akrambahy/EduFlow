package communication

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

type Notification struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	Link      *string    `json:"link"`
	Read      bool       `json:"read"`
	ReadAt    *time.Time `json:"readAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

type Announcement struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	AuthorID    string    `json:"authorId"`
	AuthorName  string    `json:"authorName"`
	AuthorRole  string    `json:"authorRole"`
	CourseID    *string   `json:"courseId"`
	TargetRoles []string  `json:"targetRoles"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Service struct {
	db       *pgxpool.Pool
	learning *learning.Service
}

func NewService(db *pgxpool.Pool, l *learning.Service) *Service {
	return &Service{db: db, learning: l}
}

func (s *Service) Notifications(ctx context.Context, userID string, unreadOnly bool, page httpx.Page) ([]Notification, int, int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, type, title, message, link, read_at, created_at FROM notifications
		WHERE user_id = $1 AND (NOT $2 OR read_at IS NULL)
		ORDER BY created_at DESC, id LIMIT $3 OFFSET $4`, userID, unreadOnly, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Notification, error) {
		var n Notification
		err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.Link, &n.ReadAt, &n.CreatedAt)
		n.Read = n.ReadAt != nil
		return n, err
	})
	if err != nil {
		return nil, 0, 0, err
	}
	var total, unread int
	err = s.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE NOT $2 OR read_at IS NULL), count(*) FILTER (WHERE read_at IS NULL)
		FROM notifications WHERE user_id = $1`, userID, unreadOnly).Scan(&total, &unread)
	return list, total, unread, err
}

func (s *Service) MarkRead(ctx context.Context, userID, id string) (*Notification, error) {
	var n Notification
	err := s.db.QueryRow(ctx, `
		UPDATE notifications SET read_at = COALESCE(read_at, now()) WHERE id = $1 AND user_id = $2
		RETURNING id, type, title, message, link, read_at, created_at`, id, userID).
		Scan(&n.ID, &n.Type, &n.Title, &n.Message, &n.Link, &n.ReadAt, &n.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	n.Read = true
	return &n, err
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	tag, err := s.db.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, userID)
	return tag.RowsAffected(), err
}

const announcementSelect = `
	SELECT a.id, a.title, a.content, a.author_id, u.name,
		COALESCE((SELECT r.name FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = a.author_id
			ORDER BY array_position(ARRAY['admin','instructor','mentor','tracker','student'], r.name) LIMIT 1), ''),
		a.course_id, a.target_roles, a.created_at
	FROM announcements a JOIN users u ON u.id = a.author_id
	LEFT JOIN courses c ON c.id = a.course_id`

const announcementVisible = `a.deleted_at IS NULL AND ($1 OR a.author_id = $2 OR (
	(cardinality(a.target_roles) = 0 OR a.target_roles && $3::text[])
	AND (a.course_id IS NULL OR ` + learning.CourseVisibility + `)))`

func scanAnnouncement(row pgx.Row) (Announcement, error) {
	var a Announcement
	err := row.Scan(&a.ID, &a.Title, &a.Content, &a.AuthorID, &a.AuthorName, &a.AuthorRole, &a.CourseID, &a.TargetRoles, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

func (s *Service) Announcements(ctx context.Context, v learning.Viewer, courseID string, page httpx.Page) ([]Announcement, int, error) {
	where := ` WHERE ` + announcementVisible + ` AND ($4::uuid IS NULL OR a.course_id = $4::uuid)`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(courseID)}
	rows, err := s.db.Query(ctx, announcementSelect+where+` ORDER BY a.created_at DESC, a.id LIMIT $5 OFFSET $6`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Announcement, error) { return scanAnnouncement(row) })
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM announcements a LEFT JOIN courses c ON c.id = a.course_id`+where, args...).Scan(&total)
	return list, total, err
}

type AnnouncementInput struct {
	Title       string
	Content     string
	CourseID    *string
	TargetRoles []string
}

func (s *Service) CreateAnnouncement(ctx context.Context, v learning.Viewer, in AnnouncementInput) (*Announcement, error) {
	switch {
	case v.IsAdmin:
		if in.CourseID != nil {
			if _, err := s.learning.Course(ctx, v, *in.CourseID); err != nil {
				return nil, err
			}
		}
	case v.Has(identity.RoleInstructor):
		if in.CourseID == nil {
			return nil, ErrForbidden
		}
		if err := s.learning.CheckManage(ctx, v, *in.CourseID); err != nil {
			return nil, err
		}
	default:
		return nil, ErrForbidden
	}
	if in.TargetRoles == nil {
		in.TargetRoles = []string{}
	}
	var id string
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO announcements (title, content, author_id, course_id, target_roles) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			strings.TrimSpace(in.Title), in.Content, v.UserID, in.CourseID, in.TargetRoles).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO notifications (user_id, type, title, message, link, dedupe_key)
			SELECT DISTINCT u.id, 'announcement', $2, left($3, 300), '/dashboard', 'announcement:' || $1
			FROM users u
			WHERE u.is_active AND u.id <> $4
			AND (cardinality($5::text[]) = 0 OR EXISTS (
				SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = u.id AND r.name = ANY($5::text[])))
			AND ($6::uuid IS NULL OR EXISTS (
				SELECT 1 FROM courses c WHERE c.id = $6::uuid AND (
					c.instructor_id = u.id
					OR EXISTS (SELECT 1 FROM enrollments e WHERE e.course_id = c.id AND e.student_id = u.id AND e.status IN ('active', 'completed'))
					OR EXISTS (SELECT 1 FROM track_mentors tm WHERE tm.track_id = c.track_id AND tm.user_id = u.id)
					OR EXISTS (SELECT 1 FROM track_trackers tt WHERE tt.track_id = c.track_id AND tt.user_id = u.id))))
			ON CONFLICT (user_id, dedupe_key) DO NOTHING`,
			id, strings.TrimSpace(in.Title), in.Content, v.UserID, in.TargetRoles, in.CourseID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: "announcement.created", TargetType: "announcement", TargetID: id,
			Context: map[string]any{"courseId": in.CourseID, "targetRoles": in.TargetRoles}, IP: v.IP})
	})
	if err != nil {
		return nil, err
	}
	a, err := scanAnnouncement(s.db.QueryRow(ctx, announcementSelect+` WHERE a.id = $1`, id))
	return &a, err
}

func (s *Service) DeleteAnnouncement(ctx context.Context, v learning.Viewer, id string) error {
	var author string
	err := s.db.QueryRow(ctx, `SELECT author_id FROM announcements WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&author)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !v.IsAdmin && author != v.UserID {
		var visible bool
		_ = s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM announcements a LEFT JOIN courses c ON c.id = a.course_id WHERE a.id = $4 AND `+announcementVisible+`)`,
			v.IsAdmin, v.UserID, v.Roles, id).Scan(&visible)
		if !visible {
			return ErrNotFound
		}
		return ErrForbidden
	}
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE announcements SET deleted_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: "announcement.deleted", TargetType: "announcement", TargetID: id, IP: v.IP})
	})
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
