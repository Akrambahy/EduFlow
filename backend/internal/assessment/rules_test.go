package assessment

import (
	"errors"
	"testing"
	"time"
)

var base = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func TestQuizStatus(t *testing.T) {
	open, due := base.Add(time.Hour), base.Add(2*time.Hour)
	cases := []struct {
		pub  bool
		now  time.Time
		want string
	}{
		{false, base, QuizDraft},
		{true, base, QuizScheduled},
		{true, open, QuizOpen},
		{true, due, QuizClosed},
	}
	for _, c := range cases {
		if got := quizStatus(c.pub, &open, &due, c.now); got != c.want {
			t.Errorf("now=%v got %s want %s", c.now, got, c.want)
		}
	}
	if quizStatus(true, nil, nil, base) != QuizOpen {
		t.Fatal("unbounded quiz should be open")
	}
}

func TestAttemptDeadline(t *testing.T) {
	if attemptDeadline(base, nil, nil) != nil {
		t.Fatal("no limit means no deadline")
	}
	if d := attemptDeadline(base, ptr(30), nil); !d.Equal(base.Add(30 * time.Minute)) {
		t.Fatalf("time limit: %v", d)
	}
	due := base.Add(10 * time.Minute)
	if d := attemptDeadline(base, ptr(30), &due); !d.Equal(due) {
		t.Fatalf("due date should cap the limit: %v", d)
	}
}

func TestCanReveal(t *testing.T) {
	if !canReveal(RevealAfterSubmission, QuizOpen, true) || canReveal(RevealAfterSubmission, QuizOpen, false) {
		t.Fatal("after_submission")
	}
	if canReveal(RevealAfterClose, QuizOpen, true) || !canReveal(RevealAfterClose, QuizClosed, true) {
		t.Fatal("after_close")
	}
	if canReveal(RevealNever, QuizClosed, true) {
		t.Fatal("never")
	}
}

func TestScoreAttempt(t *testing.T) {
	qs := []scoredQuestion{{"q1", 1, "a"}, {"q2", 2, "b"}, {"q3", 1, "c"}}
	r := scoreAttempt(qs, map[string]string{"q1": "a", "q2": "x", "q3": "c"}, 50)
	if r.Score != 2 || r.MaxScore != 4 || r.Percentage != 50 || !r.Passed {
		t.Fatalf("got %+v", r)
	}
	r = scoreAttempt(qs, map[string]string{}, 50)
	if r.Score != 0 || r.Passed {
		t.Fatalf("empty answers %+v", r)
	}
}

func TestSubmissionTiming(t *testing.T) {
	due := base
	until := base.Add(24 * time.Hour)
	if late, err := submissionTiming(due, LateReject, nil, base.Add(-time.Minute)); late || err != nil {
		t.Fatal("on time")
	}
	if _, err := submissionTiming(due, LateReject, nil, base.Add(time.Minute)); !errors.Is(err, ErrPastDeadline) {
		t.Fatal("reject policy")
	}
	if late, err := submissionTiming(due, LatePenalty, &until, base.Add(time.Hour)); !late || err != nil {
		t.Fatal("penalty window")
	}
	if _, err := submissionTiming(due, LateAccept, &until, until.Add(time.Second)); !errors.Is(err, ErrPastDeadline) {
		t.Fatal("after late_until")
	}
	if late, err := submissionTiming(due, LateAccept, nil, base.Add(100*time.Hour)); !late || err != nil {
		t.Fatal("accept without limit")
	}
}

func TestFinalGrade(t *testing.T) {
	if finalGrade(80, true, LatePenalty, 10) != 72 {
		t.Fatal("penalty")
	}
	if finalGrade(80, true, LateAccept, 10) != 80 || finalGrade(80, false, LatePenalty, 10) != 80 {
		t.Fatal("no penalty cases")
	}
	if finalGrade(77.5, true, LatePenalty, 15) != 65.88 {
		t.Fatalf("rounding: %v", finalGrade(77.5, true, LatePenalty, 15))
	}
}
