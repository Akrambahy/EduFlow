package assessment

import (
	"errors"
	"math"
	"time"
)

const (
	QuizDraft     = "draft"
	QuizScheduled = "scheduled"
	QuizOpen      = "open"
	QuizClosed    = "closed"

	RevealAfterSubmission = "after_submission"
	RevealAfterClose      = "after_close"
	RevealNever           = "never"

	AttemptInProgress = "in_progress"
	AttemptSubmitted  = "submitted"
	AttemptExpired    = "expired"

	LateReject  = "reject"
	LateAccept  = "accept"
	LatePenalty = "penalty"

	SubmissionSubmitted  = "submitted"
	SubmissionGraded     = "graded"
	SubmissionReturned   = "returned"
	SubmissionSuperseded = "superseded"

	SubmitGrace = 2 * time.Minute
)

var (
	ErrNotFound          = errors.New("not found")
	ErrForbidden         = errors.New("forbidden")
	ErrQuizNotOpen       = errors.New("quiz is not open")
	ErrAttemptLimit      = errors.New("attempt limit reached")
	ErrAttemptClosed     = errors.New("attempt is no longer in progress")
	ErrAttemptExpired    = errors.New("attempt time is over")
	ErrQuizHasAttempts   = errors.New("quiz already has attempts")
	ErrQuizInvalid       = errors.New("every question needs at least two options and exactly one correct option")
	ErrInvalidAnswer     = errors.New("answers must reference this quiz's questions and options")
	ErrNotEnrolled       = errors.New("you are not enrolled in this course")
	ErrPastDeadline      = errors.New("the submission deadline has passed")
	ErrAssignmentNotOpen = errors.New("assignment is not published")
	ErrAlreadyGraded     = errors.New("submission is already graded")
	ErrSubmissionType    = errors.New("submission type is not allowed for this assignment")
	ErrInvalidFile       = errors.New("file must exist and be uploaded by you")
	ErrGradeRange        = errors.New("grade must be between 0 and the maximum grade")
	ErrReasonRequired    = errors.New("a reason is required when changing an existing grade")
	ErrNotLatest         = errors.New("only the latest submission can be graded")
	ErrInvalidLatePolicy = errors.New("late policy settings are inconsistent")
	ErrInvalidSchedule   = errors.New("dueAt must be after opensAt")
)

func quizStatus(published bool, opensAt, dueAt *time.Time, now time.Time) string {
	switch {
	case !published:
		return QuizDraft
	case opensAt != nil && now.Before(*opensAt):
		return QuizScheduled
	case dueAt != nil && !now.Before(*dueAt):
		return QuizClosed
	default:
		return QuizOpen
	}
}

func attemptDeadline(started time.Time, limitMinutes *int, dueAt *time.Time) *time.Time {
	var deadline *time.Time
	if limitMinutes != nil {
		d := started.Add(time.Duration(*limitMinutes) * time.Minute)
		deadline = &d
	}
	if dueAt != nil && (deadline == nil || dueAt.Before(*deadline)) {
		d := *dueAt
		deadline = &d
	}
	return deadline
}

func canReveal(policy, quizState string, attemptSubmitted bool) bool {
	switch policy {
	case RevealAfterSubmission:
		return attemptSubmitted
	case RevealAfterClose:
		return quizState == QuizClosed
	default:
		return false
	}
}

type scoredQuestion struct {
	ID            string
	Points        int
	CorrectOption string
}

type ScoreResult struct {
	Score      int
	MaxScore   int
	Percentage int
	Passed     bool
}

func scoreAttempt(questions []scoredQuestion, answers map[string]string, passPercent int) ScoreResult {
	var r ScoreResult
	for _, q := range questions {
		r.MaxScore += q.Points
		if answers[q.ID] != "" && answers[q.ID] == q.CorrectOption {
			r.Score += q.Points
		}
	}
	if r.MaxScore > 0 {
		r.Percentage = r.Score * 100 / r.MaxScore
	}
	r.Passed = r.Percentage >= passPercent
	return r
}

func submissionTiming(dueAt time.Time, policy string, lateUntil *time.Time, now time.Time) (late bool, err error) {
	if !now.After(dueAt) {
		return false, nil
	}
	if policy == LateReject {
		return true, ErrPastDeadline
	}
	if lateUntil != nil && now.After(*lateUntil) {
		return true, ErrPastDeadline
	}
	return true, nil
}

func finalGrade(grade float64, late bool, policy string, penaltyPercent int) float64 {
	if !late || policy != LatePenalty || penaltyPercent == 0 {
		return grade
	}
	return math.Round(grade*float64(100-penaltyPercent)) / 100
}
