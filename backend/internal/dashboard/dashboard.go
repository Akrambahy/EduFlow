package dashboard

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/performance"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db          *pgxpool.Pool
	performance *performance.Service
	loc         *time.Location
}

func NewService(db *pgxpool.Pool, p *performance.Service, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{db: db, performance: p, loc: loc}
}

type SessionCard struct {
	ID          string    `json:"id"`
	CourseID    string    `json:"courseId"`
	CourseTitle string    `json:"courseTitle"`
	Title       string    `json:"title"`
	StartsAt    time.Time `json:"startsAt"`
	Date        string    `json:"date"`
	StartTime   string    `json:"startTime"`
	Location    string    `json:"location"`
	Instructor  *string   `json:"instructorName"`
}

type WorkItem struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	CourseID    string    `json:"courseId"`
	CourseTitle string    `json:"courseTitle"`
	Title       string    `json:"title"`
	DueAt       time.Time `json:"dueAt"`
	DueDate     string    `json:"dueDate"`
}

func (s *Service) nextSessions(ctx context.Context, v learning.Viewer, limit int) ([]SessionCard, error) {
	rows, err := s.db.Query(ctx, `
		SELECT cs.id, cs.course_id, c.title, cs.title, cs.starts_at, cs.location, iu.name
		FROM class_sessions cs JOIN courses c ON c.id = cs.course_id LEFT JOIN users iu ON iu.id = cs.instructor_id
		WHERE cs.cancelled_at IS NULL AND cs.ends_at > now() AND c.status <> 'archived' AND `+learning.CourseVisibility+`
		ORDER BY cs.starts_at LIMIT $4`, v.IsAdmin, v.UserID, v.Roles, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (SessionCard, error) {
		var sc SessionCard
		err := row.Scan(&sc.ID, &sc.CourseID, &sc.CourseTitle, &sc.Title, &sc.StartsAt, &sc.Location, &sc.Instructor)
		local := sc.StartsAt.In(s.loc)
		sc.Date, sc.StartTime, sc.StartsAt = local.Format("2006-01-02"), local.Format("15:04"), sc.StartsAt.UTC()
		return sc, err
	})
}

func (s *Service) count(ctx context.Context, q string, args ...any) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

func (s *Service) unread(ctx context.Context, userID string) (int, error) {
	return s.count(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`, userID)
}

func (s *Service) Student(ctx context.Context, v learning.Viewer) (gin.H, error) {
	var active, completed int
	var avgProgress *float64
	err := s.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE e.status = 'active'), count(*) FILTER (WHERE e.status = 'completed'),
			avg(CASE WHEN e.status = 'completed' THEN 100 ELSE
				(SELECT CASE WHEN count(l.id) = 0 THEN 0 ELSE count(lp.lesson_id) * 100.0 / count(l.id) END
				 FROM lessons l JOIN modules m ON m.id = l.module_id
				 LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.student_id = e.student_id AND lp.completed_at IS NOT NULL
				 WHERE m.course_id = e.course_id) END)::float8
		FROM enrollments e JOIN courses c ON c.id = e.course_id
		WHERE e.student_id = $1 AND e.status IN ('active', 'completed') AND c.status <> 'draft'`, v.UserID).Scan(&active, &completed, &avgProgress)
	if err != nil {
		return nil, err
	}
	sessions, err := s.nextSessions(ctx, v, 3)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT a.id, 'assignment', a.course_id, c.title, a.title, a.due_at FROM assignments a JOIN courses c ON c.id = a.course_id
		JOIN enrollments e ON e.course_id = c.id AND e.student_id = $1 AND e.status = 'active'
		WHERE a.status = 'published' AND c.status = 'published'
		AND (a.due_at > now() OR (a.late_policy <> 'reject' AND (a.late_until IS NULL OR a.late_until > now())))
		AND NOT EXISTS (SELECT 1 FROM assignment_submissions s WHERE s.assignment_id = a.id AND s.student_id = $1 AND s.status IN ('submitted', 'graded'))
		UNION ALL
		SELECT q.id, 'quiz', q.course_id, c.title, q.title, q.due_at FROM quizzes q JOIN courses c ON c.id = q.course_id
		JOIN enrollments e ON e.course_id = c.id AND e.student_id = $1 AND e.status = 'active'
		WHERE q.status = 'published' AND c.status = 'published' AND q.due_at IS NOT NULL AND q.due_at > now()
		AND (q.opens_at IS NULL OR q.opens_at <= now())
		AND (SELECT count(*) FROM quiz_attempts at WHERE at.quiz_id = q.id AND at.student_id = $1 AND at.status <> 'in_progress') = 0
		ORDER BY 6 LIMIT 10`, v.UserID)
	if err != nil {
		return nil, err
	}
	work, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (WorkItem, error) {
		var w WorkItem
		err := row.Scan(&w.ID, &w.Kind, &w.CourseID, &w.CourseTitle, &w.Title, &w.DueAt)
		w.DueDate = w.DueAt.In(s.loc).Format("2006-01-02")
		return w, err
	})
	if err != nil {
		return nil, err
	}
	grades, err := s.performance.StudentGrades(ctx, v, v.UserID)
	if err != nil {
		return nil, err
	}
	unread, err := s.unread(ctx, v.UserID)
	if err != nil {
		return nil, err
	}
	var attendance *float64
	var attended, counted int
	if err := s.db.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status IN ('present', 'late')), count(*) FILTER (WHERE status <> 'excused')
		FROM attendance_records WHERE student_id = $1`, v.UserID).Scan(&attended, &counted); err != nil {
		return nil, err
	}
	if counted > 0 {
		a := float64(attended*100) / float64(counted)
		attendance = &a
	}
	var next *SessionCard
	if len(sessions) > 0 {
		next = &sessions[0]
	}
	return gin.H{
		"role": identity.RoleStudent, "activeCourses": active, "completedCourses": completed, "averageProgress": avgProgress,
		"nextSession": next, "upcomingSessions": sessions, "pendingWork": work, "pendingCount": len(work),
		"gpa": grades.GPA, "standing": grades.Standing, "attendanceRate": attendance, "unreadNotifications": unread,
	}, nil
}

func (s *Service) Instructor(ctx context.Context, v learning.Viewer) (gin.H, error) {
	var courses, students, toReview, projectsToReview, openAttendance int
	err := s.db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM courses c WHERE c.instructor_id = $1 AND c.status <> 'archived'),
			(SELECT count(DISTINCT e.student_id) FROM enrollments e JOIN courses c ON c.id = e.course_id
			 WHERE c.instructor_id = $1 AND e.status IN ('active', 'completed')),
			(SELECT count(*) FROM assignment_submissions s JOIN assignments a ON a.id = s.assignment_id JOIN courses c ON c.id = a.course_id
			 WHERE c.instructor_id = $1 AND s.status = 'submitted'),
			(SELECT count(*) FROM project_submissions s JOIN projects p ON p.id = s.project_id JOIN courses c ON c.id = p.course_id
			 WHERE c.instructor_id = $1 AND s.status = 'submitted'),
			(SELECT count(*) FROM attendance_sessions w JOIN class_sessions cs ON cs.id = w.session_id JOIN courses c ON c.id = cs.course_id
			 WHERE c.instructor_id = $1 AND w.status = 'open')`, v.UserID).
		Scan(&courses, &students, &toReview, &projectsToReview, &openAttendance)
	if err != nil {
		return nil, err
	}
	sessions, err := s.nextSessions(ctx, v, 5)
	if err != nil {
		return nil, err
	}
	unread, err := s.unread(ctx, v.UserID)
	if err != nil {
		return nil, err
	}
	var next *SessionCard
	if len(sessions) > 0 {
		next = &sessions[0]
	}
	return gin.H{
		"role": identity.RoleInstructor, "assignedCourses": courses, "students": students,
		"toReview": toReview + projectsToReview, "assignmentsToReview": toReview, "projectsToReview": projectsToReview,
		"openAttendance": openAttendance, "nextSession": next, "upcomingSessions": sessions, "unreadNotifications": unread,
	}, nil
}

func (s *Service) Staff(ctx context.Context, v learning.Viewer, role string) (gin.H, error) {
	var tracks, courses, learners int
	err := s.db.QueryRow(ctx, `
		WITH my_tracks AS (
			SELECT track_id FROM track_mentors WHERE user_id = $1 AND $2
			UNION SELECT track_id FROM track_trackers WHERE user_id = $1 AND $3)
		SELECT (SELECT count(*) FROM my_tracks),
			(SELECT count(*) FROM courses c WHERE c.track_id IN (SELECT track_id FROM my_tracks) AND c.status = 'published'),
			(SELECT count(DISTINCT e.student_id) FROM enrollments e JOIN courses c ON c.id = e.course_id
			 WHERE c.track_id IN (SELECT track_id FROM my_tracks) AND c.status = 'published' AND e.status IN ('active', 'completed'))`,
		v.UserID, v.Has(identity.RoleMentor), v.Has(identity.RoleTracker)).Scan(&tracks, &courses, &learners)
	if err != nil {
		return nil, err
	}
	risk, err := s.performance.AtRisk(ctx, v)
	if err != nil {
		return nil, err
	}
	missing := 0
	for _, r := range risk {
		missing += r.Missing
	}
	top := risk
	if len(top) > 5 {
		top = top[:5]
	}
	unread, err := s.unread(ctx, v.UserID)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"role": role, "tracks": tracks, "courses": courses, "learners": learners, "needAttention": len(risk),
		"missingWork": missing, "atRisk": top, "unreadNotifications": unread,
	}, nil
}

func (s *Service) Admin(ctx context.Context, v learning.Viewer) (gin.H, error) {
	rows, err := s.db.Query(ctx, `
		SELECT r.name, count(DISTINCT u.id) FROM roles r
		LEFT JOIN user_roles ur ON ur.role_id = r.id LEFT JOIN users u ON u.id = ur.user_id AND u.is_active
		GROUP BY r.name`)
	if err != nil {
		return nil, err
	}
	usersByRole := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			rows.Close()
			return nil, err
		}
		usersByRole[name] = n
	}
	rows.Close()
	var users, inactive, tracks, activeTracks, published, drafts, sessionsWeek, enrollments, certificates, audit24h int
	err = s.db.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM users WHERE is_active), (SELECT count(*) FROM users WHERE NOT is_active),
			(SELECT count(*) FROM tracks), (SELECT count(*) FROM tracks WHERE status = 'active'),
			(SELECT count(*) FROM courses WHERE status = 'published'), (SELECT count(*) FROM courses WHERE status = 'draft'),
			(SELECT count(*) FROM class_sessions WHERE cancelled_at IS NULL AND starts_at >= date_trunc('week', now()) AND starts_at < date_trunc('week', now()) + interval '7 days'),
			(SELECT count(*) FROM enrollments WHERE status IN ('active', 'completed')),
			(SELECT count(*) FROM certificates WHERE status = 'issued'),
			(SELECT count(*) FROM audit_logs WHERE created_at > now() - interval '24 hours')`).
		Scan(&users, &inactive, &tracks, &activeTracks, &published, &drafts, &sessionsWeek, &enrollments, &certificates, &audit24h)
	if err != nil {
		return nil, err
	}
	sessions, err := s.nextSessions(ctx, v, 5)
	if err != nil {
		return nil, err
	}
	unread, err := s.unread(ctx, v.UserID)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"role": identity.RoleAdmin, "activeUsers": users, "inactiveUsers": inactive, "usersByRole": usersByRole,
		"tracks": tracks, "activeTracks": activeTracks, "publishedCourses": published, "draftCourses": drafts,
		"sessionsThisWeek": sessionsWeek, "enrollments": enrollments, "certificatesIssued": certificates,
		"auditEventsLast24h": audit24h, "upcomingSessions": sessions, "unreadNotifications": unread,
	}, nil
}

var ErrForbidden = errors.New("forbidden")

func (s *Service) For(ctx context.Context, v learning.Viewer, role string) (gin.H, error) {
	if role == "" {
		role = identity.PrimaryRole(v.Roles)
	}
	if !v.Has(role) {
		return nil, ErrForbidden
	}
	switch role {
	case identity.RoleAdmin:
		return s.Admin(ctx, v)
	case identity.RoleInstructor:
		return s.Instructor(ctx, v)
	case identity.RoleMentor, identity.RoleTracker:
		return s.Staff(ctx, v, role)
	case identity.RoleStudent:
		return s.Student(ctx, v)
	}
	return nil, ErrForbidden
}

func Register(g *gin.RouterGroup, auth gin.HandlerFunc, svc *Service) {
	g.GET("/dashboard", auth, func(c *gin.Context) {
		role := c.Query("role")
		if role != "" && !identity.IsValidRole(role) {
			httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"role": "must be a known role"})
			return
		}
		data, err := svc.For(c.Request.Context(), learning.NewViewer(identity.CurrentPrincipal(c), c.ClientIP()), role)
		if errors.Is(err, ErrForbidden) {
			identity.Forbidden(c)
			return
		}
		if err != nil {
			httpx.Internal(c, err)
			return
		}
		httpx.Data(c, http.StatusOK, data)
	})
}
