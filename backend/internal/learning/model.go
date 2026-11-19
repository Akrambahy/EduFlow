package learning

import (
	"errors"
	"strconv"
	"time"
)

const (
	TrackDraft    = "draft"
	TrackActive   = "active"
	TrackArchived = "archived"

	CourseDraft     = "draft"
	CoursePublished = "published"
	CourseArchived  = "archived"

	EnrollmentActive    = "active"
	EnrollmentCompleted = "completed"
	EnrollmentWithdrawn = "withdrawn"

	LessonCompleted  = "completed"
	LessonInProgress = "in_progress"
	LessonAvailable  = "available"
	LessonLocked     = "locked"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrForbidden         = errors.New("forbidden")
	ErrTrackNameTaken    = errors.New("track name already exists")
	ErrInvalidAssignee   = errors.New("every assignee must be an active user with the required role")
	ErrInvalidReference  = errors.New("referenced resource does not exist")
	ErrInvalidInstructor = errors.New("instructor must be an active user with the instructor role")
	ErrAlreadyEnrolled   = errors.New("student is already enrolled")
	ErrNotEnrolled       = errors.New("student is not enrolled")
	ErrNotStudent        = errors.New("user is not an active student")
	ErrCourseArchived    = errors.New("course is archived")
	ErrLessonLocked      = errors.New("lesson is locked")
	ErrModuleNotEmpty    = errors.New("module still has lessons")
	ErrLessonHasProgress = errors.New("lesson has student progress")
)

type Track struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	Courses       []string  `json:"courses"`
	TotalStudents int       `json:"totalStudents"`
	MentorID      *string   `json:"mentorId"`
	TrackerID     *string   `json:"trackerId"`
	MentorIDs     []string  `json:"mentorIds"`
	TrackerIDs    []string  `json:"trackerIds"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Course struct {
	ID               string     `json:"id"`
	TrackID          *string    `json:"trackId"`
	TrackName        *string    `json:"trackName"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Category         string     `json:"category"`
	Color            string     `json:"color"`
	InstructorID     *string    `json:"instructorId"`
	InstructorName   *string    `json:"instructorName"`
	Duration         string     `json:"duration"`
	DurationWeeks    int        `json:"durationWeeks"`
	TotalModules     int        `json:"totalModules"`
	TotalLessons     int        `json:"totalLessons"`
	EnrolledCount    int        `json:"enrolledCount"`
	Status           string     `json:"status"`
	SequentialUnlock bool       `json:"sequentialUnlock"`
	PublishedAt      *time.Time `json:"publishedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type Module struct {
	ID       string   `json:"id"`
	CourseID string   `json:"courseId"`
	Title    string   `json:"title"`
	Order    int      `json:"order"`
	Lessons  []Lesson `json:"lessons"`
}

type Lesson struct {
	ID              string  `json:"id"`
	ModuleID        string  `json:"moduleId"`
	CourseID        string  `json:"courseId"`
	Title           string  `json:"title"`
	Type            string  `json:"type"`
	Duration        string  `json:"duration"`
	DurationMinutes int     `json:"durationMinutes"`
	Order           int     `json:"order"`
	Status          string  `json:"status"`
	Content         *string `json:"content,omitempty"`
	VideoURL        *string `json:"videoUrl,omitempty"`
}

type Enrollment struct {
	ID               string     `json:"id"`
	StudentID        string     `json:"studentId"`
	StudentName      string     `json:"studentName"`
	CourseID         string     `json:"courseId"`
	CourseTitle      string     `json:"courseTitle"`
	Progress         int        `json:"progress"`
	CompletedLessons int        `json:"completedLessons"`
	TotalLessons     int        `json:"totalLessons"`
	Status           string     `json:"status"`
	EnrolledAt       time.Time  `json:"enrolledAt"`
	CompletedAt      *time.Time `json:"completedAt"`
	WithdrawnAt      *time.Time `json:"withdrawnAt"`
}

func weeksLabel(n int) string {
	if n == 1 {
		return "1 week"
	}
	return strconv.Itoa(n) + " weeks"
}

func minutesLabel(n int) string {
	return strconv.Itoa(n) + " min"
}

func progressPercent(done, total int) int {
	if total == 0 {
		return 0
	}
	return done * 100 / total
}

func enrollmentAPIStatus(db string, completed int) string {
	switch db {
	case EnrollmentActive:
		if completed == 0 {
			return "enrolled"
		}
		return "in_progress"
	default:
		return db
	}
}
