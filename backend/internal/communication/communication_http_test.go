package communication_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/assessment"
	"github.com/Akrambahy/EduFlow/backend/internal/communication"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
)

type env struct {
	app                                         *testkit.App
	admin, instructor, mentor, alice, outsider  string
	aliceID, outsiderID, mentorID, instructorID string
	course                                      learning.Course
}

func setup(t *testing.T) *env {
	app := testkit.NewApp(t)
	e := &env{app: app}
	_, e.admin = app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	e.instructorID, e.instructor = app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")
	e.mentorID, e.mentor = app.UserWithToken("Mentor", "mentor@eduflow.dev", "mentor")
	e.aliceID, e.alice = app.UserWithToken("Alice", "alice@eduflow.dev", "student")
	e.outsiderID, e.outsider = app.UserWithToken("Zed", "zed@eduflow.dev", "student")
	var track learning.Track
	app.Do(http.MethodPost, "/api/v1/tracks", e.admin, map[string]any{"name": "Backend", "status": "active"}).JSON(&track)
	app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/mentors", e.admin, map[string]any{"userIds": []string{e.mentorID}})
	app.Do(http.MethodPost, "/api/v1/courses", e.admin, map[string]any{"title": "Go", "status": "published", "trackId": track.ID, "instructorId": e.instructorID}).JSON(&e.course)
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/enrollments", e.instructor, map[string]any{"studentId": e.aliceID}), http.StatusCreated, "")
	return e
}

func (e *env) notifications(token string) []communication.Notification {
	var list []communication.Notification
	e.app.Do(http.MethodGet, "/api/v1/notifications?limit=100", token, nil).JSON(&list)
	return list
}

func countType(list []communication.Notification, typ string) int {
	n := 0
	for _, x := range list {
		if x.Type == typ {
			n++
		}
	}
	return n
}

func TestEventNotifications(t *testing.T) {
	e := setup(t)
	app := e.app
	start := time.Now().Add(24 * time.Hour).UTC()
	var session struct {
		ID string `json:"id"`
	}
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/sessions", e.instructor, map[string]any{
		"title": "Lecture 1", "startsAt": start.Format(time.RFC3339), "endsAt": start.Add(time.Hour).Format(time.RFC3339),
	}).JSON(&session)
	app.Do(http.MethodPatch, "/api/v1/sessions/"+session.ID, e.instructor, map[string]any{"cancelled": true})

	var a assessment.Assignment
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/assignments", e.instructor, map[string]any{
		"title": "A1", "status": "published", "dueAt": start.Format(time.RFC3339), "submissionTypes": []string{"text"},
	}).JSON(&a)
	app.Do(http.MethodPatch, "/api/v1/assignments/"+a.ID, e.instructor, map[string]any{"status": "published"})
	var sub assessment.Submission
	app.Do(http.MethodPost, "/api/v1/assignments/"+a.ID+"/submissions", e.alice, map[string]any{"text": "x"}).JSON(&sub)
	app.Expect(app.Do(http.MethodPost, "/api/v1/submissions/"+sub.ID+"/grade", e.instructor, map[string]any{"grade": 90}), http.StatusOK, "")

	list := e.notifications(e.alice)
	if countType(list, "session") != 2 || countType(list, "assignment") != 1 || countType(list, "grade") != 1 {
		t.Fatalf("alice notifications %+v", list)
	}
	if len(e.notifications(e.outsider)) != 0 {
		t.Fatal("outsider must not be notified")
	}

	res := app.Do(http.MethodGet, "/api/v1/notifications?unread=true", e.alice, nil)
	if res.Header().Get("X-Unread-Count") != "4" || res.Header().Get("X-Total-Count") != "4" {
		t.Fatalf("headers %v", res.Header())
	}
	app.Expect(app.Do(http.MethodPatch, "/api/v1/notifications/"+list[0].ID+"/read", e.outsider, nil), http.StatusNotFound, "not_found")
	res = app.Do(http.MethodPatch, "/api/v1/notifications/"+list[0].ID+"/read", e.alice, nil)
	app.Expect(res, http.StatusOK, "")
	var n communication.Notification
	res.JSON(&n)
	if !n.Read {
		t.Fatal("notification should be read")
	}
	var out struct {
		Updated int `json:"updated"`
	}
	app.Do(http.MethodPost, "/api/v1/notifications/read-all", e.alice, nil).JSON(&out)
	if out.Updated != 3 {
		t.Fatalf("read-all updated %d", out.Updated)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/notifications", "", nil), http.StatusUnauthorized, "unauthorized")
}

func TestAnnouncements(t *testing.T) {
	e := setup(t)
	app := e.app
	app.Expect(app.Do(http.MethodPost, "/api/v1/announcements", e.alice, map[string]any{"title": "Hi", "content": "x"}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/announcements", e.instructor, map[string]any{"title": "Hi", "content": "x"}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/announcements", e.admin, map[string]any{"title": "Hi", "content": "x", "targetRoles": []string{"boss"}}), http.StatusBadRequest, "validation_error")

	res := app.Do(http.MethodPost, "/api/v1/announcements", e.instructor, map[string]any{"title": "Room change", "content": "Lab 2", "courseId": e.course.ID})
	app.Expect(res, http.StatusCreated, "")
	var courseAnn communication.Announcement
	res.JSON(&courseAnn)
	app.Expect(app.Do(http.MethodPost, "/api/v1/announcements", e.admin, map[string]any{"title": "Holiday", "content": "Office closed", "targetRoles": []string{"student"}}), http.StatusCreated, "")

	var list []communication.Announcement
	app.Do(http.MethodGet, "/api/v1/announcements", e.alice, nil).JSON(&list)
	if len(list) != 2 {
		t.Fatalf("alice announcements %d", len(list))
	}
	app.Do(http.MethodGet, "/api/v1/announcements", e.outsider, nil).JSON(&list)
	if len(list) != 1 || list[0].Title != "Holiday" {
		t.Fatalf("outsider sees only the global one: %+v", list)
	}
	app.Do(http.MethodGet, "/api/v1/announcements", e.mentor, nil).JSON(&list)
	if len(list) != 1 || list[0].Title != "Room change" {
		t.Fatalf("mentor sees course announcement only: %+v", list)
	}

	if countType(e.notifications(e.alice), "announcement") != 2 || countType(e.notifications(e.outsider), "announcement") != 1 ||
		countType(e.notifications(e.mentor), "announcement") != 1 || countType(e.notifications(e.instructor), "announcement") != 0 {
		t.Fatal("announcement fan-out mismatch")
	}

	app.Expect(app.Do(http.MethodDelete, "/api/v1/announcements/"+courseAnn.ID, e.alice, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/announcements/"+courseAnn.ID, e.outsider, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/announcements/"+courseAnn.ID, e.instructor, nil), http.StatusNoContent, "")
	app.Do(http.MethodGet, "/api/v1/announcements", e.alice, nil).JSON(&list)
	if len(list) != 1 {
		t.Fatalf("deleted announcement still visible: %d", len(list))
	}
}

func TestDashboards(t *testing.T) {
	e := setup(t)
	app := e.app
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/assignments", e.instructor, map[string]any{
		"title": "A1", "status": "published", "dueAt": time.Now().Add(48 * time.Hour).Format(time.RFC3339), "submissionTypes": []string{"text"},
	})
	start := time.Now().Add(2 * time.Hour).UTC()
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/sessions", e.instructor, map[string]any{
		"title": "Lecture", "startsAt": start.Format(time.RFC3339), "endsAt": start.Add(time.Hour).Format(time.RFC3339),
	})

	var student map[string]any
	res := app.Do(http.MethodGet, "/api/v1/dashboard", e.alice, nil)
	app.Expect(res, http.StatusOK, "")
	res.JSON(&student)
	if student["role"] != "student" || student["activeCourses"].(float64) != 1 || student["pendingCount"].(float64) != 1 || student["nextSession"] == nil {
		t.Fatalf("student dashboard %+v", student)
	}

	var outsider map[string]any
	app.Do(http.MethodGet, "/api/v1/dashboard", e.outsider, nil).JSON(&outsider)
	if outsider["activeCourses"].(float64) != 0 || outsider["nextSession"] != nil {
		t.Fatalf("outsider dashboard leaks data %+v", outsider)
	}

	var inst map[string]any
	app.Do(http.MethodGet, "/api/v1/dashboard", e.instructor, nil).JSON(&inst)
	if inst["assignedCourses"].(float64) != 1 || inst["students"].(float64) != 1 {
		t.Fatalf("instructor dashboard %+v", inst)
	}
	var mentor map[string]any
	app.Do(http.MethodGet, "/api/v1/dashboard", e.mentor, nil).JSON(&mentor)
	if mentor["tracks"].(float64) != 1 || mentor["learners"].(float64) != 1 {
		t.Fatalf("mentor dashboard %+v", mentor)
	}
	var admin map[string]any
	app.Do(http.MethodGet, "/api/v1/dashboard", e.admin, nil).JSON(&admin)
	if admin["publishedCourses"].(float64) != 1 || admin["usersByRole"] == nil {
		t.Fatalf("admin dashboard %+v", admin)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/dashboard?role=admin", e.alice, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/dashboard?role=king", e.alice, nil), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodGet, "/api/v1/dashboard", "", nil), http.StatusUnauthorized, "unauthorized")

	var dup int
	_ = app.DB.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'assignment'`, e.aliceID).Scan(&dup)
	if dup != 1 {
		t.Fatalf("assignment notifications %d", dup)
	}
}
