package performance

import (
	"errors"
	"math"
)

const (
	StandingGood      = "good"
	StandingWarning   = "warning"
	StandingProbation = "probation"
)

var ErrInvalidPolicy = errors.New("weights must add up to 100 and probation thresholds must not exceed warning thresholds")

type Policy struct {
	ID                  *string `json:"id"`
	CourseID            string  `json:"courseId"`
	Version             int     `json:"version"`
	QuizWeight          int     `json:"quizWeight"`
	AssignmentWeight    int     `json:"assignmentWeight"`
	AttendanceWeight    int     `json:"attendanceWeight"`
	ProjectWeight       int     `json:"projectWeight"`
	WarningGrade        int     `json:"warningGrade"`
	ProbationGrade      int     `json:"probationGrade"`
	WarningAttendance   int     `json:"warningAttendance"`
	ProbationAttendance int     `json:"probationAttendance"`
	WarningMissing      int     `json:"warningMissing"`
	Reason              *string `json:"reason"`
	IsDefault           bool    `json:"isDefault"`
}

func DefaultPolicy(courseID string) Policy {
	return Policy{
		CourseID: courseID, QuizWeight: 20, AssignmentWeight: 40, AttendanceWeight: 20, ProjectWeight: 20,
		WarningGrade: 60, ProbationGrade: 50, WarningAttendance: 75, ProbationAttendance: 60, WarningMissing: 2,
		IsDefault: true,
	}
}

func (p Policy) Validate() error {
	if p.QuizWeight+p.AssignmentWeight+p.AttendanceWeight+p.ProjectWeight != 100 {
		return ErrInvalidPolicy
	}
	if p.ProbationGrade > p.WarningGrade || p.ProbationAttendance > p.WarningAttendance || p.WarningMissing < 1 {
		return ErrInvalidPolicy
	}
	return nil
}

type Item struct {
	Percent  *float64
	Pending  bool
	Missing  bool
	Excluded bool
}

type Inputs struct {
	Quizzes     []Item
	Assignments []Item
	Projects    []Item
	Attendance  *float64
}

type Factor struct {
	Code      string   `json:"code"`
	Value     *float64 `json:"value,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Count     *int     `json:"count,omitempty"`
}

type Result struct {
	QuizAvg       *float64 `json:"quizAvg"`
	AssignmentAvg *float64 `json:"assignmentAvg"`
	ProjectGrade  *float64 `json:"projectGrade"`
	Attendance    *float64 `json:"attendance"`
	Overall       *float64 `json:"overall"`
	Missing       int      `json:"missingWork"`
	Pending       int      `json:"pendingWork"`
	Standing      string   `json:"standing"`
	Factors       []Factor `json:"factors"`
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func average(items []Item) (*float64, int, int) {
	var sum float64
	var n, missing, pending int
	for _, it := range items {
		switch {
		case it.Excluded:
		case it.Pending:
			pending++
		case it.Missing:
			missing++
			n++
		case it.Percent != nil:
			sum += *it.Percent
			n++
		}
	}
	if n == 0 {
		return nil, missing, pending
	}
	v := round2(sum / float64(n))
	return &v, missing, pending
}

func Compute(p Policy, in Inputs) Result {
	var r Result
	var qm, qp, am, ap, pm, pp int
	r.QuizAvg, qm, qp = average(in.Quizzes)
	r.AssignmentAvg, am, ap = average(in.Assignments)
	r.ProjectGrade, pm, pp = average(in.Projects)
	if in.Attendance != nil {
		v := round2(*in.Attendance)
		r.Attendance = &v
	}
	r.Missing = qm + am + pm
	r.Pending = qp + ap + pp

	var weighted, weights float64
	add := func(v *float64, w int) {
		if v != nil && w > 0 {
			weighted += *v * float64(w)
			weights += float64(w)
		}
	}
	add(r.QuizAvg, p.QuizWeight)
	add(r.AssignmentAvg, p.AssignmentWeight)
	add(r.ProjectGrade, p.ProjectWeight)
	add(r.Attendance, p.AttendanceWeight)
	if weights > 0 {
		v := round2(weighted / weights)
		r.Overall = &v
	}

	r.Factors = []Factor{}
	r.Standing = StandingGood
	escalate := func(s string) {
		if s == StandingProbation || (s == StandingWarning && r.Standing == StandingGood) {
			r.Standing = s
		}
	}
	threshold := func(v int) *float64 {
		f := float64(v)
		return &f
	}
	if r.Overall != nil {
		switch {
		case *r.Overall < float64(p.ProbationGrade):
			r.Factors = append(r.Factors, Factor{Code: "low_grade", Value: r.Overall, Threshold: threshold(p.ProbationGrade)})
			escalate(StandingProbation)
		case *r.Overall < float64(p.WarningGrade):
			r.Factors = append(r.Factors, Factor{Code: "low_grade", Value: r.Overall, Threshold: threshold(p.WarningGrade)})
			escalate(StandingWarning)
		}
	}
	if r.Attendance != nil {
		switch {
		case *r.Attendance < float64(p.ProbationAttendance):
			r.Factors = append(r.Factors, Factor{Code: "low_attendance", Value: r.Attendance, Threshold: threshold(p.ProbationAttendance)})
			escalate(StandingProbation)
		case *r.Attendance < float64(p.WarningAttendance):
			r.Factors = append(r.Factors, Factor{Code: "low_attendance", Value: r.Attendance, Threshold: threshold(p.WarningAttendance)})
			escalate(StandingWarning)
		}
	}
	if r.Missing >= p.WarningMissing {
		n := r.Missing
		r.Factors = append(r.Factors, Factor{Code: "missing_work", Count: &n, Threshold: threshold(p.WarningMissing)})
		escalate(StandingWarning)
	}
	return r
}

func worst(a, b string) string {
	rank := map[string]int{StandingGood: 0, StandingWarning: 1, StandingProbation: 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
