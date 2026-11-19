package performance

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

type ItemSource func(ctx context.Context, db *pgxpool.Pool, courseID, studentID string) (map[string][]Item, error)

type Service struct {
	db       *pgxpool.Pool
	learning *learning.Service
	projects ItemSource
}

func NewService(db *pgxpool.Pool, l *learning.Service) *Service {
	return &Service{db: db, learning: l}
}

func (s *Service) SetProjectSource(src ItemSource) {
	s.projects = src
}

const policyColumns = `id, course_id, version, quiz_weight, assignment_weight, attendance_weight, project_weight,
	warning_grade, probation_grade, warning_attendance, probation_attendance, warning_missing, reason`

func scanPolicy(row pgx.Row) (Policy, error) {
	var p Policy
	var id string
	err := row.Scan(&id, &p.CourseID, &p.Version, &p.QuizWeight, &p.AssignmentWeight, &p.AttendanceWeight, &p.ProjectWeight,
		&p.WarningGrade, &p.ProbationGrade, &p.WarningAttendance, &p.ProbationAttendance, &p.WarningMissing, &p.Reason)
	p.ID = &id
	return p, err
}

func (s *Service) currentPolicy(ctx context.Context, courseID string) (Policy, error) {
	p, err := scanPolicy(s.db.QueryRow(ctx, `SELECT `+policyColumns+` FROM grade_policies WHERE course_id = $1 ORDER BY version DESC LIMIT 1`, courseID))
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultPolicy(courseID), nil
	}
	return p, err
}

func (s *Service) Policy(ctx context.Context, v learning.Viewer, courseID string) (Policy, error) {
	if _, err := s.learning.Course(ctx, v, courseID); err != nil {
		return Policy{}, err
	}
	return s.currentPolicy(ctx, courseID)
}

func (s *Service) PolicyVersions(ctx context.Context, v learning.Viewer, courseID string) ([]Policy, error) {
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT `+policyColumns+` FROM grade_policies WHERE course_id = $1 ORDER BY version DESC LIMIT 100`, courseID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Policy, error) { return scanPolicy(row) })
}

func (s *Service) SetPolicy(ctx context.Context, v learning.Viewer, courseID string, p Policy) (Policy, error) {
	if err := p.Validate(); err != nil {
		return Policy{}, err
	}
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return Policy{}, err
	}
	var out Policy
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM courses WHERE id = $1 FOR UPDATE`, courseID); err != nil {
			return err
		}
		var err error
		out, err = scanPolicy(tx.QueryRow(ctx, `
			INSERT INTO grade_policies (course_id, version, quiz_weight, assignment_weight, attendance_weight, project_weight,
				warning_grade, probation_grade, warning_attendance, probation_attendance, warning_missing, reason, created_by)
			SELECT $1, COALESCE(max(version), 0) + 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''), $12
			FROM grade_policies WHERE course_id = $1
			RETURNING `+policyColumns,
			courseID, p.QuizWeight, p.AssignmentWeight, p.AttendanceWeight, p.ProjectWeight, p.WarningGrade, p.ProbationGrade,
			p.WarningAttendance, p.ProbationAttendance, p.WarningMissing, strings.TrimSpace(deref(p.Reason)), v.UserID))
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: "grade_policy.updated", TargetType: "course", TargetID: courseID,
			Context: map[string]any{"version": out.Version, "weights": []int{p.QuizWeight, p.AssignmentWeight, p.AttendanceWeight, p.ProjectWeight}}, IP: v.IP})
	})
	return out, err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Service) inputs(ctx context.Context, courseID, studentID string) (map[string]*Inputs, error) {
	var student any
	if studentID != "" {
		student = studentID
	}
	out := map[string]*Inputs{}
	rows, err := s.db.Query(ctx, `
		SELECT e.student_id FROM enrollments e
		WHERE e.course_id = $1 AND e.status IN ('active', 'completed') AND ($2::uuid IS NULL OR e.student_id = $2)`, courseID, student)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = &Inputs{}
	}

	rows, err = s.db.Query(ctx, `
		SELECT e.student_id,
			(SELECT max(a.percentage)::float8 FROM quiz_attempts a WHERE a.quiz_id = q.id AND a.student_id = e.student_id AND a.status <> 'in_progress'),
			q.due_at IS NOT NULL AND q.due_at <= now()
		FROM enrollments e JOIN quizzes q ON q.course_id = e.course_id AND q.status = 'published'
		WHERE e.course_id = $1 AND e.status IN ('active', 'completed') AND ($2::uuid IS NULL OR e.student_id = $2)`, courseID, student)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		var best *float64
		var closed bool
		if err := rows.Scan(&sid, &best, &closed); err != nil {
			return nil, err
		}
		in := out[sid]
		switch {
		case best != nil:
			in.Quizzes = append(in.Quizzes, Item{Percent: best})
		case closed:
			in.Quizzes = append(in.Quizzes, Item{Missing: true})
		default:
			in.Quizzes = append(in.Quizzes, Item{Pending: true})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.Query(ctx, `
		SELECT e.student_id, s.status, (s.final_grade / a.max_grade * 100)::float8,
			now() > a.due_at AND (a.late_policy = 'reject' OR (a.late_until IS NOT NULL AND now() > a.late_until))
		FROM enrollments e
		JOIN assignments a ON a.course_id = e.course_id AND a.status = 'published'
		LEFT JOIN assignment_submissions s ON s.assignment_id = a.id AND s.student_id = e.student_id AND s.status <> 'superseded'
		WHERE e.course_id = $1 AND e.status IN ('active', 'completed') AND ($2::uuid IS NULL OR e.student_id = $2)`, courseID, student)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		var status *string
		var pctv *float64
		var closed bool
		if err := rows.Scan(&sid, &status, &pctv, &closed); err != nil {
			return nil, err
		}
		in := out[sid]
		switch {
		case pctv != nil:
			in.Assignments = append(in.Assignments, Item{Percent: pctv})
		case status != nil:
			in.Assignments = append(in.Assignments, Item{Pending: true})
		case closed:
			in.Assignments = append(in.Assignments, Item{Missing: true})
		default:
			in.Assignments = append(in.Assignments, Item{Pending: true})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.Query(ctx, `
		SELECT r.student_id,
			count(*) FILTER (WHERE r.status IN ('present', 'late')),
			count(*) FILTER (WHERE r.status <> 'excused')
		FROM attendance_records r
		JOIN attendance_sessions w ON w.id = r.attendance_session_id AND w.status = 'closed'
		JOIN class_sessions cs ON cs.id = w.session_id
		WHERE cs.course_id = $1 AND ($2::uuid IS NULL OR r.student_id = $2)
		GROUP BY r.student_id`, courseID, student)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		var attended, counted int
		if err := rows.Scan(&sid, &attended, &counted); err != nil {
			return nil, err
		}
		if in, ok := out[sid]; ok && counted > 0 {
			v := float64(attended) * 100 / float64(counted)
			in.Attendance = &v
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if s.projects != nil {
		items, err := s.projects(ctx, s.db, courseID, studentID)
		if err != nil {
			return nil, err
		}
		for sid, list := range items {
			if in, ok := out[sid]; ok {
				in.Projects = list
			}
		}
	}
	return out, nil
}

type CourseGrade struct {
	CourseID      string `json:"courseId"`
	CourseName    string `json:"courseName"`
	PolicyVersion int    `json:"policyVersion"`
	Result
}

type StudentGrades struct {
	StudentID string        `json:"studentId"`
	Courses   []CourseGrade `json:"courses"`
	GPA       *float64      `json:"gpa"`
	Standing  string        `json:"standing"`
}

type StudentStanding struct {
	StudentID   string `json:"studentId"`
	StudentName string `json:"studentName"`
	CourseID    string `json:"courseId"`
	CourseName  string `json:"courseName"`
	Result
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

func (s *Service) StudentGrades(ctx context.Context, v learning.Viewer, studentID string) (*StudentGrades, error) {
	if err := s.canViewStudent(ctx, v, studentID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id, c.title FROM enrollments e JOIN courses c ON c.id = e.course_id
		WHERE e.student_id = $4 AND e.status IN ('active', 'completed') AND `+learning.CourseVisibility+`
		ORDER BY c.title, c.id LIMIT 200`, v.IsAdmin, v.UserID, v.Roles, studentID)
	if err != nil {
		return nil, err
	}
	type course struct{ id, name string }
	courses, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (course, error) {
		var c course
		err := row.Scan(&c.id, &c.name)
		return c, err
	})
	if err != nil {
		return nil, err
	}

	out := &StudentGrades{StudentID: studentID, Courses: []CourseGrade{}, Standing: StandingGood}
	var sum float64
	var n int
	for _, c := range courses {
		policy, err := s.currentPolicy(ctx, c.id)
		if err != nil {
			return nil, err
		}
		inputs, err := s.inputs(ctx, c.id, studentID)
		if err != nil {
			return nil, err
		}
		in := inputs[studentID]
		if in == nil {
			continue
		}
		r := Compute(policy, *in)
		out.Courses = append(out.Courses, CourseGrade{CourseID: c.id, CourseName: c.name, PolicyVersion: policy.Version, Result: r})
		out.Standing = worst(out.Standing, r.Standing)
		if r.Overall != nil {
			sum += *r.Overall
			n++
		}
	}
	if n > 0 {
		g := round2(sum / float64(n))
		out.GPA = &g
	}
	return out, nil
}

func (s *Service) courseStanding(ctx context.Context, courseID string) ([]StudentStanding, Policy, error) {
	policy, err := s.currentPolicy(ctx, courseID)
	if err != nil {
		return nil, policy, err
	}
	inputs, err := s.inputs(ctx, courseID, "")
	if err != nil {
		return nil, policy, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT u.id, u.name, c.title FROM enrollments e JOIN users u ON u.id = e.student_id JOIN courses c ON c.id = e.course_id
		WHERE e.course_id = $1 AND e.status IN ('active', 'completed')`, courseID)
	if err != nil {
		return nil, policy, err
	}
	defer rows.Close()
	var out []StudentStanding
	for rows.Next() {
		var st StudentStanding
		if err := rows.Scan(&st.StudentID, &st.StudentName, &st.CourseName); err != nil {
			return nil, policy, err
		}
		st.CourseID = courseID
		in := inputs[st.StudentID]
		if in == nil {
			in = &Inputs{}
		}
		st.Result = Compute(policy, *in)
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Standing != out[j].Standing {
			return worst(out[i].Standing, out[j].Standing) == out[i].Standing
		}
		return out[i].StudentName < out[j].StudentName
	})
	return out, policy, rows.Err()
}

func (s *Service) staffForCourse(ctx context.Context, v learning.Viewer, courseID string) error {
	err := s.learning.CheckManage(ctx, v, courseID)
	if err == nil || !errors.Is(err, learning.ErrForbidden) {
		return err
	}
	if v.Has(identity.RoleMentor) || v.Has(identity.RoleTracker) {
		return nil
	}
	return ErrForbidden
}

func filterStanding(list []StudentStanding, standing string) []StudentStanding {
	if standing == "" {
		return list
	}
	out := []StudentStanding{}
	for _, s := range list {
		if (standing == "at_risk" && s.Standing != StandingGood) || s.Standing == standing {
			out = append(out, s)
		}
	}
	return out
}

func (s *Service) CourseStanding(ctx context.Context, v learning.Viewer, courseID, standing string) ([]StudentStanding, error) {
	if err := s.staffForCourse(ctx, v, courseID); err != nil {
		return nil, err
	}
	list, _, err := s.courseStanding(ctx, courseID)
	return filterStanding(list, standing), err
}

func (s *Service) AtRisk(ctx context.Context, v learning.Viewer) ([]StudentStanding, error) {
	if !v.IsAdmin && !v.Has(identity.RoleInstructor) && !v.Has(identity.RoleMentor) && !v.Has(identity.RoleTracker) {
		return nil, ErrForbidden
	}
	rows, err := s.db.Query(ctx, `SELECT c.id FROM courses c WHERE c.status <> 'archived' AND `+learning.CourseVisibility+` AND ($1
		OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2)
		OR EXISTS (SELECT 1 FROM track_mentors tm WHERE tm.track_id = c.track_id AND tm.user_id = $2)
		OR EXISTS (SELECT 1 FROM track_trackers tt WHERE tt.track_id = c.track_id AND tt.user_id = $2))
		ORDER BY c.title LIMIT 200`, v.IsAdmin, v.UserID, v.Roles)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := []StudentStanding{}
	for _, id := range ids {
		list, _, err := s.courseStanding(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, filterStanding(list, "at_risk")...)
	}
	return out, nil
}

func (s *Service) Snapshot(ctx context.Context, v learning.Viewer, courseID string) (int, error) {
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return 0, err
	}
	list, policy, err := s.courseStanding(ctx, courseID)
	if err != nil {
		return 0, err
	}
	err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		for _, st := range list {
			components, _ := json.Marshal(map[string]any{"quizAvg": st.QuizAvg, "assignmentAvg": st.AssignmentAvg,
				"projectGrade": st.ProjectGrade, "attendance": st.Attendance, "missingWork": st.Missing, "pendingWork": st.Pending})
			factors, _ := json.Marshal(st.Factors)
			if _, err := tx.Exec(ctx, `
				INSERT INTO standing_snapshots (student_id, course_id, policy_id, overall, standing, components, factors, computed_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				st.StudentID, courseID, policy.ID, st.Overall, st.Standing, components, factors, v.UserID); err != nil {
				return err
			}
		}
		return nil
	})
	return len(list), err
}
