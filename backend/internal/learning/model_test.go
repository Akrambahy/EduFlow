package learning

import "testing"

func TestLabels(t *testing.T) {
	if weeksLabel(1) != "1 week" || weeksLabel(6) != "6 weeks" || minutesLabel(15) != "15 min" {
		t.Fatal("label mismatch")
	}
}

func TestProgressPercent(t *testing.T) {
	cases := [][3]int{{0, 0, 0}, {0, 4, 0}, {1, 3, 33}, {2, 3, 66}, {4, 4, 100}}
	for _, c := range cases {
		if got := progressPercent(c[0], c[1]); got != c[2] {
			t.Errorf("progressPercent(%d,%d)=%d want %d", c[0], c[1], got, c[2])
		}
	}
}

func TestEnrollmentAPIStatus(t *testing.T) {
	if enrollmentAPIStatus("active", 0) != "enrolled" ||
		enrollmentAPIStatus("active", 2) != "in_progress" ||
		enrollmentAPIStatus("completed", 5) != "completed" ||
		enrollmentAPIStatus("withdrawn", 1) != "withdrawn" {
		t.Fatal("status mapping mismatch")
	}
}
