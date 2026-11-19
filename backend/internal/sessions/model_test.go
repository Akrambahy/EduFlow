package sessions

import (
	"testing"
	"time"
)

func TestSessionStatus(t *testing.T) {
	start := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	cancelled := start
	cases := []struct {
		now       time.Time
		cancelled *time.Time
		want      string
	}{
		{start.Add(-time.Minute), nil, StatusUpcoming},
		{start, nil, StatusLive},
		{end.Add(-time.Second), nil, StatusLive},
		{end, nil, StatusCompleted},
		{start.Add(-time.Hour), &cancelled, StatusCancelled},
	}
	for _, c := range cases {
		if got := sessionStatus(c.cancelled, start, end, c.now); got != c.want {
			t.Errorf("now=%v got %s want %s", c.now, got, c.want)
		}
	}
}

func TestFinalizeUsesLocalTimezone(t *testing.T) {
	cairo, err := time.LoadLocation("Africa/Cairo")
	if err != nil {
		t.Fatal(err)
	}
	s := Session{
		StartsAt: time.Date(2026, 11, 20, 22, 30, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC),
	}
	s.finalize(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), cairo)
	if s.Date != "2026-11-21" || s.StartTime != "00:30" || s.EndTime != "02:00" || s.Status != StatusUpcoming {
		t.Fatalf("unexpected %+v", s)
	}
}
