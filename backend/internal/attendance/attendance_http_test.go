package attendance_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/attendance"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
	"github.com/Akrambahy/EduFlow/backend/internal/sessions"
)

type env struct {
	app                                         *testkit.App
	admin, instructor, other, mentor            string
	student, student2, outsider                 string
	studentID, student2ID, outsiderID, mentorID string
	course                                      learning.Course
}

func setup(t *testing.T) *env {
	app := testkit.NewApp(t)
	e := &env{app: app}
	_, e.admin = app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	_, e.instructor = app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")
	_, e.other = app.UserWithToken("Other", "other@eduflow.dev", "instructor")
	e.mentorID, e.mentor = app.UserWithToken("Mentor", "mentor@eduflow.dev", "mentor")
	e.studentID, e.student = app.UserWithToken("Alice", "alice@eduflow.dev", "student")
	e.student2ID, e.student2 = app.UserWithToken("Bob", "bob@eduflow.dev", "student")
	e.outsiderID, e.outsider = app.UserWithToken("Zed", "zed@eduflow.dev", "student")

	var track learning.Track
	res := app.Do(http.MethodPost, "/api/v1/tracks", e.admin, map[string]any{"name": "Backend", "status": "active"})
	res.JSON(&track)
	app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/mentors", e.admin, map[string]any{"userIds": []string{e.mentorID}})

	res = app.Do(http.MethodPost, "/api/v1/courses", e.admin, map[string]any{"title": "Go", "status": "published", "trackId": track.ID, "instructorId": userID(t, app, "inst@eduflow.dev")})
	app.Expect(res, http.StatusCreated, "")
	res.JSON(&e.course)
	for _, id := range []string{e.studentID, e.student2ID} {
		app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/enrollments", e.instructor, map[string]any{"studentId": id}), http.StatusCreated, "")
	}
	return e
}

func userID(t *testing.T, app *testkit.App, email string) string {
	var id string
	if err := app.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE email = $1`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *env) session(startOffset time.Duration) sessions.Session {
	start := time.Now().Add(startOffset).UTC()
	res := e.app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/sessions", e.instructor, map[string]any{
		"title": "Lecture", "startsAt": start.Format(time.RFC3339), "endsAt": start.Add(2 * time.Hour).Format(time.RFC3339),
	})
	e.app.Expect(res, http.StatusCreated, "")
	var s sessions.Session
	res.JSON(&s)
	return s
}

func (e *env) start(sessionID string) attendance.Window {
	res := e.app.Do(http.MethodPost, "/api/v1/sessions/"+sessionID+"/attendance/start", e.instructor, nil)
	e.app.Expect(res, http.StatusOK, "")
	var w attendance.Window
	res.JSON(&w)
	return w
}

func (e *env) checkIn(token, value string) testkit.Response {
	return e.app.Do(http.MethodPost, "/api/v1/attendance/check-in", token, map[string]any{"token": value})
}

func TestAttendanceLifecycle(t *testing.T) {
	e := setup(t)
	app := e.app
	s := e.session(-2 * time.Minute)
	base := "/api/v1/sessions/" + s.ID + "/attendance"

	app.Expect(app.Do(http.MethodPost, base+"/start", "", nil), http.StatusUnauthorized, "unauthorized")
	app.Expect(app.Do(http.MethodPost, base+"/start", e.student, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, base+"/start", e.other, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPost, base+"/start", e.mentor, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, base+"/token", e.instructor, nil), http.StatusConflict, "attendance_not_started")

	w := e.start(s.ID)
	if w.Status != "open" || w.Token == nil || w.TokenExpiresAt == nil || w.GraceMinutes != 10 || w.Summary.Enrolled != 2 || w.Summary.Pending != 2 {
		t.Fatalf("unexpected window %+v", w)
	}
	again := e.start(s.ID)
	if again.ID != w.ID {
		t.Fatal("starting twice must reuse the window")
	}

	app.Expect(e.checkIn(e.instructor, *w.Token), http.StatusForbidden, "forbidden")
	app.Expect(e.checkIn(e.outsider, *w.Token), http.StatusForbidden, "not_enrolled")
	app.Expect(e.checkIn(e.student, "garbage"), http.StatusBadRequest, "attendance_token_invalid")

	res := e.checkIn(e.student, *w.Token)
	app.Expect(res, http.StatusCreated, "")
	var rec attendance.Record
	res.JSON(&rec)
	if rec.Status != "present" || rec.Method != "qr" || rec.CheckedInAt == nil || rec.StudentID != e.studentID {
		t.Fatalf("unexpected record %+v", rec)
	}
	app.Expect(e.checkIn(e.student, *w.Token), http.StatusConflict, "already_checked_in")

	res = app.Do(http.MethodGet, base, e.mentor, nil)
	app.Expect(res, http.StatusOK, "")
	var view attendance.Window
	res.JSON(&view)
	if view.Token != nil || view.Summary.Present != 1 || view.Summary.Pending != 1 {
		t.Fatalf("staff view must not expose the token: %+v", view)
	}
	app.Expect(app.Do(http.MethodGet, base, e.student, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, base+"/records", e.outsider, nil), http.StatusNotFound, "not_found")

	app.Expect(app.Do(http.MethodPost, base+"/close", e.student, nil), http.StatusForbidden, "forbidden")
	res = app.Do(http.MethodPost, base+"/close", e.instructor, nil)
	app.Expect(res, http.StatusOK, "")
	res.JSON(&view)
	if view.Status != "closed" || view.Summary.Present != 1 || view.Summary.Absent != 1 || view.Summary.Pending != 0 {
		t.Fatalf("after close %+v", view)
	}
	app.Expect(app.Do(http.MethodPost, base+"/close", e.instructor, nil), http.StatusOK, "")
	app.Expect(e.checkIn(e.student2, *w.Token), http.StatusConflict, "attendance_closed")
	app.Expect(app.Do(http.MethodGet, base+"/token", e.instructor, nil), http.StatusConflict, "attendance_closed")
	app.Expect(app.Do(http.MethodPost, base+"/start", e.instructor, nil), http.StatusConflict, "attendance_closed")

	res = app.Do(http.MethodGet, base+"/records?status=absent", e.instructor, nil)
	var records []attendance.Record
	res.JSON(&records)
	if len(records) != 1 || records[0].StudentID != e.student2ID || records[0].Method != "auto" {
		t.Fatalf("absent records %+v", records)
	}

	path := "/api/v1/attendance/records/" + records[0].ID
	app.Expect(app.Do(http.MethodPatch, path, e.instructor, map[string]any{"status": "excused"}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPatch, path, e.mentor, map[string]any{"status": "excused", "reason": "medical note"}), http.StatusForbidden, "forbidden")
	res = app.Do(http.MethodPatch, path, e.instructor, map[string]any{"status": "excused", "reason": "medical note"})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&rec)
	if rec.Status != "excused" || rec.Reason == nil || rec.Method != "manual" {
		t.Fatalf("corrected record %+v", rec)
	}
	var audited int
	_ = app.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'attendance.corrected' AND target_id = $1
		AND context->>'before' = 'absent' AND context->>'after' = 'excused'`, rec.ID).Scan(&audited)
	if audited != 1 {
		t.Fatal("correction must be audited with before/after")
	}

	res = app.Do(http.MethodGet, "/api/v1/students/"+e.student2ID+"/attendance/summary", e.student2, nil)
	var sum attendance.StudentSummary
	res.JSON(&sum)
	if sum.TotalSessions != 1 || sum.Excused != 1 || sum.Rate != 100 {
		t.Fatalf("summary %+v", sum)
	}
	res = app.Do(http.MethodGet, "/api/v1/students/"+e.studentID+"/attendance/summary", e.mentor, nil)
	res.JSON(&sum)
	if sum.Present != 1 || sum.Rate != 100 {
		t.Fatalf("mentor view of student summary %+v", sum)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/students/"+e.studentID+"/attendance/summary", e.student2, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/students/"+e.studentID+"/attendance", e.other, nil), http.StatusForbidden, "forbidden")
	res = app.Do(http.MethodGet, "/api/v1/students/"+e.studentID+"/attendance", e.student, nil)
	res.JSON(&records)
	if len(records) != 1 || res.Header().Get("X-Total-Count") != "1" {
		t.Fatalf("student records %+v", records)
	}
}

func TestLateCheckIn(t *testing.T) {
	e := setup(t)
	s := e.session(-30 * time.Minute)
	w := e.start(s.ID)
	res := e.checkIn(e.student, *w.Token)
	e.app.Expect(res, http.StatusCreated, "")
	var rec attendance.Record
	res.JSON(&rec)
	if rec.Status != "late" {
		t.Fatalf("expected late, got %s", rec.Status)
	}

	s2 := e.session(-30 * time.Minute)
	res = e.app.Do(http.MethodPost, "/api/v1/sessions/"+s2.ID+"/attendance/start", e.instructor, map[string]any{"graceMinutes": 45})
	var w2 attendance.Window
	res.JSON(&w2)
	res = e.checkIn(e.student, *w2.Token)
	res.JSON(&rec)
	if rec.Status != "present" || w2.GraceMinutes != 45 {
		t.Fatalf("custom grace: %s %d", rec.Status, w2.GraceMinutes)
	}
}

func TestExpiredAndForgedTokens(t *testing.T) {
	e := setup(t)
	s := e.session(0)
	w := e.start(s.ID)

	var secret []byte
	if err := e.app.DB.QueryRow(context.Background(), `SELECT token_secret FROM attendance_sessions WHERE id = $1`, w.ID).Scan(&secret); err != nil {
		t.Fatal(err)
	}
	old, err := attendance.IssueToken(w.ID, secret, 30*time.Second, time.Now().Add(-2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	e.app.Expect(e.checkIn(e.student, old.Value), http.StatusBadRequest, "attendance_token_expired")

	forged, _ := attendance.IssueToken(w.ID, []byte("not-the-real-secret-not-the-real"), 30*time.Second, time.Now())
	e.app.Expect(e.checkIn(e.student, forged.Value), http.StatusBadRequest, "attendance_token_invalid")

	unknown, _ := attendance.IssueToken("00000000-0000-0000-0000-000000000000", secret, 30*time.Second, time.Now())
	e.app.Expect(e.checkIn(e.student, unknown.Value), http.StatusBadRequest, "attendance_token_invalid")

	fresh := e.app.Do(http.MethodGet, "/api/v1/sessions/"+s.ID+"/attendance/token", e.instructor, nil)
	var w2 attendance.Window
	fresh.JSON(&w2)
	e.app.Expect(e.checkIn(e.student, *w2.Token), http.StatusCreated, "")
}

func TestUnenrolledAfterQRAndCancelledSession(t *testing.T) {
	e := setup(t)
	s := e.session(0)
	w := e.start(s.ID)
	e.app.Expect(e.app.Do(http.MethodDelete, "/api/v1/courses/"+e.course.ID+"/enrollments/"+e.student2ID, e.instructor, nil), http.StatusNoContent, "")
	e.app.Expect(e.checkIn(e.student2, *w.Token), http.StatusForbidden, "not_enrolled")

	e.app.Expect(e.app.Do(http.MethodPatch, "/api/v1/sessions/"+s.ID, e.instructor, map[string]any{"cancelled": true}), http.StatusOK, "")
	e.app.Expect(e.checkIn(e.student, *w.Token), http.StatusConflict, "session_cancelled")

	s2 := e.session(time.Hour)
	e.app.Do(http.MethodPatch, "/api/v1/sessions/"+s2.ID, e.instructor, map[string]any{"cancelled": true})
	e.app.Expect(e.app.Do(http.MethodPost, "/api/v1/sessions/"+s2.ID+"/attendance/start", e.instructor, nil), http.StatusConflict, "session_cancelled")
}

func TestManualAttendance(t *testing.T) {
	e := setup(t)
	s := e.session(0)
	base := "/api/v1/sessions/" + s.ID + "/attendance/records"
	body := map[string]any{"studentId": e.studentID, "status": "present", "reason": "phone battery died"}

	e.app.Expect(e.app.Do(http.MethodPost, base, e.instructor, body), http.StatusConflict, "attendance_not_started")
	w := e.start(s.ID)
	e.app.Expect(e.app.Do(http.MethodPost, base, e.student, body), http.StatusForbidden, "forbidden")
	e.app.Expect(e.app.Do(http.MethodPost, base, e.instructor, map[string]any{"studentId": e.outsiderID, "status": "present", "reason": "walked in"}), http.StatusBadRequest, "validation_error")
	res := e.app.Do(http.MethodPost, base, e.instructor, body)
	e.app.Expect(res, http.StatusCreated, "")
	var rec attendance.Record
	res.JSON(&rec)
	if rec.Method != "manual" || rec.Status != "present" {
		t.Fatalf("manual record %+v", rec)
	}
	e.app.Expect(e.app.Do(http.MethodPost, base, e.instructor, body), http.StatusConflict, "already_checked_in")
	e.app.Expect(e.checkIn(e.student, *w.Token), http.StatusConflict, "already_checked_in")
}

func TestConcurrentCheckInsCreateOneRecord(t *testing.T) {
	e := setup(t)
	s := e.session(0)
	w := e.start(s.ID)

	const n = 10
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = e.checkIn(e.student, *w.Token).Code
		}(i)
	}
	wg.Wait()
	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Fatalf("unexpected status %d in %v", c, codes)
		}
	}
	var rows int
	_ = e.app.DB.QueryRow(context.Background(), `SELECT count(*) FROM attendance_records WHERE student_id = $1`, e.studentID).Scan(&rows)
	if created != 1 || rows != 1 {
		t.Fatalf("created=%d rows=%d", created, rows)
	}
}
