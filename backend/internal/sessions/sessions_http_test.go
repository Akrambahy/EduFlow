package sessions_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/files"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
	"github.com/Akrambahy/EduFlow/backend/internal/sessions"
)

type env struct {
	app                               *testkit.App
	admin, instructor, other, student string
	outsider                          string
	studentID, outsiderID             string
	course                            learning.Course
}

var pdf = []byte("%PDF-1.7\n1 0 obj << >> endobj\ntrailer\n%%EOF\n")

func setup(t *testing.T) *env {
	app := testkit.NewApp(t)
	e := &env{app: app}
	_, e.admin = app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	_, e.instructor = app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")
	_, e.other = app.UserWithToken("Other", "other@eduflow.dev", "instructor")
	e.studentID, e.student = app.UserWithToken("Stu", "stu@eduflow.dev", "student")
	e.outsiderID, e.outsider = app.UserWithToken("Out", "out@eduflow.dev", "student")

	res := app.Do(http.MethodPost, "/api/v1/courses", e.instructor, map[string]any{"title": "Go", "status": "published"})
	app.Expect(res, http.StatusCreated, "")
	res.JSON(&e.course)
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/enrollments", e.instructor, map[string]any{"studentId": e.studentID}), http.StatusCreated, "")
	return e
}

func (e *env) createSession(token string, start time.Time, hours int) testkit.Response {
	return e.app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/sessions", token, map[string]any{
		"title": "Lecture", "type": "lecture", "location": "Hall A",
		"startsAt": start.Format(time.RFC3339), "endsAt": start.Add(time.Duration(hours) * time.Hour).Format(time.RFC3339),
	})
}

func TestSessionScheduling(t *testing.T) {
	e := setup(t)
	app := e.app
	future := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Minute)

	app.Expect(e.createSession("", future, 2), http.StatusUnauthorized, "unauthorized")
	app.Expect(e.createSession(e.student, future, 2), http.StatusForbidden, "forbidden")
	app.Expect(e.createSession(e.other, future, 2), http.StatusNotFound, "not_found")
	app.Expect(e.createSession(e.instructor, future, 0), http.StatusBadRequest, "validation_error")
	app.Expect(e.createSession(e.instructor, future, 13), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/sessions", e.instructor, map[string]any{"title": "x"}), http.StatusBadRequest, "validation_error")

	res := e.createSession(e.instructor, future, 2)
	app.Expect(res, http.StatusCreated, "")
	var s sessions.Session
	res.JSON(&s)
	if s.Status != "upcoming" || s.Date != future.Format("2006-01-02") || s.StartTime != future.Format("15:04") || s.InstructorName == nil || s.CourseTitle != "Go" {
		t.Fatalf("unexpected session %+v", s)
	}

	past := time.Now().Add(-72 * time.Hour).UTC()
	app.Expect(e.createSession(e.instructor, past, 1), http.StatusCreated, "")

	res = app.Do(http.MethodGet, "/api/v1/sessions?status=upcoming", e.student, nil)
	var list []sessions.Session
	res.JSON(&list)
	if len(list) != 1 || list[0].ID != s.ID {
		t.Fatalf("student upcoming: %+v", list)
	}
	res = app.Do(http.MethodGet, "/api/v1/sessions?status=completed", e.student, nil)
	res.JSON(&list)
	if len(list) != 1 || list[0].Status != "completed" {
		t.Fatalf("student completed: %+v", list)
	}
	res = app.Do(http.MethodGet, "/api/v1/sessions", e.outsider, nil)
	res.JSON(&list)
	if len(list) != 0 || res.Header().Get("X-Total-Count") != "0" {
		t.Fatalf("outsider must not see sessions: %d", len(list))
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/sessions/"+s.ID, e.outsider, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodGet, "/api/v1/sessions?from=yesterday", e.student, nil), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodGet, "/api/v1/sessions?status=live", e.student, nil), http.StatusBadRequest, "validation_error")

	res = app.Do(http.MethodGet, "/api/v1/sessions?from="+time.Now().Format("2006-01-02")+"&courseId="+e.course.ID, e.admin, nil)
	res.JSON(&list)
	if len(list) != 1 {
		t.Fatalf("from filter: %d", len(list))
	}

	app.Expect(app.Do(http.MethodPatch, "/api/v1/sessions/"+s.ID, e.student, map[string]any{"title": "Hacked"}), http.StatusForbidden, "forbidden")
	res = app.Do(http.MethodPatch, "/api/v1/sessions/"+s.ID, e.instructor, map[string]any{"cancelled": true, "recordingUrl": "https://videos.example.com/1"})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&s)
	if s.Status != "cancelled" || s.RecordingURL == nil {
		t.Fatalf("cancel: %+v", s)
	}
	app.Expect(app.Do(http.MethodPatch, "/api/v1/sessions/"+s.ID, e.instructor, map[string]any{"endsAt": s.StartsAt.Add(-time.Hour).Format(time.RFC3339)}), http.StatusBadRequest, "validation_error")
}

func TestFileUploadValidationAndAccess(t *testing.T) {
	e := setup(t)
	app := e.app

	app.Expect(app.Upload("", "notes.pdf", pdf), http.StatusUnauthorized, "unauthorized")
	app.Expect(app.Upload(e.instructor, "virus.exe", []byte("MZ\x90\x00")), http.StatusUnsupportedMediaType, "unsupported_file_type")
	app.Expect(app.Upload(e.instructor, "fake.pdf", []byte("<html><script>alert(1)</script>")), http.StatusUnsupportedMediaType, "unsupported_file_type")
	app.Expect(app.Upload(e.instructor, "empty.pdf", nil), http.StatusBadRequest, "validation_error")
	big := append(append([]byte{}, pdf...), bytes.Repeat([]byte("x"), 2<<20)...)
	app.Expect(app.Upload(e.instructor, "big.pdf", big), http.StatusRequestEntityTooLarge, "file_too_large")

	res := app.Upload(e.instructor, `..\..\Week 1 "slides".pdf`, pdf)
	app.Expect(res, http.StatusCreated, "")
	var f files.File
	res.JSON(&f)
	if f.Name != "Week 1 slides.pdf" || f.Size != int64(len(pdf)) || f.ContentType != "application/pdf" {
		t.Fatalf("unexpected file %+v", f)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.student, nil), http.StatusNotFound, "not_found")

	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/materials", e.instructor, map[string]any{"title": "Both", "url": "https://x.dev", "fileId": f.ID}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/materials", e.instructor, map[string]any{"title": "None"}), http.StatusBadRequest, "validation_error")

	res = app.Upload(e.other, "theirs.pdf", pdf)
	var theirs files.File
	res.JSON(&theirs)
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/materials", e.instructor, map[string]any{"title": "Stolen", "fileId": theirs.ID}), http.StatusBadRequest, "validation_error")

	res = app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/materials", e.instructor, map[string]any{"title": "Slides", "fileId": f.ID})
	app.Expect(res, http.StatusCreated, "")
	var m sessions.Material
	res.JSON(&m)
	if m.Kind != "file" || m.File == nil || m.File.ID != f.ID {
		t.Fatalf("material %+v", m)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/materials", e.instructor, map[string]any{"title": "Docs", "url": "https://go.dev/doc"}), http.StatusCreated, "")

	res = app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.student, nil)
	app.Expect(res, http.StatusOK, "")
	if !bytes.Equal(res.Body.Bytes(), pdf) {
		t.Fatal("downloaded content differs")
	}
	if !strings.HasPrefix(res.Header().Get("Content-Disposition"), "attachment") || res.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe download headers: %v", res.Header())
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.outsider, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID, e.outsider, nil), http.StatusNotFound, "not_found")

	res = app.Do(http.MethodGet, "/api/v1/courses/"+e.course.ID+"/materials", e.student, nil)
	var list []sessions.Material
	res.JSON(&list)
	if len(list) != 2 || res.Header().Get("X-Total-Count") != "2" {
		t.Fatalf("materials: %d", len(list))
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+e.course.ID+"/materials", e.outsider, nil), http.StatusNotFound, "not_found")

	app.Expect(app.Do(http.MethodDelete, "/api/v1/files/"+f.ID, e.instructor, nil), http.StatusConflict, "file_in_use")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/files/"+f.ID, e.student, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/materials/"+m.ID, e.student, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/materials/"+m.ID, e.instructor, nil), http.StatusNoContent, "")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/files/"+f.ID, e.instructor, nil), http.StatusNoContent, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID, e.instructor, nil), http.StatusNotFound, "not_found")
}

func TestRecordingFileAccess(t *testing.T) {
	e := setup(t)
	app := e.app
	res := e.createSession(e.instructor, time.Now().Add(-3*time.Hour), 1)
	var s sessions.Session
	res.JSON(&s)

	res = app.Upload(e.instructor, "recording.zip", []byte("PK\x03\x04content"))
	var f files.File
	res.JSON(&f)

	app.Expect(app.Do(http.MethodPatch, "/api/v1/sessions/"+s.ID, e.instructor, map[string]any{"recordingFileId": "not-a-uuid"}), http.StatusBadRequest, "validation_error")
	res = app.Do(http.MethodPatch, "/api/v1/sessions/"+s.ID, e.instructor, map[string]any{"recordingFileId": f.ID})
	app.Expect(res, http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.student, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.outsider, nil), http.StatusNotFound, "not_found")

	res = app.Do(http.MethodPatch, "/api/v1/sessions/"+s.ID, e.instructor, map[string]any{"recordingFileId": ""})
	res.JSON(&s)
	if s.RecordingFileID != nil {
		t.Fatal("recording file should be cleared")
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.student, nil), http.StatusNotFound, "not_found")
}
