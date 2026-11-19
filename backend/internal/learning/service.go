package learning

import (
	"context"
	"errors"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type Viewer struct {
	UserID  string
	Roles   []string
	IsAdmin bool
	IP      string
}

func NewViewer(p *identity.Principal, ip string) Viewer {
	roles := p.Roles
	if roles == nil {
		roles = []string{}
	}
	return Viewer{UserID: p.UserID, Roles: roles, IsAdmin: p.IsAdmin(), IP: ip}
}

func (v Viewer) Has(role string) bool {
	for _, r := range v.Roles {
		if r == role {
			return true
		}
	}
	return false
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) record(ctx context.Context, tx pgx.Tx, v Viewer, action, targetType, targetID string, payload map[string]any) error {
	return audit.Record(ctx, tx, audit.Event{ActorID: v.UserID, Action: action, TargetType: targetType, TargetID: targetID, Context: payload, IP: v.IP})
}

func (s *Service) ListTracks(ctx context.Context, v Viewer, status string, page httpx.Page) ([]Track, int, error) {
	return s.repo.ListTracks(ctx, v, status, page)
}

func (s *Service) Track(ctx context.Context, v Viewer, id string) (*Track, error) {
	return s.repo.Track(ctx, v, id)
}

func (s *Service) CreateTrack(ctx context.Context, v Viewer, name, description, status string) (*Track, error) {
	if !v.IsAdmin {
		return nil, ErrForbidden
	}
	var id string
	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		var err error
		id, err = s.repo.InsertTrack(ctx, tx, strings.TrimSpace(name), description, status, v.UserID)
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, "track.created", "track", id, map[string]any{"status": status})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Track(ctx, v, id)
}

func (s *Service) UpdateTrack(ctx context.Context, v Viewer, id string, name, description, status *string) (*Track, error) {
	if !v.IsAdmin {
		return nil, ErrForbidden
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		name = &n
	}
	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.UpdateTrack(ctx, tx, id, name, description, status); err != nil {
			return err
		}
		payload := map[string]any{}
		if status != nil {
			payload["status"] = *status
		}
		return s.record(ctx, tx, v, "track.updated", "track", id, payload)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Track(ctx, v, id)
}

func (s *Service) SetTrackStaff(ctx context.Context, v Viewer, trackID, role string, userIDs []string) (*Track, error) {
	if !v.IsAdmin {
		return nil, ErrForbidden
	}
	table := "track_mentors"
	if role == identity.RoleTracker {
		table = "track_trackers"
	}
	ids := dedupe(userIDs)
	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.LockTrack(ctx, tx, trackID); err != nil {
			return err
		}
		if err := s.repo.ReplaceTrackStaff(ctx, tx, table, trackID, ids, role, v.UserID); err != nil {
			return err
		}
		return s.record(ctx, tx, v, "track."+role+"s_assigned", "track", trackID, map[string]any{"userIds": ids})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Track(ctx, v, trackID)
}

func (s *Service) ListCourses(ctx context.Context, v Viewer, f CourseFilter, page httpx.Page) ([]Course, int, error) {
	return s.repo.ListCourses(ctx, v, f, page)
}

func (s *Service) Course(ctx context.Context, v Viewer, id string) (*Course, error) {
	return s.repo.Course(ctx, v, id)
}

func (s *Service) CreateCourse(ctx context.Context, v Viewer, in CourseInput) (*Course, error) {
	switch {
	case v.IsAdmin:
	case v.Has(identity.RoleInstructor):
		if in.InstructorID != nil && *in.InstructorID != v.UserID {
			return nil, ErrForbidden
		}
		if in.TrackID != nil {
			return nil, ErrForbidden
		}
		in.InstructorID = &v.UserID
	default:
		return nil, ErrForbidden
	}
	var id string
	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.checkInstructor(ctx, tx, in.InstructorID); err != nil {
			return err
		}
		var err error
		id, err = s.repo.InsertCourse(ctx, tx, in, v.UserID)
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, "course.created", "course", id, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Course(ctx, v, id)
}

func (s *Service) UpdateCourse(ctx context.Context, v Viewer, id string, in CourseInput, clearTrack bool) (*Course, error) {
	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.authorizeCourse(ctx, tx, v, id); err != nil {
			return err
		}
		if !v.IsAdmin && (in.InstructorID != nil || in.TrackID != nil || clearTrack) {
			return ErrForbidden
		}
		if err := s.checkInstructor(ctx, tx, in.InstructorID); err != nil {
			return err
		}
		if err := s.repo.UpdateCourse(ctx, tx, id, in, clearTrack); err != nil {
			return err
		}
		payload := map[string]any{}
		if in.Status != nil {
			payload["status"] = *in.Status
		}
		return s.record(ctx, tx, v, "course.updated", "course", id, payload)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Course(ctx, v, id)
}

func (s *Service) checkInstructor(ctx context.Context, tx pgx.Tx, id *string) error {
	if id == nil {
		return nil
	}
	ok, err := s.repo.userHasRole(ctx, tx, *id, identity.RoleInstructor)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidInstructor
	}
	return nil
}

func (s *Service) authorizeCourse(ctx context.Context, tx pgx.Tx, v Viewer, courseID string) error {
	owner, err := s.repo.lockCourse(ctx, tx, courseID)
	if err != nil {
		return err
	}
	if v.IsAdmin || (v.Has(identity.RoleInstructor) && owner.InstructorID != nil && *owner.InstructorID == v.UserID) {
		return nil
	}
	if _, err := s.repo.Course(ctx, v, courseID); errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return ErrForbidden
}

func (s *Service) Outline(ctx context.Context, v Viewer, courseID string) ([]Module, error) {
	course, err := s.repo.Course(ctx, v, courseID)
	if err != nil {
		return nil, err
	}
	student, err := s.studentFor(ctx, v, courseID)
	if err != nil {
		return nil, err
	}
	return s.repo.Outline(ctx, s.repo.db, courseID, course.SequentialUnlock, student)
}

func (s *Service) studentFor(ctx context.Context, v Viewer, courseID string) (string, error) {
	if !v.Has(identity.RoleStudent) {
		return "", nil
	}
	ok, err := s.repo.IsEnrolled(ctx, v.UserID, courseID)
	if err != nil || !ok {
		return "", err
	}
	return v.UserID, nil
}

func (s *Service) Lesson(ctx context.Context, v Viewer, lessonID string) (*Lesson, error) {
	ref, err := s.repo.lessonRef(ctx, s.repo.db, lessonID)
	if err != nil {
		return nil, err
	}
	modules, err := s.Outline(ctx, v, ref.CourseID)
	if err != nil {
		return nil, err
	}
	lesson := findLesson(modules, lessonID)
	if lesson == nil {
		return nil, ErrNotFound
	}
	if lesson.Status == LessonLocked {
		return nil, ErrLessonLocked
	}
	lesson.Content = &ref.Content
	lesson.VideoURL = ref.VideoURL
	return lesson, nil
}

func (s *Service) CreateModule(ctx context.Context, v Viewer, courseID, title string, position *int) ([]Module, error) {
	return s.mutateCourse(ctx, v, courseID, func(tx pgx.Tx) error {
		_, err := s.repo.InsertModule(ctx, tx, courseID, strings.TrimSpace(title), position)
		return err
	})
}

func (s *Service) UpdateModule(ctx context.Context, v Viewer, moduleID string, title *string, position *int) ([]Module, error) {
	courseID, err := s.repo.moduleCourse(ctx, s.repo.db, moduleID)
	if err != nil {
		return nil, err
	}
	if title != nil {
		t := strings.TrimSpace(*title)
		title = &t
	}
	return s.mutateCourse(ctx, v, courseID, func(tx pgx.Tx) error {
		return s.repo.UpdateModule(ctx, tx, moduleID, courseID, title, position)
	})
}

func (s *Service) DeleteModule(ctx context.Context, v Viewer, moduleID string) error {
	courseID, err := s.repo.moduleCourse(ctx, s.repo.db, moduleID)
	if err != nil {
		return err
	}
	_, err = s.mutateCourse(ctx, v, courseID, func(tx pgx.Tx) error {
		return s.repo.DeleteModule(ctx, tx, moduleID, courseID)
	})
	return err
}

func (s *Service) CreateLesson(ctx context.Context, v Viewer, moduleID string, in LessonInput) ([]Module, error) {
	courseID, err := s.repo.moduleCourse(ctx, s.repo.db, moduleID)
	if err != nil {
		return nil, err
	}
	return s.mutateCourse(ctx, v, courseID, func(tx pgx.Tx) error {
		_, err := s.repo.InsertLesson(ctx, tx, moduleID, in)
		return err
	})
}

func (s *Service) UpdateLesson(ctx context.Context, v Viewer, lessonID string, in LessonInput) ([]Module, error) {
	ref, err := s.repo.lessonRef(ctx, s.repo.db, lessonID)
	if err != nil {
		return nil, err
	}
	return s.mutateCourse(ctx, v, ref.CourseID, func(tx pgx.Tx) error {
		return s.repo.UpdateLesson(ctx, tx, lessonID, ref.ModuleID, in)
	})
}

func (s *Service) DeleteLesson(ctx context.Context, v Viewer, lessonID string) error {
	ref, err := s.repo.lessonRef(ctx, s.repo.db, lessonID)
	if err != nil {
		return err
	}
	_, err = s.mutateCourse(ctx, v, ref.CourseID, func(tx pgx.Tx) error {
		return s.repo.DeleteLesson(ctx, tx, lessonID, ref.ModuleID)
	})
	return err
}

func (s *Service) mutateCourse(ctx context.Context, v Viewer, courseID string, fn func(pgx.Tx) error) ([]Module, error) {
	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.authorizeCourse(ctx, tx, v, courseID); err != nil {
			return err
		}
		return fn(tx)
	})
	if err != nil {
		return nil, err
	}
	return s.Outline(ctx, v, courseID)
}

func (s *Service) RecordProgress(ctx context.Context, v Viewer, lessonID string, complete bool) (*Enrollment, error) {
	ref, err := s.repo.lessonRef(ctx, s.repo.db, lessonID)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.Course(ctx, v, ref.CourseID); err != nil {
		return nil, err
	}
	var enrollmentID string
	err = s.repo.inTx(ctx, func(tx pgx.Tx) error {
		course, err := s.repo.lockCourse(ctx, tx, ref.CourseID)
		if err != nil {
			return err
		}
		if course.Status == CourseArchived {
			return ErrCourseArchived
		}
		id, _, err := s.repo.lockCurrentEnrollment(ctx, tx, v.UserID, ref.CourseID)
		if err != nil {
			return err
		}
		enrollmentID = id
		modules, err := s.repo.Outline(ctx, tx, ref.CourseID, course.Sequential, v.UserID)
		if err != nil {
			return err
		}
		lesson := findLesson(modules, lessonID)
		if lesson == nil {
			return ErrNotFound
		}
		if lesson.Status == LessonLocked {
			return ErrLessonLocked
		}
		if err := s.repo.markProgress(ctx, tx, v.UserID, lessonID, complete); err != nil {
			return err
		}
		return s.repo.completeIfFinished(ctx, tx, id, v.UserID, ref.CourseID)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Enrollment(ctx, s.repo.db, enrollmentID)
}

func (s *Service) ListEnrollments(ctx context.Context, v Viewer, f EnrollmentFilter, page httpx.Page) ([]Enrollment, int, error) {
	return s.repo.ListEnrollments(ctx, v, f, page)
}

func (s *Service) Enroll(ctx context.Context, v Viewer, courseID, studentID string) (*Enrollment, error) {
	var id string
	err := s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.authorizeCourse(ctx, tx, v, courseID); err != nil {
			return err
		}
		course, err := s.repo.lockCourse(ctx, tx, courseID)
		if err != nil {
			return err
		}
		if course.Status == CourseArchived {
			return ErrCourseArchived
		}
		ok, err := s.repo.userHasRole(ctx, tx, studentID, identity.RoleStudent)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotStudent
		}
		id, err = s.repo.InsertEnrollment(ctx, tx, studentID, courseID, v.UserID)
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, "enrollment.created", "enrollment", id, map[string]any{"courseId": courseID, "studentId": studentID})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.Enrollment(ctx, s.repo.db, id)
}

func (s *Service) Withdraw(ctx context.Context, v Viewer, courseID, studentID string) error {
	return s.repo.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.authorizeCourse(ctx, tx, v, courseID); err != nil {
			return err
		}
		id, err := s.repo.WithdrawEnrollment(ctx, tx, studentID, courseID)
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, "enrollment.withdrawn", "enrollment", id, map[string]any{"courseId": courseID, "studentId": studentID})
	})
}

func findLesson(modules []Module, id string) *Lesson {
	for i := range modules {
		for j := range modules[i].Lessons {
			if modules[i].Lessons[j].ID == id {
				l := modules[i].Lessons[j]
				return &l
			}
		}
	}
	return nil
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.ToLower(id)
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

const CourseVisibility = courseVisible

func (s *Service) CheckManage(ctx context.Context, v Viewer, courseID string) error {
	return s.repo.inTx(ctx, func(tx pgx.Tx) error {
		return s.authorizeCourse(ctx, tx, v, courseID)
	})
}
