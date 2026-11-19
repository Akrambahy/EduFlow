package learning_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
)

type fixture struct {
	app                            *testkit.App
	admin, instructor, other       string
	mentor, tracker, student, outs string
	instructorID, mentorID         string
	trackerID, studentID, outsID   string
	otherInstructorID              string
}

func setup(t *testing.T) *fixture {
	app := testkit.NewApp(t)
	f := &fixture{app: app}
	_, f.admin = app.UserWithToken("Sara Admin", "admin@eduflow.dev", "admin")
	f.instructorID, f.instructor = app.UserWithToken("Ahmed Khalil", "instructor@eduflow.dev", "instructor")
	f.otherInstructorID, f.other = app.UserWithToken("Other Inst", "other@eduflow.dev", "instructor")
	f.mentorID, f.mentor = app.UserWithToken("Layla Hassan", "mentor@eduflow.dev", "mentor")
	f.trackerID, f.tracker = app.UserWithToken("Omar Nabil", "tracker@eduflow.dev", "tracker")
	f.studentID, f.student = app.UserWithToken("Marwan Ashour", "student@eduflow.dev", "student")
	f.outsID, f.outs = app.UserWithToken("Outsider", "outsider@eduflow.dev", "student")
	return f
}

func (f *fixture) createTrack(name, status string) learning.Track {
	f.app.T.Helper()
	res := f.app.Do(http.MethodPost, "/api/v1/tracks", f.admin, map[string]any{"name": name, "status": status})
	f.app.Expect(res, http.StatusCreated, "")
	var t learning.Track
	res.JSON(&t)
	return t
}

func (f *fixture) createCourse(token string, body map[string]any) learning.Course {
	f.app.T.Helper()
	res := f.app.Do(http.MethodPost, "/api/v1/courses", token, body)
	f.app.Expect(res, http.StatusCreated, "")
	var c learning.Course
	res.JSON(&c)
	return c
}

func (f *fixture) addModule(token, courseID, title string) []learning.Module {
	f.app.T.Helper()
	res := f.app.Do(http.MethodPost, "/api/v1/courses/"+courseID+"/modules", token, map[string]any{"title": title})
	f.app.Expect(res, http.StatusCreated, "")
	var m []learning.Module
	res.JSON(&m)
	return m
}

func (f *fixture) addLesson(token, moduleID, title string) []learning.Module {
	f.app.T.Helper()
	res := f.app.Do(http.MethodPost, "/api/v1/modules/"+moduleID+"/lessons", token, map[string]any{"title": title, "type": "video", "durationMinutes": 15, "content": "body of " + title})
	f.app.Expect(res, http.StatusCreated, "")
	var m []learning.Module
	res.JSON(&m)
	return m
}

func (f *fixture) enroll(token, courseID, studentID string) testkit.Response {
	return f.app.Do(http.MethodPost, "/api/v1/courses/"+courseID+"/enrollments", token, map[string]any{"studentId": studentID})
}

func (f *fixture) outline(token, courseID string) []learning.Module {
	f.app.T.Helper()
	res := f.app.Do(http.MethodGet, "/api/v1/courses/"+courseID+"/modules", token, nil)
	f.app.Expect(res, http.StatusOK, "")
	var m []learning.Module
	res.JSON(&m)
	return m
}

func statuses(modules []learning.Module) []string {
	var out []string
	for _, m := range modules {
		for _, l := range m.Lessons {
			out = append(out, l.Status)
		}
	}
	return out
}

func TestTracksAdminOnlyWritesAndStaff(t *testing.T) {
	f := setup(t)
	app := f.app

	app.Expect(app.Do(http.MethodPost, "/api/v1/tracks", "", map[string]any{"name": "X"}), http.StatusUnauthorized, "unauthorized")
	app.Expect(app.Do(http.MethodPost, "/api/v1/tracks", f.instructor, map[string]any{"name": "X"}), http.StatusForbidden, "forbidden")

	track := f.createTrack("Go Backend", "active")
	draft := f.createTrack("Secret Draft", "draft")
	app.Expect(app.Do(http.MethodPost, "/api/v1/tracks", f.admin, map[string]any{"name": "go backend"}), http.StatusConflict, "track_name_taken")
	app.Expect(app.Do(http.MethodPost, "/api/v1/tracks", f.admin, map[string]any{"name": "Y", "status": "live"}), http.StatusBadRequest, "validation_error")

	app.Expect(app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/mentors", f.admin, map[string]any{"userIds": []string{f.studentID}}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/mentors", f.mentor, map[string]any{"userIds": []string{f.mentorID}}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPut, "/api/v1/tracks/00000000-0000-0000-0000-000000000000/mentors", f.admin, map[string]any{"userIds": []string{f.mentorID}}), http.StatusNotFound, "not_found")

	res := app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/mentors", f.admin, map[string]any{"userIds": []string{f.mentorID}})
	app.Expect(res, http.StatusOK, "")
	app.Expect(app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/trackers", f.admin, map[string]any{"userIds": []string{f.trackerID}}), http.StatusOK, "")

	res = app.Do(http.MethodGet, "/api/v1/tracks/"+track.ID, f.student, nil)
	app.Expect(res, http.StatusOK, "")
	var got learning.Track
	res.JSON(&got)
	if got.MentorID == nil || *got.MentorID != f.mentorID || got.TrackerID == nil || *got.TrackerID != f.trackerID || got.Courses == nil {
		t.Fatalf("unexpected track %+v", got)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/tracks/"+draft.ID, f.student, nil), http.StatusNotFound, "not_found")
	res = app.Do(http.MethodGet, "/api/v1/tracks", f.student, nil)
	var list []learning.Track
	res.JSON(&list)
	if len(list) != 1 || res.Header().Get("X-Total-Count") != "1" {
		t.Fatalf("student should see only active tracks: %d", len(list))
	}
	res = app.Do(http.MethodGet, "/api/v1/tracks", f.admin, nil)
	res.JSON(&list)
	if len(list) != 2 {
		t.Fatalf("admin should see all tracks: %d", len(list))
	}
}

func TestCourseOwnershipAndVisibility(t *testing.T) {
	f := setup(t)
	app := f.app
	track := f.createTrack("Go Backend", "active")

	app.Expect(app.Do(http.MethodPost, "/api/v1/courses", f.student, map[string]any{"title": "Nope"}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses", f.instructor, map[string]any{"title": "Nope", "trackId": track.ID}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses", f.instructor, map[string]any{"title": "Nope", "instructorId": f.otherInstructorID}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses", f.admin, map[string]any{"title": "Bad", "instructorId": f.studentID}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses", f.admin, map[string]any{"title": "Bad", "color": "black"}), http.StatusBadRequest, "validation_error")

	own := f.createCourse(f.instructor, map[string]any{"title": "Go Fundamentals", "category": "Backend", "durationWeeks": 6, "color": "sky"})
	if own.InstructorID == nil || *own.InstructorID != f.instructorID || own.Status != "draft" || own.Duration != "6 weeks" || own.InstructorName == nil {
		t.Fatalf("unexpected course %+v", own)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+own.ID, f.other, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/courses/"+own.ID, f.other, map[string]any{"title": "Hijack"}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/courses/"+own.ID, f.instructor, map[string]any{"trackId": track.ID}), http.StatusForbidden, "forbidden")

	res := app.Do(http.MethodPatch, "/api/v1/courses/"+own.ID, f.admin, map[string]any{"trackId": track.ID, "status": "published"})
	app.Expect(res, http.StatusOK, "")
	var c learning.Course
	res.JSON(&c)
	if c.TrackID == nil || *c.TrackID != track.ID || c.PublishedAt == nil || c.TrackName == nil {
		t.Fatalf("admin update failed %+v", c)
	}

	app.Expect(f.enroll(f.instructor, own.ID, f.studentID), http.StatusCreated, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+own.ID, f.student, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/courses/"+own.ID, f.student, map[string]any{"title": "Hacked"}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+own.ID, f.outs, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/not-a-uuid", f.admin, nil), http.StatusNotFound, "not_found")

	app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/mentors", f.admin, map[string]any{"userIds": []string{f.mentorID}})
	res = app.Do(http.MethodGet, "/api/v1/courses", f.mentor, nil)
	var list []learning.Course
	res.JSON(&list)
	if len(list) != 1 || list[0].ID != own.ID || list[0].EnrolledCount != 1 {
		t.Fatalf("mentor scope: %+v", list)
	}
	res = app.Do(http.MethodGet, "/api/v1/courses", f.tracker, nil)
	res.JSON(&list)
	if len(list) != 0 {
		t.Fatalf("unassigned tracker sees courses: %d", len(list))
	}

	f.createCourse(f.other, map[string]any{"title": "Other Draft"})
	res = app.Do(http.MethodGet, "/api/v1/courses", f.instructor, nil)
	res.JSON(&list)
	if len(list) != 1 {
		t.Fatalf("instructor should only see own courses: %d", len(list))
	}
	res = app.Do(http.MethodGet, "/api/v1/courses?status=draft", f.admin, nil)
	res.JSON(&list)
	if len(list) != 1 || res.Header().Get("X-Total-Count") != "1" {
		t.Fatalf("admin status filter: %d", len(list))
	}

	app.Expect(app.Do(http.MethodPatch, "/api/v1/courses/"+own.ID, f.admin, map[string]any{"status": "draft"}), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+own.ID, f.student, nil), http.StatusNotFound, "not_found")
}

func TestSequentialUnlockAndProgress(t *testing.T) {
	f := setup(t)
	app := f.app
	course := f.createCourse(f.instructor, map[string]any{"title": "Go", "sequentialUnlock": true})
	m1 := f.addModule(f.instructor, course.ID, "Basics")[0]
	f.addModule(f.instructor, course.ID, "Empty middle")
	m3 := f.addModule(f.instructor, course.ID, "Advanced")[2]
	f.addLesson(f.instructor, m1.ID, "Intro")
	f.addLesson(f.instructor, m1.ID, "Variables")
	outline := f.addLesson(f.instructor, m3.ID, "Goroutines")
	l1, l2, l3 := outline[0].Lessons[0], outline[0].Lessons[1], outline[2].Lessons[0]

	app.Expect(app.Do(http.MethodPost, "/api/v1/modules/"+m1.ID+"/lessons", f.other, map[string]any{"title": "X"}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/courses/"+course.ID, f.instructor, map[string]any{"status": "published"}), http.StatusOK, "")
	app.Expect(f.enroll(f.instructor, course.ID, f.studentID), http.StatusCreated, "")

	got := statuses(f.outline(f.student, course.ID))
	if len(got) != 3 || got[0] != "available" || got[1] != "locked" || got[2] != "locked" {
		t.Fatalf("initial statuses %v", got)
	}
	if s := statuses(f.outline(f.instructor, course.ID)); s[1] != "available" {
		t.Fatalf("staff must not see locks: %v", s)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/lessons/"+l2.ID, f.student, nil), http.StatusForbidden, "lesson_locked")
	app.Expect(app.Do(http.MethodPost, "/api/v1/lessons/"+l2.ID+"/progress", f.student, map[string]any{"status": "completed"}), http.StatusForbidden, "lesson_locked")
	app.Expect(app.Do(http.MethodPost, "/api/v1/lessons/"+l1.ID+"/progress", f.outs, map[string]any{"status": "completed"}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPost, "/api/v1/lessons/"+l1.ID+"/progress", f.instructor, map[string]any{"status": "completed"}), http.StatusConflict, "not_enrolled")

	res := app.Do(http.MethodGet, "/api/v1/lessons/"+l1.ID, f.student, nil)
	app.Expect(res, http.StatusOK, "")
	var lesson learning.Lesson
	res.JSON(&lesson)
	if lesson.Content == nil || *lesson.Content != "body of Intro" || lesson.Duration != "15 min" {
		t.Fatalf("lesson detail %+v", lesson)
	}

	app.Expect(app.Do(http.MethodPost, "/api/v1/lessons/"+l1.ID+"/progress", f.student, map[string]any{"status": "in_progress"}), http.StatusOK, "")
	if s := statuses(f.outline(f.student, course.ID)); s[0] != "in_progress" || s[1] != "locked" {
		t.Fatalf("after start %v", s)
	}

	res = app.Do(http.MethodPost, "/api/v1/lessons/"+l1.ID+"/progress", f.student, map[string]any{"status": "completed"})
	var e learning.Enrollment
	res.JSON(&e)
	if e.Progress != 33 || e.Status != "in_progress" || e.CompletedLessons != 1 {
		t.Fatalf("after l1 %+v", e)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/lessons/"+l1.ID+"/progress", f.student, map[string]any{"status": "in_progress"}), http.StatusOK, "")
	if s := statuses(f.outline(f.student, course.ID)); s[0] != "completed" || s[1] != "available" || s[2] != "locked" {
		t.Fatalf("completion must not regress / unlock next: %v", s)
	}

	app.Do(http.MethodPost, "/api/v1/lessons/"+l2.ID+"/progress", f.student, map[string]any{"status": "completed"})
	res = app.Do(http.MethodPost, "/api/v1/lessons/"+l3.ID+"/progress", f.student, map[string]any{"status": "completed"})
	res.JSON(&e)
	if e.Progress != 100 || e.Status != "completed" || e.CompletedAt == nil {
		t.Fatalf("final enrollment %+v", e)
	}

	app.Expect(app.Do(http.MethodDelete, "/api/v1/lessons/"+l1.ID, f.instructor, nil), http.StatusConflict, "lesson_has_progress")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/modules/"+m1.ID, f.instructor, nil), http.StatusConflict, "module_not_empty")
}

func TestNonSequentialCourseIsOpen(t *testing.T) {
	f := setup(t)
	course := f.createCourse(f.admin, map[string]any{"title": "Open", "status": "published", "instructorId": f.instructorID})
	m := f.addModule(f.instructor, course.ID, "M")[0]
	f.addLesson(f.instructor, m.ID, "A")
	f.addLesson(f.instructor, m.ID, "B")
	f.app.Expect(f.enroll(f.admin, course.ID, f.studentID), http.StatusCreated, "")
	if s := statuses(f.outline(f.student, course.ID)); s[0] != "available" || s[1] != "available" {
		t.Fatalf("open course statuses %v", s)
	}
}

func TestModuleAndLessonOrdering(t *testing.T) {
	f := setup(t)
	app := f.app
	course := f.createCourse(f.instructor, map[string]any{"title": "Order"})
	f.addModule(f.instructor, course.ID, "A")
	f.addModule(f.instructor, course.ID, "B")
	mods := f.addModule(f.instructor, course.ID, "C")

	res := app.Do(http.MethodPatch, "/api/v1/modules/"+mods[2].ID, f.instructor, map[string]any{"position": 1})
	app.Expect(res, http.StatusOK, "")
	var got []learning.Module
	res.JSON(&got)
	if got[0].Title != "C" || got[1].Title != "A" || got[2].Title != "B" || got[0].Order != 1 || got[2].Order != 3 {
		t.Fatalf("reorder: %+v", got)
	}

	res = app.Do(http.MethodPost, "/api/v1/courses/"+course.ID+"/modules", f.instructor, map[string]any{"title": "Z", "position": 2})
	res.JSON(&got)
	if got[1].Title != "Z" || len(got) != 4 {
		t.Fatalf("insert at position: %+v", got)
	}

	app.Expect(app.Do(http.MethodDelete, "/api/v1/modules/"+got[1].ID, f.instructor, nil), http.StatusNoContent, "")
	got = f.outline(f.instructor, course.ID)
	for i, m := range got {
		if m.Order != i+1 {
			t.Fatalf("positions not compacted: %+v", got)
		}
	}
	app.Expect(app.Do(http.MethodDelete, "/api/v1/modules/"+got[0].ID, f.other, nil), http.StatusNotFound, "not_found")
}

func TestEnrollmentRules(t *testing.T) {
	f := setup(t)
	app := f.app
	course := f.createCourse(f.instructor, map[string]any{"title": "Go", "status": "published"})

	app.Expect(f.enroll(f.student, course.ID, f.studentID), http.StatusNotFound, "not_found")
	app.Expect(f.enroll(f.other, course.ID, f.studentID), http.StatusNotFound, "not_found")
	app.Expect(f.enroll(f.instructor, course.ID, f.mentorID), http.StatusBadRequest, "validation_error")
	app.Expect(f.enroll(f.instructor, course.ID, "not-a-uuid"), http.StatusBadRequest, "validation_error")
	app.Expect(f.enroll(f.instructor, course.ID, f.studentID), http.StatusCreated, "")
	app.Expect(f.enroll(f.instructor, course.ID, f.studentID), http.StatusConflict, "already_enrolled")

	path := "/api/v1/courses/" + course.ID + "/enrollments/" + f.studentID
	app.Expect(app.Do(http.MethodDelete, path, f.instructor, nil), http.StatusNoContent, "")
	app.Expect(app.Do(http.MethodDelete, path, f.instructor, nil), http.StatusConflict, "not_enrolled")
	app.Expect(f.enroll(f.admin, course.ID, f.studentID), http.StatusCreated, "")

	app.Expect(app.Do(http.MethodPatch, "/api/v1/courses/"+course.ID, f.instructor, map[string]any{"status": "archived"}), http.StatusOK, "")
	app.Expect(f.enroll(f.admin, course.ID, f.outsID), http.StatusConflict, "course_archived")

	var history int
	_ = app.DB.QueryRow(context.Background(), `SELECT count(*) FROM enrollments WHERE student_id = $1`, f.studentID).Scan(&history)
	if history != 2 {
		t.Fatalf("withdrawn enrollment should be kept as history, got %d rows", history)
	}
}

func TestConcurrentEnrollmentCreatesOneRecord(t *testing.T) {
	f := setup(t)
	course := f.createCourse(f.admin, map[string]any{"title": "Race", "status": "published"})

	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = f.enroll(f.admin, course.ID, f.studentID).Code
		}(i)
	}
	wg.Wait()

	created, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected status %d in %v", c, codes)
		}
	}
	if created != 1 || conflicts != n-1 {
		t.Fatalf("created=%d conflicts=%d", created, conflicts)
	}
}

func TestEnrollmentListScope(t *testing.T) {
	f := setup(t)
	app := f.app
	course := f.createCourse(f.instructor, map[string]any{"title": "Go", "status": "published"})
	otherCourse := f.createCourse(f.other, map[string]any{"title": "Rust", "status": "published"})
	f.enroll(f.instructor, course.ID, f.studentID)
	f.enroll(f.instructor, course.ID, f.outsID)
	f.enroll(f.other, otherCourse.ID, f.outsID)

	check := func(token string, want int) {
		t.Helper()
		res := app.Do(http.MethodGet, "/api/v1/enrollments", token, nil)
		app.Expect(res, http.StatusOK, "")
		var list []learning.Enrollment
		res.JSON(&list)
		if len(list) != want {
			t.Fatalf("want %d enrollments, got %d", want, len(list))
		}
	}
	check(f.student, 1)
	check(f.outs, 2)
	check(f.instructor, 2)
	check(f.other, 1)
	check(f.mentor, 0)
	check(f.admin, 3)

	res := app.Do(http.MethodGet, "/api/v1/enrollments?courseId="+course.ID+"&limit=1", f.admin, nil)
	if res.Header().Get("X-Total-Count") != "2" {
		t.Fatalf("total header %q", res.Header().Get("X-Total-Count"))
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/enrollments?status=bogus", f.admin, nil), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodGet, "/api/v1/enrollments", "", nil), http.StatusUnauthorized, "unauthorized")
}

func TestProfileExtrasAndScopedPeople(t *testing.T) {
	f := setup(t)
	app := f.app
	track := f.createTrack("Go Backend", "active")
	course := f.createCourse(f.admin, map[string]any{"title": "Go", "status": "published", "trackId": track.ID, "instructorId": f.instructorID})
	app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/mentors", f.admin, map[string]any{"userIds": []string{f.mentorID}})
	f.enroll(f.admin, course.ID, f.studentID)

	var me struct {
		Role            string   `json:"role"`
		EnrolledCourses []string `json:"enrolledCourses"`
		Track           *string  `json:"track"`
		Courses         []string `json:"courses"`
		Tracks          []string `json:"tracks"`
	}
	res := app.Do(http.MethodGet, "/api/v1/auth/me", f.student, nil)
	res.JSON(&me)
	if len(me.EnrolledCourses) != 1 || me.EnrolledCourses[0] != course.ID || me.Track == nil || *me.Track != "Go Backend" {
		t.Fatalf("student extras %+v", me)
	}

	res = app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "outsider@eduflow.dev", "password": testkit.Password})
	if body := res.Body.String(); !contains(body, `"enrolledCourses":[]`) {
		t.Fatalf("student without courses must get an empty array: %s", body)
	}

	res = app.Do(http.MethodGet, "/api/v1/users/me", f.instructor, nil)
	res.JSON(&me)
	if len(me.Courses) != 1 || me.Courses[0] != course.ID {
		t.Fatalf("instructor extras %+v", me)
	}
	res = app.Do(http.MethodGet, "/api/v1/auth/me", f.mentor, nil)
	res.JSON(&me)
	if len(me.Tracks) != 1 || me.Tracks[0] != track.ID {
		t.Fatalf("mentor extras %+v", me)
	}

	res = app.Do(http.MethodGet, "/api/v1/users", f.mentor, nil)
	app.Expect(res, http.StatusOK, "")
	var people []struct {
		ID string `json:"id"`
	}
	res.JSON(&people)
	seen := map[string]bool{}
	for _, p := range people {
		seen[p.ID] = true
	}
	if !seen[f.studentID] || !seen[f.instructorID] || !seen[f.mentorID] || seen[f.outsID] || seen[f.trackerID] {
		t.Fatalf("mentor people scope wrong: %v", seen)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/users/"+f.studentID, f.mentor, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/users/"+f.outsID, f.mentor, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/users/"+f.outsID, f.student, nil), http.StatusForbidden, "forbidden")
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
