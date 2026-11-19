package certificates

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/communication"
	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/performance"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrForbidden     = errors.New("forbidden")
	ErrNotEligible   = errors.New("student is not eligible for a certificate")
	ErrAlreadyIssued = errors.New("an active certificate already exists for this student and course")
	ErrNotEnrolled   = errors.New("student is not enrolled in this course")
	ErrBadState      = errors.New("certificate is not in a state that allows this action")
)

const (
	StatusIssued  = "issued"
	StatusRevoked = "revoked"

	ReasonLessons    = "lessons_incomplete"
	ReasonGrade      = "grade_below_minimum"
	ReasonNoGrade    = "no_grade_yet"
	ReasonAttendance = "attendance_below_minimum"
	ReasonProbation  = "on_probation"
)

type Requirements struct {
	CourseID          string `json:"courseId"`
	MinGrade          int    `json:"minGrade"`
	MinAttendance     int    `json:"minAttendance"`
	RequireAllLessons bool   `json:"requireAllLessons"`
	AllowProbation    bool   `json:"allowProbation"`
	IsDefault         bool   `json:"isDefault"`
}

func DefaultRequirements(courseID string) Requirements {
	return Requirements{CourseID: courseID, MinGrade: 60, MinAttendance: 75, RequireAllLessons: true, IsDefault: true}
}

type Eligibility struct {
	StudentID        string   `json:"studentId"`
	StudentName      string   `json:"studentName"`
	CourseID         string   `json:"courseId"`
	Eligible         bool     `json:"eligible"`
	Reasons          []string `json:"reasons"`
	Overall          *float64 `json:"overall"`
	Attendance       *float64 `json:"attendance"`
	Standing         string   `json:"standing"`
	CompletedLessons int      `json:"completedLessons"`
	TotalLessons     int      `json:"totalLessons"`
	CertificateID    *string  `json:"certificateId"`
}

func Evaluate(req Requirements, r performance.Result, completed, total int) (bool, []string) {
	reasons := []string{}
	if req.RequireAllLessons && completed < total {
		reasons = append(reasons, ReasonLessons)
	}
	switch {
	case r.Overall == nil:
		reasons = append(reasons, ReasonNoGrade)
	case *r.Overall < float64(req.MinGrade):
		reasons = append(reasons, ReasonGrade)
	}
	if r.Attendance != nil && *r.Attendance < float64(req.MinAttendance) {
		reasons = append(reasons, ReasonAttendance)
	}
	if !req.AllowProbation && r.Standing == performance.StandingProbation {
		reasons = append(reasons, ReasonProbation)
	}
	return len(reasons) == 0, reasons
}

type Certificate struct {
	ID               string     `json:"id"`
	StudentID        string     `json:"studentId"`
	StudentName      string     `json:"studentName"`
	CourseID         string     `json:"courseId"`
	CourseName       string     `json:"courseName"`
	VerificationCode string     `json:"verificationCode"`
	Status           string     `json:"status"`
	Grade            *float64   `json:"grade"`
	IssueDate        string     `json:"issueDate"`
	IssuedAt         time.Time  `json:"issuedAt"`
	RevokedAt        *time.Time `json:"revokedAt"`
	RevokeReason     *string    `json:"revokeReason,omitempty"`
}

type PublicCertificate struct {
	VerificationCode string     `json:"verificationCode"`
	Status           string     `json:"status"`
	StudentName      string     `json:"studentName"`
	CourseName       string     `json:"courseName"`
	IssueDate        string     `json:"issueDate"`
	RevokedAt        *time.Time `json:"revokedAt"`
}

type Event struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	ActorID   *string   `json:"actorId"`
	ActorName *string   `json:"actorName"`
	Reason    *string   `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

type Service struct {
	db          *pgxpool.Pool
	learning    *learning.Service
	performance *performance.Service
	loc         *time.Location
}

func NewService(db *pgxpool.Pool, l *learning.Service, p *performance.Service, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{db: db, learning: l, performance: p, loc: loc}
}

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func NewCode(now time.Time) (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(codeAlphabet)))
	for i := 0; i < 8; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(codeAlphabet[n.Int64()])
	}
	return fmt.Sprintf("EF-%04d-%s", now.Year(), b.String()), nil
}

func (s *Service) Requirements(ctx context.Context, courseID string) (Requirements, error) {
	r := Requirements{CourseID: courseID}
	err := s.db.QueryRow(ctx, `SELECT min_grade, min_attendance, require_all_lessons, allow_probation FROM certificate_requirements WHERE course_id = $1`, courseID).
		Scan(&r.MinGrade, &r.MinAttendance, &r.RequireAllLessons, &r.AllowProbation)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultRequirements(courseID), nil
	}
	return r, err
}

func (s *Service) GetRequirements(ctx context.Context, v learning.Viewer, courseID string) (Requirements, error) {
	if _, err := s.learning.Course(ctx, v, courseID); err != nil {
		return Requirements{}, err
	}
	return s.Requirements(ctx, courseID)
}

func (s *Service) SetRequirements(ctx context.Context, v learning.Viewer, r Requirements) (Requirements, error) {
	if !v.IsAdmin {
		return Requirements{}, ErrForbidden
	}
	if _, err := s.learning.Course(ctx, v, r.CourseID); err != nil {
		return Requirements{}, err
	}
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO certificate_requirements (course_id, min_grade, min_attendance, require_all_lessons, allow_probation, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (course_id) DO UPDATE SET min_grade = EXCLUDED.min_grade, min_attendance = EXCLUDED.min_attendance,
				require_all_lessons = EXCLUDED.require_all_lessons, allow_probation = EXCLUDED.allow_probation,
				updated_by = EXCLUDED.updated_by, updated_at = now()`,
			r.CourseID, r.MinGrade, r.MinAttendance, r.RequireAllLessons, r.AllowProbation, v.UserID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: "certificate_requirements.updated", TargetType: "course", TargetID: r.CourseID,
			Context: map[string]any{"minGrade": r.MinGrade, "minAttendance": r.MinAttendance, "requireAllLessons": r.RequireAllLessons, "allowProbation": r.AllowProbation}, IP: v.IP})
	})
	if err != nil {
		return Requirements{}, err
	}
	return s.Requirements(ctx, r.CourseID)
}

func (s *Service) eligibility(ctx context.Context, courseID, studentID string) ([]Eligibility, error) {
	req, err := s.Requirements(ctx, courseID)
	if err != nil {
		return nil, err
	}
	results, err := s.performance.CourseResults(ctx, courseID, studentID)
	if err != nil {
		return nil, err
	}
	var student any
	if studentID != "" {
		student = studentID
	}
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.name,
			(SELECT count(*) FROM lesson_progress lp JOIN lessons l ON l.id = lp.lesson_id JOIN modules m ON m.id = l.module_id
			 WHERE m.course_id = e.course_id AND lp.student_id = u.id AND lp.completed_at IS NOT NULL),
			(SELECT count(*) FROM lessons l JOIN modules m ON m.id = l.module_id WHERE m.course_id = e.course_id),
			(SELECT c.id FROM certificates c WHERE c.student_id = u.id AND c.course_id = e.course_id AND c.status = 'issued')
		FROM enrollments e JOIN users u ON u.id = e.student_id
		WHERE e.course_id = $1 AND e.status IN ('active', 'completed') AND ($2::uuid IS NULL OR e.student_id = $2)
		ORDER BY u.name, u.id`, courseID, student)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Eligibility{}
	for rows.Next() {
		var e Eligibility
		if err := rows.Scan(&e.StudentID, &e.StudentName, &e.CompletedLessons, &e.TotalLessons, &e.CertificateID); err != nil {
			return nil, err
		}
		e.CourseID = courseID
		r := results[e.StudentID]
		e.Overall, e.Attendance, e.Standing = r.Overall, r.Attendance, r.Standing
		e.Eligible, e.Reasons = Evaluate(req, r, e.CompletedLessons, e.TotalLessons)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) staffOf(ctx context.Context, v learning.Viewer, courseID string) error {
	err := s.learning.CheckManage(ctx, v, courseID)
	if err == nil || !errors.Is(err, learning.ErrForbidden) {
		return err
	}
	if v.Has(identity.RoleMentor) || v.Has(identity.RoleTracker) {
		return nil
	}
	return ErrForbidden
}

func (s *Service) CourseEligibility(ctx context.Context, v learning.Viewer, courseID string) ([]Eligibility, error) {
	if err := s.staffOf(ctx, v, courseID); err != nil {
		return nil, err
	}
	return s.eligibility(ctx, courseID, "")
}

func (s *Service) StudentEligibility(ctx context.Context, v learning.Viewer, courseID, studentID string) (*Eligibility, error) {
	if studentID != v.UserID {
		if err := s.staffOf(ctx, v, courseID); err != nil {
			return nil, err
		}
	} else if _, err := s.learning.Course(ctx, v, courseID); err != nil {
		return nil, err
	}
	list, err := s.eligibility(ctx, courseID, studentID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotEnrolled
	}
	return &list[0], nil
}

func (s *Service) issue(ctx context.Context, tx pgx.Tx, v learning.Viewer, e Eligibility, courseTitle string) (string, error) {
	var id string
	for attempt := 0; attempt < 5; attempt++ {
		code, err := NewCode(time.Now().In(s.loc))
		if err != nil {
			return "", err
		}
		_, err = tx.Exec(ctx, `SAVEPOINT cert_issue`)
		if err != nil {
			return "", err
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO certificates (student_id, course_id, verification_code, student_name, course_title, final_grade, issued_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			e.StudentID, e.CourseID, code, e.StudentName, courseTitle, e.Overall, v.UserID).Scan(&id)
		if database.IsUniqueViolation(err, "certificates_one_active") {
			_, _ = tx.Exec(ctx, `ROLLBACK TO SAVEPOINT cert_issue`)
			return "", ErrAlreadyIssued
		}
		if database.IsUniqueViolation(err, "certificates_verification_code_key") {
			if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT cert_issue`); err != nil {
				return "", err
			}
			continue
		}
		if err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO certificate_events (certificate_id, action, actor_id) VALUES ($1, 'issued', $2)`, id, v.UserID); err != nil {
			return "", err
		}
		if err := communication.NotifyUsers(ctx, tx, []string{e.StudentID}, communication.Message{
			Type: communication.TypeCertificate, Title: "Your certificate is ready", Message: courseTitle, Link: "/certificates", Key: "certificate:issued:" + id,
		}); err != nil {
			return "", err
		}
		return id, audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: "certificate.issued", TargetType: "certificate", TargetID: id,
			Context: map[string]any{"courseId": e.CourseID, "studentId": e.StudentID, "code": code}, IP: v.IP})
	}
	return "", errors.New("could not generate a unique verification code")
}

type IssueResult struct {
	Issued  []Certificate `json:"issued"`
	Skipped []Eligibility `json:"skipped"`
}

func (s *Service) Issue(ctx context.Context, v learning.Viewer, courseID, studentID string) (*IssueResult, error) {
	if !v.IsAdmin {
		return nil, ErrForbidden
	}
	course, err := s.learning.Course(ctx, v, courseID)
	if err != nil {
		return nil, err
	}
	list, err := s.eligibility(ctx, courseID, studentID)
	if err != nil {
		return nil, err
	}
	if studentID != "" && len(list) == 0 {
		return nil, ErrNotEnrolled
	}
	if studentID != "" && list[0].CertificateID != nil {
		return nil, ErrAlreadyIssued
	}
	if studentID != "" && !list[0].Eligible {
		return &IssueResult{Skipped: list}, ErrNotEligible
	}
	result := &IssueResult{Issued: []Certificate{}, Skipped: []Eligibility{}}
	var ids []string
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		for _, e := range list {
			if !e.Eligible || e.CertificateID != nil {
				result.Skipped = append(result.Skipped, e)
				continue
			}
			id, err := s.issue(ctx, tx, v, e, course.Title)
			if errors.Is(err, ErrAlreadyIssued) {
				if studentID != "" {
					return err
				}
				result.Skipped = append(result.Skipped, e)
				continue
			}
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		c, err := s.get(ctx, id)
		if err != nil {
			return nil, err
		}
		result.Issued = append(result.Issued, *c)
	}
	return result, nil
}

const certSelect = `SELECT c.id, c.student_id, c.student_name, c.course_id, c.course_title, c.verification_code, c.status,
	c.final_grade::float8, c.issued_at, c.revoked_at, c.revoke_reason FROM certificates c`

func (s *Service) scan(row pgx.Row) (*Certificate, error) {
	var c Certificate
	err := row.Scan(&c.ID, &c.StudentID, &c.StudentName, &c.CourseID, &c.CourseName, &c.VerificationCode, &c.Status,
		&c.Grade, &c.IssuedAt, &c.RevokedAt, &c.RevokeReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.IssueDate = c.IssuedAt.In(s.loc).Format("2006-01-02")
	return &c, nil
}

func (s *Service) get(ctx context.Context, id string) (*Certificate, error) {
	return s.scan(s.db.QueryRow(ctx, certSelect+` WHERE c.id = $1`, id))
}

func (s *Service) canSee(ctx context.Context, v learning.Viewer, c *Certificate) (bool, error) {
	if v.IsAdmin || c.StudentID == v.UserID {
		return true, nil
	}
	err := s.staffOf(ctx, v, c.CourseID)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, ErrForbidden), errors.Is(err, learning.ErrNotFound), errors.Is(err, learning.ErrForbidden):
		return false, nil
	default:
		return false, err
	}
}

func (s *Service) Certificate(ctx context.Context, v learning.Viewer, id string) (*Certificate, error) {
	c, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	ok, err := s.canSee(ctx, v, c)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

type Filter struct {
	CourseID  string
	StudentID string
	Status    string
}

func (s *Service) List(ctx context.Context, v learning.Viewer, f Filter, page httpx.Page) ([]Certificate, int, error) {
	where := ` JOIN courses co ON co.id = c.course_id WHERE ($1 OR c.student_id = $2
		OR ('instructor' = ANY($3::text[]) AND co.instructor_id = $2)
		OR ('mentor' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_mentors tm WHERE tm.track_id = co.track_id AND tm.user_id = $2))
		OR ('tracker' = ANY($3::text[]) AND EXISTS (SELECT 1 FROM track_trackers tt WHERE tt.track_id = co.track_id AND tt.user_id = $2)))
		AND ($4::uuid IS NULL OR c.course_id = $4::uuid)
		AND ($5::uuid IS NULL OR c.student_id = $5::uuid)
		AND ($6 = '' OR c.status = $6)`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(f.CourseID), nullable(f.StudentID), f.Status}
	rows, err := s.db.Query(ctx, certSelect+where+` ORDER BY c.issued_at DESC, c.id LIMIT $7 OFFSET $8`, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Certificate, error) {
		c, err := s.scan(row)
		if err != nil {
			return Certificate{}, err
		}
		return *c, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM certificates c`+where, args...).Scan(&total)
	return list, total, err
}

func (s *Service) changeStatus(ctx context.Context, v learning.Viewer, id, to, reason string) (*Certificate, error) {
	if !v.IsAdmin {
		return nil, ErrForbidden
	}
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM certificates WHERE id = $1 FOR UPDATE`, id).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		action := "revoked"
		if to == StatusRevoked {
			if status != StatusIssued {
				return ErrBadState
			}
			_, err = tx.Exec(ctx, `UPDATE certificates SET status = 'revoked', revoked_at = now(), revoked_by = $2, revoke_reason = $3 WHERE id = $1`, id, v.UserID, reason)
		} else {
			if status != StatusRevoked {
				return ErrBadState
			}
			action = "reinstated"
			_, err = tx.Exec(ctx, `UPDATE certificates SET status = 'issued', revoked_at = NULL, revoked_by = NULL, revoke_reason = NULL WHERE id = $1`, id)
			if database.IsUniqueViolation(err, "certificates_one_active") {
				return ErrAlreadyIssued
			}
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO certificate_events (certificate_id, action, actor_id, reason) VALUES ($1, $2, $3, $4)`, id, action, v.UserID, reason); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: "certificate." + action, TargetType: "certificate", TargetID: id,
			Context: map[string]any{"reason": reason}, IP: v.IP})
	})
	if err != nil {
		return nil, err
	}
	return s.get(ctx, id)
}

func (s *Service) Revoke(ctx context.Context, v learning.Viewer, id, reason string) (*Certificate, error) {
	return s.changeStatus(ctx, v, id, StatusRevoked, reason)
}

func (s *Service) Reinstate(ctx context.Context, v learning.Viewer, id, reason string) (*Certificate, error) {
	return s.changeStatus(ctx, v, id, StatusIssued, reason)
}

func (s *Service) Events(ctx context.Context, v learning.Viewer, id string) ([]Event, error) {
	if !v.IsAdmin {
		return nil, ErrForbidden
	}
	if _, err := s.get(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT e.id, e.action, e.actor_id, u.name, e.reason, e.created_at FROM certificate_events e
		LEFT JOIN users u ON u.id = e.actor_id WHERE e.certificate_id = $1 ORDER BY e.created_at, e.id`, id)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Event, error) {
		var e Event
		err := row.Scan(&e.ID, &e.Action, &e.ActorID, &e.ActorName, &e.Reason, &e.CreatedAt)
		return e, err
	})
}

func (s *Service) Verify(ctx context.Context, code string) (*PublicCertificate, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(code) != 16 {
		return nil, ErrNotFound
	}
	var p PublicCertificate
	var issued time.Time
	err := s.db.QueryRow(ctx, `SELECT verification_code, status, student_name, course_title, issued_at, revoked_at FROM certificates WHERE verification_code = $1`, code).
		Scan(&p.VerificationCode, &p.Status, &p.StudentName, &p.CourseName, &issued, &p.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.IssueDate = issued.In(s.loc).Format("2006-01-02")
	return &p, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
