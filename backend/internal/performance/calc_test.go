package performance

import "testing"

func pct(v float64) *float64 { return &v }

func TestPolicyValidate(t *testing.T) {
	if err := DefaultPolicy("c").Validate(); err != nil {
		t.Fatal(err)
	}
	p := DefaultPolicy("c")
	p.QuizWeight = 30
	if p.Validate() == nil {
		t.Fatal("weights summing to 110 must fail")
	}
	p = DefaultPolicy("c")
	p.ProbationGrade = 70
	if p.Validate() == nil {
		t.Fatal("probation above warning must fail")
	}
}

func TestComputeRedistributesMissingComponents(t *testing.T) {
	r := Compute(DefaultPolicy("c"), Inputs{
		Quizzes:     []Item{{Percent: pct(80)}, {Percent: pct(60)}},
		Assignments: []Item{{Percent: pct(90)}, {Pending: true}},
		Attendance:  pct(100),
	})
	if *r.QuizAvg != 70 || *r.AssignmentAvg != 90 || r.ProjectGrade != nil {
		t.Fatalf("components %+v", r)
	}
	want := (70*20 + 90*40 + 100*20) / 80.0
	if *r.Overall != round2(want) || r.Standing != StandingGood || r.Pending != 1 || len(r.Factors) != 0 {
		t.Fatalf("overall %v want %v, %+v", *r.Overall, want, r)
	}
}

func TestComputeMissingWorkCountsAsZero(t *testing.T) {
	r := Compute(DefaultPolicy("c"), Inputs{
		Assignments: []Item{{Percent: pct(100)}, {Missing: true}, {Missing: true}, {Excluded: true}},
	})
	if *r.AssignmentAvg != 33.33 || r.Missing != 2 {
		t.Fatalf("%+v", r)
	}
	if r.Standing != StandingProbation {
		t.Fatalf("33%% overall should be probation, got %s", r.Standing)
	}
	codes := map[string]bool{}
	for _, f := range r.Factors {
		codes[f.Code] = true
	}
	if !codes["low_grade"] || !codes["missing_work"] {
		t.Fatalf("factors %+v", r.Factors)
	}
}

func TestComputeAttendanceThresholds(t *testing.T) {
	p := DefaultPolicy("c")
	cases := map[float64]string{100: StandingGood, 74: StandingWarning, 59: StandingProbation}
	for att, want := range cases {
		r := Compute(p, Inputs{Quizzes: []Item{{Percent: pct(95)}}, Attendance: pct(att)})
		if r.Standing != want {
			t.Errorf("attendance %v: got %s want %s", att, r.Standing, want)
		}
	}
}

func TestComputeNoData(t *testing.T) {
	r := Compute(DefaultPolicy("c"), Inputs{Quizzes: []Item{{Pending: true}}})
	if r.Overall != nil || r.Standing != StandingGood || r.Pending != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestWorst(t *testing.T) {
	if worst(StandingGood, StandingWarning) != StandingWarning || worst(StandingProbation, StandingWarning) != StandingProbation {
		t.Fatal("worst")
	}
}
