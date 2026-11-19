package assessment

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type Quiz struct {
	ID             string     `json:"id"`
	CourseID       string     `json:"courseId"`
	CourseTitle    string     `json:"courseTitle"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	TimeLimit      *int       `json:"timeLimit"`
	OpensAt        *time.Time `json:"opensAt"`
	DueAt          *time.Time `json:"dueAt"`
	DueDate        *string    `json:"dueDate"`
	MaxAttempts    int        `json:"maxAttempts"`
	PassPercent    int        `json:"passPercent"`
	RevealAnswers  string     `json:"revealAnswers"`
	TotalQuestions int        `json:"totalQuestions"`
	TotalPoints    int        `json:"totalPoints"`
	AttemptsUsed   *int       `json:"attemptsUsed,omitempty"`
	BestPercentage *int       `json:"bestPercentage,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	published      bool
}

type Option struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Order     int    `json:"order"`
	IsCorrect *bool  `json:"isCorrect,omitempty"`
}

type Question struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Points  int      `json:"points"`
	Order   int      `json:"order"`
	Options []Option `json:"options"`
}

type Answer struct {
	QuestionID      string  `json:"questionId"`
	OptionID        string  `json:"optionId"`
	CorrectOptionID *string `json:"correctOptionId,omitempty"`
	Correct         *bool   `json:"correct,omitempty"`
}

type Attempt struct {
	ID            string     `json:"id"`
	QuizID        string     `json:"quizId"`
	StudentID     string     `json:"studentId"`
	StudentName   string     `json:"studentName"`
	AttemptNumber int        `json:"attemptNumber"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"startedAt"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	SubmittedAt   *time.Time `json:"submittedAt"`
	Score         *int       `json:"score"`
	Total         *int       `json:"total"`
	Percentage    *int       `json:"percentage"`
	Passed        *bool      `json:"passed"`
	Answers       []Answer   `json:"answers"`
	Questions     []Question `json:"questions,omitempty"`
	courseID      string
}

type QuizInput struct {
	Title         *string
	Description   *string
	TimeLimit     *int
	ClearLimit    bool
	OpensAt       *time.Time
	DueAt         *time.Time
	MaxAttempts   *int
	PassPercent   *int
	RevealAnswers *string
	Status        *string
}

type QuestionInput struct {
	Text    string
	Points  int
	Options []OptionInput
}

type OptionInput struct {
	Text    string
	Correct bool
}

const quizSelect = `
	SELECT q.id, q.course_id, c.title, q.title, q.description, q.status = 'published', q.time_limit_minutes,
		q.opens_at, q.due_at, q.max_attempts, q.pass_percent, q.reveal_answers, q.created_at, q.updated_at,
		(SELECT count(*) FROM quiz_questions qq WHERE qq.quiz_id = q.id),
		(SELECT COALESCE(sum(points), 0) FROM quiz_questions qq WHERE qq.quiz_id = q.id)
	FROM quizzes q JOIN courses c ON c.id = q.course_id`

const quizVisible = learning.CourseVisibility + `
	AND (q.status = 'published' OR $1 OR ('instructor' = ANY($3::text[]) AND c.instructor_id = $2))`

func (s *Service) scanQuiz(row pgx.Row) (*Quiz, error) {
	var q Quiz
	err := row.Scan(&q.ID, &q.CourseID, &q.CourseTitle, &q.Title, &q.Description, &q.published, &q.TimeLimit,
		&q.OpensAt, &q.DueAt, &q.MaxAttempts, &q.PassPercent, &q.RevealAnswers, &q.CreatedAt, &q.UpdatedAt,
		&q.TotalQuestions, &q.TotalPoints)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	q.Status = quizStatus(q.published, q.OpensAt, q.DueAt, s.now())
	q.DueDate = s.dateLabel(q.DueAt)
	return &q, nil
}

func (s *Service) withStudentStats(ctx context.Context, v learning.Viewer, q *Quiz) error {
	if !v.Has(identity.RoleStudent) {
		return nil
	}
	var used int
	var best *int
	if err := s.db.QueryRow(ctx, `
		SELECT count(*), max(percentage) FROM quiz_attempts WHERE quiz_id = $1 AND student_id = $2`,
		q.ID, v.UserID).Scan(&used, &best); err != nil {
		return err
	}
	q.AttemptsUsed = &used
	q.BestPercentage = best
	return nil
}

func (s *Service) ListQuizzes(ctx context.Context, v learning.Viewer, courseID string, page httpx.Page) ([]Quiz, int, error) {
	where := ` WHERE ` + quizVisible + ` AND ($4::uuid IS NULL OR q.course_id = $4::uuid)`
	args := []any{v.IsAdmin, v.UserID, v.Roles, nullable(courseID)}
	rows, err := s.db.Query(ctx, quizSelect+where+` ORDER BY q.due_at NULLS LAST, q.created_at, q.id LIMIT $5 OFFSET $6`,
		append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Quiz, error) {
		q, err := s.scanQuiz(row)
		if err != nil {
			return Quiz{}, err
		}
		return *q, nil
	})
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		if err := s.withStudentStats(ctx, v, &list[i]); err != nil {
			return nil, 0, err
		}
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM quizzes q JOIN courses c ON c.id = q.course_id`+where, args...).Scan(&total)
	return list, total, err
}

func (s *Service) Quiz(ctx context.Context, v learning.Viewer, id string) (*Quiz, error) {
	q, err := s.scanQuiz(s.db.QueryRow(ctx, quizSelect+` WHERE q.id = $4 AND `+quizVisible, v.IsAdmin, v.UserID, v.Roles, id))
	if err != nil {
		return nil, err
	}
	return q, s.withStudentStats(ctx, v, q)
}

func (s *Service) quizCourse(ctx context.Context, db database.DBTX, id string, lock bool) (string, bool, error) {
	q := `SELECT course_id, status = 'published' FROM quizzes WHERE id = $1`
	if lock {
		q += ` FOR UPDATE`
	}
	var courseID string
	var published bool
	err := db.QueryRow(ctx, q, id).Scan(&courseID, &published)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, ErrNotFound
	}
	return courseID, published, err
}

func (s *Service) authorizeQuiz(ctx context.Context, v learning.Viewer, id string) (string, error) {
	courseID, _, err := s.quizCourse(ctx, s.db, id, false)
	if err != nil {
		return "", err
	}
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		if errors.Is(err, learning.ErrForbidden) {
			if _, verr := s.Quiz(ctx, v, id); errors.Is(verr, ErrNotFound) {
				return "", ErrNotFound
			}
		}
		return "", err
	}
	return courseID, nil
}

func (s *Service) CreateQuiz(ctx context.Context, v learning.Viewer, courseID string, in QuizInput) (*Quiz, error) {
	if err := s.learning.CheckManage(ctx, v, courseID); err != nil {
		return nil, err
	}
	if in.OpensAt != nil && in.DueAt != nil && !in.DueAt.After(*in.OpensAt) {
		return nil, ErrInvalidSchedule
	}
	var id string
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO quizzes (course_id, title, description, time_limit_minutes, opens_at, due_at, max_attempts,
				pass_percent, reveal_answers, created_by)
			VALUES ($1, $2, COALESCE($3, ''), $4, $5, $6, COALESCE($7, 1), COALESCE($8, 50), COALESCE($9, 'after_close'), $10)
			RETURNING id`,
			courseID, strings.TrimSpace(*in.Title), in.Description, in.TimeLimit, in.OpensAt, in.DueAt, in.MaxAttempts,
			in.PassPercent, in.RevealAnswers, v.UserID).Scan(&id)
		if err != nil {
			return err
		}
		return record(ctx, tx, v, "quiz.created", "quiz", id, map[string]any{"courseId": courseID})
	})
	if err != nil {
		return nil, err
	}
	return s.Quiz(ctx, v, id)
}

func (s *Service) UpdateQuiz(ctx context.Context, v learning.Viewer, id string, in QuizInput) (*Quiz, error) {
	if _, err := s.authorizeQuiz(ctx, v, id); err != nil {
		return nil, err
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		in.Title = &t
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, _, err := s.quizCourse(ctx, tx, id, true); err != nil {
			return err
		}
		if in.Status != nil && *in.Status == "published" {
			if err := s.validateQuestions(ctx, tx, id); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE quizzes SET
				title = COALESCE($2, title),
				description = COALESCE($3, description),
				time_limit_minutes = CASE WHEN $4 THEN NULL ELSE COALESCE($5, time_limit_minutes) END,
				opens_at = COALESCE($6, opens_at),
				due_at = COALESCE($7, due_at),
				max_attempts = COALESCE($8, max_attempts),
				pass_percent = COALESCE($9, pass_percent),
				reveal_answers = COALESCE($10, reveal_answers),
				status = COALESCE($11, status)
			WHERE id = $1`, id, in.Title, in.Description, in.ClearLimit, in.TimeLimit, in.OpensAt, in.DueAt,
			in.MaxAttempts, in.PassPercent, in.RevealAnswers, in.Status)
		if database.IsCheckViolation(err) {
			return ErrInvalidSchedule
		}
		if err != nil {
			return err
		}
		return record(ctx, tx, v, "quiz.updated", "quiz", id, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.Quiz(ctx, v, id)
}

func (s *Service) validateQuestions(ctx context.Context, db database.DBTX, quizID string) error {
	var questions, invalid int
	err := db.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE opts < 2 OR correct <> 1) FROM (
			SELECT qq.id, count(o.id) AS opts, count(o.id) FILTER (WHERE o.is_correct) AS correct
			FROM quiz_questions qq LEFT JOIN quiz_options o ON o.question_id = qq.id
			WHERE qq.quiz_id = $1 GROUP BY qq.id) x`, quizID).Scan(&questions, &invalid)
	if err != nil {
		return err
	}
	if questions == 0 || invalid > 0 {
		return ErrQuizInvalid
	}
	return nil
}

func ValidateQuestionInput(qs []QuestionInput) error {
	if len(qs) == 0 {
		return ErrQuizInvalid
	}
	for _, q := range qs {
		correct := 0
		for _, o := range q.Options {
			if o.Correct {
				correct++
			}
		}
		if len(q.Options) < 2 || correct != 1 {
			return ErrQuizInvalid
		}
	}
	return nil
}

func (s *Service) ReplaceQuestions(ctx context.Context, v learning.Viewer, quizID string, qs []QuestionInput) ([]Question, error) {
	if err := ValidateQuestionInput(qs); err != nil {
		return nil, err
	}
	if _, err := s.authorizeQuiz(ctx, v, quizID); err != nil {
		return nil, err
	}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, _, err := s.quizCourse(ctx, tx, quizID, true); err != nil {
			return err
		}
		var attempts bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM quiz_attempts WHERE quiz_id = $1)`, quizID).Scan(&attempts); err != nil {
			return err
		}
		if attempts {
			return ErrQuizHasAttempts
		}
		if _, err := tx.Exec(ctx, `DELETE FROM quiz_options WHERE question_id IN (SELECT id FROM quiz_questions WHERE quiz_id = $1)`, quizID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM quiz_questions WHERE quiz_id = $1`, quizID); err != nil {
			return err
		}
		for i, q := range qs {
			points := q.Points
			if points == 0 {
				points = 1
			}
			var qid string
			if err := tx.QueryRow(ctx, `INSERT INTO quiz_questions (quiz_id, text, points, position) VALUES ($1, $2, $3, $4) RETURNING id`,
				quizID, strings.TrimSpace(q.Text), points, i+1).Scan(&qid); err != nil {
				return err
			}
			for j, o := range q.Options {
				if _, err := tx.Exec(ctx, `INSERT INTO quiz_options (question_id, text, is_correct, position) VALUES ($1, $2, $3, $4)`,
					qid, strings.TrimSpace(o.Text), o.Correct, j+1); err != nil {
					return err
				}
			}
		}
		return record(ctx, tx, v, "quiz.questions_replaced", "quiz", quizID, map[string]any{"questions": len(qs)})
	})
	if err != nil {
		return nil, err
	}
	return s.questions(ctx, s.db, quizID, true)
}

func (s *Service) Questions(ctx context.Context, v learning.Viewer, quizID string) ([]Question, error) {
	courseID, _, err := s.quizCourse(ctx, s.db, quizID, false)
	if err != nil {
		return nil, err
	}
	staff, err := s.isStaff(ctx, v, courseID)
	if err != nil {
		return nil, err
	}
	if !staff {
		if _, err := s.Quiz(ctx, v, quizID); err != nil {
			return nil, err
		}
		return nil, ErrForbidden
	}
	return s.questions(ctx, s.db, quizID, true)
}

func (s *Service) questions(ctx context.Context, db database.DBTX, quizID string, withAnswers bool) ([]Question, error) {
	rows, err := db.Query(ctx, `
		SELECT qq.id, qq.text, qq.points, qq.position, o.id, o.text, o.position, o.is_correct
		FROM quiz_questions qq JOIN quiz_options o ON o.question_id = qq.id
		WHERE qq.quiz_id = $1 ORDER BY qq.position, o.position`, quizID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Question{}
	for rows.Next() {
		var q Question
		var o Option
		var correct bool
		if err := rows.Scan(&q.ID, &q.Text, &q.Points, &q.Order, &o.ID, &o.Text, &o.Order, &correct); err != nil {
			return nil, err
		}
		if withAnswers {
			c := correct
			o.IsCorrect = &c
		}
		if len(out) == 0 || out[len(out)-1].ID != q.ID {
			q.Options = []Option{}
			out = append(out, q)
		}
		last := &out[len(out)-1]
		last.Options = append(last.Options, o)
	}
	return out, rows.Err()
}

const attemptSelect = `
	SELECT a.id, a.quiz_id, a.student_id, u.name, a.attempt_number, a.status, a.started_at, a.expires_at,
		a.submitted_at, a.score, a.max_score, a.percentage, a.passed, q.course_id
	FROM quiz_attempts a JOIN users u ON u.id = a.student_id JOIN quizzes q ON q.id = a.quiz_id`

func scanAttempt(row pgx.Row) (*Attempt, error) {
	var a Attempt
	err := row.Scan(&a.ID, &a.QuizID, &a.StudentID, &a.StudentName, &a.AttemptNumber, &a.Status, &a.StartedAt,
		&a.ExpiresAt, &a.SubmittedAt, &a.Score, &a.Total, &a.Percentage, &a.Passed, &a.courseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	a.Answers = []Answer{}
	return &a, err
}

func (s *Service) expireAttempt(ctx context.Context, db database.DBTX, id string) error {
	_, err := db.Exec(ctx, `
		UPDATE quiz_attempts SET status = 'expired', submitted_at = now(), score = 0,
			max_score = (SELECT COALESCE(sum(points), 0) FROM quiz_questions qq WHERE qq.quiz_id = quiz_attempts.quiz_id),
			percentage = 0, passed = false
		WHERE id = $1 AND status = 'in_progress'`, id)
	return err
}

func (s *Service) StartAttempt(ctx context.Context, v learning.Viewer, quizID string) (*Attempt, bool, error) {
	if !v.Has(identity.RoleStudent) {
		return nil, false, ErrForbidden
	}
	q, err := s.Quiz(ctx, v, quizID)
	if err != nil {
		return nil, false, err
	}
	enrolled, err := s.isEnrolledStudent(ctx, s.db, v.UserID, q.CourseID)
	if err != nil {
		return nil, false, err
	}
	if !enrolled {
		return nil, false, ErrNotEnrolled
	}
	now := s.now()
	var id string
	created := false
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`, quizID, v.UserID); err != nil {
			return err
		}
		var openID string
		var expires *time.Time
		err := tx.QueryRow(ctx, `SELECT id, expires_at FROM quiz_attempts WHERE quiz_id = $1 AND student_id = $2 AND status = 'in_progress'`,
			quizID, v.UserID).Scan(&openID, &expires)
		switch {
		case err == nil:
			if expires == nil || now.Before(expires.Add(SubmitGrace)) {
				id = openID
				return nil
			}
			if err := s.expireAttempt(ctx, tx, openID); err != nil {
				return err
			}
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		if quizStatus(q.published, q.OpensAt, q.DueAt, now) != QuizOpen {
			return ErrQuizNotOpen
		}
		var used int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM quiz_attempts WHERE quiz_id = $1 AND student_id = $2`, quizID, v.UserID).Scan(&used); err != nil {
			return err
		}
		if used >= q.MaxAttempts {
			return ErrAttemptLimit
		}
		deadline := attemptDeadline(now, q.TimeLimit, q.DueAt)
		created = true
		return tx.QueryRow(ctx, `
			INSERT INTO quiz_attempts (quiz_id, student_id, attempt_number, started_at, expires_at)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, quizID, v.UserID, used+1, now, deadline).Scan(&id)
	})
	if err != nil {
		return nil, false, err
	}
	a, err := scanAttempt(s.db.QueryRow(ctx, attemptSelect+` WHERE a.id = $1`, id))
	if err != nil {
		return nil, false, err
	}
	a.Questions, err = s.questions(ctx, s.db, quizID, false)
	return a, created, err
}

func (s *Service) SubmitAttempt(ctx context.Context, v learning.Viewer, attemptID string, answers map[string]string) (*Attempt, error) {
	now := s.now()
	expired := false
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		a, err := scanAttempt(tx.QueryRow(ctx, attemptSelect+` WHERE a.id = $1 FOR UPDATE OF a`, attemptID))
		if err != nil {
			return err
		}
		if a.StudentID != v.UserID {
			return ErrNotFound
		}
		if a.Status != AttemptInProgress {
			return ErrAttemptClosed
		}
		if a.ExpiresAt != nil && now.After(a.ExpiresAt.Add(SubmitGrace)) {
			expired = true
			return s.expireAttempt(ctx, tx, attemptID)
		}

		rows, err := tx.Query(ctx, `
			SELECT qq.id, qq.points, o.id FROM quiz_questions qq
			JOIN quiz_options o ON o.question_id = qq.id AND o.is_correct
			WHERE qq.quiz_id = $1`, a.QuizID)
		if err != nil {
			return err
		}
		qs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (scoredQuestion, error) {
			var q scoredQuestion
			err := row.Scan(&q.ID, &q.Points, &q.CorrectOption)
			return q, err
		})
		if err != nil {
			return err
		}
		known := map[string]bool{}
		for _, q := range qs {
			known[q.ID] = true
		}
		for questionID, optionID := range answers {
			if !known[questionID] {
				return ErrInvalidAnswer
			}
			if _, err := tx.Exec(ctx, `INSERT INTO quiz_answers (attempt_id, quiz_id, question_id, option_id) VALUES ($1, $2, $3, $4)`,
				attemptID, a.QuizID, questionID, optionID); err != nil {
				if database.IsForeignKeyViolation(err) {
					return ErrInvalidAnswer
				}
				return err
			}
		}
		var pass int
		if err := tx.QueryRow(ctx, `SELECT pass_percent FROM quizzes WHERE id = $1`, a.QuizID).Scan(&pass); err != nil {
			return err
		}
		r := scoreAttempt(qs, answers, pass)
		_, err = tx.Exec(ctx, `
			UPDATE quiz_attempts SET status = 'submitted', submitted_at = $2, score = $3, max_score = $4, percentage = $5, passed = $6
			WHERE id = $1`, attemptID, now, r.Score, r.MaxScore, r.Percentage, r.Passed)
		return err
	})
	if err != nil {
		return nil, err
	}
	if expired {
		return nil, ErrAttemptExpired
	}
	return s.Attempt(ctx, v, attemptID)
}

func (s *Service) Attempt(ctx context.Context, v learning.Viewer, id string) (*Attempt, error) {
	a, err := scanAttempt(s.db.QueryRow(ctx, attemptSelect+` WHERE a.id = $1`, id))
	if err != nil {
		return nil, err
	}
	staff := false
	if a.StudentID != v.UserID {
		staff, err = s.isStaff(ctx, v, a.courseID)
		if err != nil {
			return nil, err
		}
		if !staff {
			return nil, ErrNotFound
		}
	}
	reveal := staff
	if !reveal {
		var policy string
		var published bool
		var opensAt, dueAt *time.Time
		if err := s.db.QueryRow(ctx, `SELECT reveal_answers, status = 'published', opens_at, due_at FROM quizzes WHERE id = $1`, a.QuizID).
			Scan(&policy, &published, &opensAt, &dueAt); err != nil {
			return nil, err
		}
		reveal = canReveal(policy, quizStatus(published, opensAt, dueAt, s.now()), a.Status != AttemptInProgress)
	}
	rows, err := s.db.Query(ctx, `
		SELECT qq.id, ans.option_id, o.id
		FROM quiz_questions qq
		LEFT JOIN quiz_answers ans ON ans.question_id = qq.id AND ans.attempt_id = $1
		LEFT JOIN quiz_options o ON o.question_id = qq.id AND o.is_correct
		WHERE qq.quiz_id = $2 AND (ans.option_id IS NOT NULL OR $3)
		ORDER BY qq.position`, id, a.QuizID, reveal && a.Status != AttemptInProgress)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ans Answer
		var chosen, correct *string
		if err := rows.Scan(&ans.QuestionID, &chosen, &correct); err != nil {
			return nil, err
		}
		if chosen != nil {
			ans.OptionID = *chosen
		}
		if reveal && a.Status != AttemptInProgress {
			ans.CorrectOptionID = correct
			ok := chosen != nil && correct != nil && *chosen == *correct
			ans.Correct = &ok
		}
		a.Answers = append(a.Answers, ans)
	}
	return a, rows.Err()
}

func (s *Service) Attempts(ctx context.Context, v learning.Viewer, quizID string, page httpx.Page) ([]Attempt, int, error) {
	courseID, _, err := s.quizCourse(ctx, s.db, quizID, false)
	if err != nil {
		return nil, 0, err
	}
	staff, err := s.isStaff(ctx, v, courseID)
	if err != nil {
		return nil, 0, err
	}
	if !staff {
		if _, err := s.Quiz(ctx, v, quizID); err != nil {
			return nil, 0, err
		}
	}
	where := ` WHERE a.quiz_id = $1 AND ($2 OR a.student_id = $3)`
	rows, err := s.db.Query(ctx, attemptSelect+where+` ORDER BY a.started_at DESC, a.id LIMIT $4 OFFSET $5`,
		quizID, staff, v.UserID, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Attempt, error) {
		a, err := scanAttempt(row)
		if err != nil {
			return Attempt{}, err
		}
		return *a, nil
	})
	if err != nil {
		return nil, 0, err
	}
	var total int
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM quiz_attempts a`+where, quizID, staff, v.UserID).Scan(&total)
	return list, total, err
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
