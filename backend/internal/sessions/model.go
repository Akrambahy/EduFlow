package sessions

import (
	"errors"
	"time"
)

const (
	StatusUpcoming  = "upcoming"
	StatusLive      = "live"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidTimes   = errors.New("endsAt must be after startsAt and within 12 hours")
	ErrInvalidFile    = errors.New("file must exist and be uploaded by you")
	ErrInvalidSession = errors.New("session must belong to the same course")
	ErrSourceRequired = errors.New("provide exactly one of url or fileId")
	ErrCancelled      = errors.New("session is cancelled")
)

type Session struct {
	ID              string    `json:"id"`
	CourseID        string    `json:"courseId"`
	CourseTitle     string    `json:"courseTitle"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Type            string    `json:"type"`
	Location        string    `json:"location"`
	Date            string    `json:"date"`
	StartTime       string    `json:"startTime"`
	EndTime         string    `json:"endTime"`
	StartsAt        time.Time `json:"startsAt"`
	EndsAt          time.Time `json:"endsAt"`
	InstructorID    *string   `json:"instructorId"`
	InstructorName  *string   `json:"instructorName"`
	Status          string    `json:"status"`
	RecordingURL    *string   `json:"recordingUrl"`
	RecordingFileID *string   `json:"recordingFileId"`
	QRCode          *string   `json:"qrCode"`
	cancelledAt     *time.Time
}

type MaterialFile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

type Material struct {
	ID          string        `json:"id"`
	CourseID    string        `json:"courseId"`
	SessionID   *string       `json:"sessionId"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Kind        string        `json:"kind"`
	URL         *string       `json:"url"`
	File        *MaterialFile `json:"file"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

func (s *Session) finalize(now time.Time, loc *time.Location) {
	local := s.StartsAt.In(loc)
	s.Date = local.Format("2006-01-02")
	s.StartTime = local.Format("15:04")
	s.EndTime = s.EndsAt.In(loc).Format("15:04")
	s.StartsAt = s.StartsAt.UTC()
	s.EndsAt = s.EndsAt.UTC()
	s.Status = sessionStatus(s.cancelledAt, s.StartsAt, s.EndsAt, now)
}

func sessionStatus(cancelledAt *time.Time, start, end, now time.Time) string {
	switch {
	case cancelledAt != nil:
		return StatusCancelled
	case !now.Before(end):
		return StatusCompleted
	case !now.Before(start):
		return StatusLive
	default:
		return StatusUpcoming
	}
}
