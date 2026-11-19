package attendance

import (
	"errors"
	"time"
)

const (
	StatusPresent = "present"
	StatusLate    = "late"
	StatusAbsent  = "absent"
	StatusExcused = "excused"

	MethodQR     = "qr"
	MethodManual = "manual"
	MethodAuto   = "auto"

	WindowOpen   = "open"
	WindowClosed = "closed"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrWindowClosed     = errors.New("attendance is closed for this session")
	ErrWindowNotOpen    = errors.New("attendance has not been started for this session")
	ErrSessionCancelled = errors.New("session is cancelled")
	ErrNotEnrolled      = errors.New("you are not enrolled in this course")
	ErrAlreadyRecorded  = errors.New("attendance already recorded")
	ErrNotStudent       = errors.New("user is not an enrolled student")
)

type Window struct {
	ID              string     `json:"id"`
	SessionID       string     `json:"sessionId"`
	CourseID        string     `json:"courseId"`
	Status          string     `json:"status"`
	GraceMinutes    int        `json:"graceMinutes"`
	TokenTTLSeconds int        `json:"tokenTtlSeconds"`
	OpenedAt        time.Time  `json:"openedAt"`
	ClosedAt        *time.Time `json:"closedAt"`
	Token           *string    `json:"token,omitempty"`
	TokenExpiresAt  *time.Time `json:"tokenExpiresAt,omitempty"`
	Summary         Summary    `json:"summary"`
	secret          []byte
	sessionStart    time.Time
}

type Summary struct {
	Enrolled int `json:"enrolled"`
	Present  int `json:"present"`
	Late     int `json:"late"`
	Absent   int `json:"absent"`
	Excused  int `json:"excused"`
	Pending  int `json:"pending"`
}

type Record struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"sessionId"`
	StudentID   string     `json:"studentId"`
	StudentName string     `json:"studentName"`
	Status      string     `json:"status"`
	Method      string     `json:"method"`
	CheckedInAt *time.Time `json:"checkedInAt"`
	Reason      *string    `json:"reason"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type StudentSummary struct {
	TotalSessions int `json:"totalSessions"`
	Present       int `json:"present"`
	Late          int `json:"late"`
	Absent        int `json:"absent"`
	Excused       int `json:"excused"`
	Rate          int `json:"rate"`
}

func checkInStatus(sessionStart time.Time, grace time.Duration, at time.Time) string {
	if at.After(sessionStart.Add(grace)) {
		return StatusLate
	}
	return StatusPresent
}

func attendanceRate(s StudentSummary) int {
	counted := s.TotalSessions - s.Excused
	if s.TotalSessions == 0 {
		return 0
	}
	if counted <= 0 {
		return 100
	}
	return (s.Present + s.Late) * 100 / counted
}
