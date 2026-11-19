package communication

import (
	"context"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
)

const (
	TypeAssignment   = "assignment"
	TypeGrade        = "grade"
	TypeSession      = "session"
	TypeAnnouncement = "announcement"
	TypeCertificate  = "certificate"
	TypeAttendance   = "attendance"
	TypeSystem       = "system"
)

type Message struct {
	Type    string
	Title   string
	Message string
	Link    string
	Key     string
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func NotifyUsers(ctx context.Context, db database.DBTX, userIDs []string, m Message) error {
	if len(userIDs) == 0 {
		return nil
	}
	var link any
	if m.Link != "" {
		link = m.Link
	}
	_, err := db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, link, dedupe_key)
		SELECT u, $2, $3, $4, $5, $6 FROM unnest($1::uuid[]) AS u
		ON CONFLICT (user_id, dedupe_key) DO NOTHING`,
		userIDs, m.Type, truncate(m.Title, 200), truncate(m.Message, 2000), link, m.Key)
	return err
}

func NotifyCourseStudents(ctx context.Context, db database.DBTX, courseID string, m Message) error {
	var link any
	if m.Link != "" {
		link = m.Link
	}
	_, err := db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, link, dedupe_key)
		SELECT e.student_id, $2, $3, $4, $5, $6
		FROM enrollments e JOIN courses c ON c.id = e.course_id
		WHERE e.course_id = $1 AND e.status IN ('active', 'completed') AND c.status = 'published'
		ON CONFLICT (user_id, dedupe_key) DO NOTHING`,
		courseID, m.Type, truncate(m.Title, 200), truncate(m.Message, 2000), link, m.Key)
	return err
}
