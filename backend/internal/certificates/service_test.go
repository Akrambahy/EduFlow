package certificates

import (
	"regexp"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/performance"
)

func f(v float64) *float64 { return &v }

func TestEvaluate(t *testing.T) {
	req := DefaultRequirements("c")
	good := performance.Result{Overall: f(80), Attendance: f(90), Standing: performance.StandingGood}
	if ok, reasons := Evaluate(req, good, 10, 10); !ok || len(reasons) != 0 {
		t.Fatalf("eligible student rejected: %v", reasons)
	}
	cases := []struct {
		r         performance.Result
		completed int
		want      string
	}{
		{good, 9, ReasonLessons},
		{performance.Result{Overall: f(59), Attendance: f(90), Standing: performance.StandingWarning}, 10, ReasonGrade},
		{performance.Result{Attendance: f(90), Standing: performance.StandingGood}, 10, ReasonNoGrade},
		{performance.Result{Overall: f(80), Attendance: f(70), Standing: performance.StandingWarning}, 10, ReasonAttendance},
		{performance.Result{Overall: f(80), Attendance: f(90), Standing: performance.StandingProbation}, 10, ReasonProbation},
	}
	for _, c := range cases {
		ok, reasons := Evaluate(req, c.r, c.completed, 10)
		found := false
		for _, r := range reasons {
			found = found || r == c.want
		}
		if ok || !found {
			t.Errorf("want %s, got ok=%t %v", c.want, ok, reasons)
		}
	}
	req.RequireAllLessons = false
	req.AllowProbation = true
	if ok, _ := Evaluate(req, performance.Result{Overall: f(80), Standing: performance.StandingProbation}, 0, 10); !ok {
		t.Fatal("relaxed requirements should pass")
	}
}

func TestNewCode(t *testing.T) {
	re := regexp.MustCompile(`^EF-2026-[A-Z2-9]{8}$`)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := NewCode(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		if err != nil || !re.MatchString(c) {
			t.Fatalf("bad code %q %v", c, err)
		}
		if seen[c] {
			t.Fatal("duplicate code")
		}
		seen[c] = true
	}
}
