package performance_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/assessment"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/performance"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
)

type env struct {
	app                               *testkit.App
	admin, instructor, other, tracker string
	alice, bob, outsider              string
	aliceID, bobID, trackerID         string
	course                            learning.Course
}

func setup(t *testing.T) *env {
	app := testkit.NewApp(t)
	e := &env{app: app}
	_, e.admin = app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	instructorID, instructor := app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")
	e.instructor = instructor
	_, e.other = app.UserWithToken("Other", "other@eduflow.dev", "instructor")
	e.trackerID, e.tracker = app.UserWithToken("Tracker", "tracker@eduflow.dev", "tracker")
	e.aliceID, e.alice = app.UserWithToken("Alice", "alice@eduflow.dev", "student")
	e.bobID, e.bob = app.UserWithToken("Bob", "bob@eduflow.dev", "student")
	_, e.outsider = app.UserWithToken("Zed", "zed@eduflow.dev", "student")

	var track learning.Track
	app.Do(http.MethodPost, "/api/v1/tracks", e.admin, map[string]any{"name": "Backend", "status": "active"}).JSON(&track)
	app.Do(http.MethodPut, "/api/v1/tracks/"+track.ID+"/trackers", e.admin, map[string]any{"userIds": []string{e.trackerID}})
	app.Do(http.MethodPost, "/api/v1/courses", e.admin, map[string]any{"title": "Go", "status": "published", "trackId": track.ID, "instructorId": instructorID}).JSON(&e.course)
	for _, id := range []string{e.aliceID, e.bobID} {
		app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/enrollments", e.instructor, map[string]any{"studentId": id}), http.StatusCreated, "")
	}
	return e
}

func (e *env) seedWork(t *testing.T) {
	app := e.app
	var quiz assessment.Quiz
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/quizzes", e.instructor, map[string]any{"title": "Q1", "maxAttempts": 2}).JSON(&quiz)
	var qs []assessment.Question
	app.Do(http.MethodPut, "/api/v1/quizzes/"+quiz.ID+"/questions", e.instructor, map[string]any{"questions": []map[string]any{
		{"text": "a", "options": []map[string]any{{"text": "x", "correct": true}, {"text": "y"}}},
		{"text": "b", "options": []map[string]any{{"text": "x", "correct": true}, {"text": "y"}}},
	}}).JSON(&qs)
	app.Expect(app.Do(http.MethodPatch, "/api/v1/quizzes/"+quiz.ID, e.instructor, map[string]any{"status": "published"}), http.StatusOK, "")

	var attempt assessment.Attempt
	app.Do(http.MethodPost, "/api/v1/quizzes/"+quiz.ID+"/attempts", e.alice, nil).JSON(&attempt)
	app.Expect(app.Do(http.MethodPost, "/api/v1/attempts/"+attempt.ID+"/submit", e.alice, map[string]any{"answers": []map[string]string{
		{"questionId": qs[0].ID, "optionId": qs[0].Options[0].ID}, {"questionId": qs[1].ID, "optionId": qs[1].Options[1].ID},
	}}), http.StatusOK, "")
	app.Do(http.MethodPost, "/api/v1/quizzes/"+quiz.ID+"/attempts", e.alice, nil).JSON(&attempt)
	app.Expect(app.Do(http.MethodPost, "/api/v1/attempts/"+attempt.ID+"/submit", e.alice, map[string]any{"answers": []map[string]string{
		{"questionId": qs[0].ID, "optionId": qs[0].Options[0].ID}, {"questionId": qs[1].ID, "optionId": qs[1].Options[0].ID},
	}}), http.StatusOK, "")

	var open assessment.Assignment
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/assignments", e.instructor, map[string]any{
		"title": "A1", "status": "published", "dueAt": time.Now().Add(48 * time.Hour).Format(time.RFC3339), "submissionTypes": []string{"text"},
	}).JSON(&open)
	var sub assessment.Submission
	app.Do(http.MethodPost, "/api/v1/assignments/"+open.ID+"/submissions", e.alice, map[string]any{"text": "done"}).JSON(&sub)
	app.Expect(app.Do(http.MethodPost, "/api/v1/submissions/"+sub.ID+"/grade", e.instructor, map[string]any{"grade": 70}), http.StatusOK, "")

	for i := 0; i < 2; i++ {
		app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/assignments", e.instructor, map[string]any{
			"title": "Past", "status": "published", "dueAt": time.Now().Add(-time.Hour).Format(time.RFC3339), "submissionTypes": []string{"text"},
		}), http.StatusCreated, "")
	}
	if _, err := app.DB.Exec(context.Background(), `UPDATE quizzes SET due_at = now() - interval '1 minute' WHERE id = $1`, quiz.ID); err != nil {
		t.Fatal(err)
	}

	start := time.Now().Add(-time.Hour).UTC()
	var session struct {
		ID string `json:"id"`
	}
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/sessions", e.instructor, map[string]any{
		"title": "L1", "startsAt": start.Format(time.RFC3339), "endsAt": start.Add(2 * time.Hour).Format(time.RFC3339),
	}).JSON(&session)
	var window struct {
		Token string `json:"token"`
	}
	app.Do(http.MethodPost, "/api/v1/sessions/"+session.ID+"/attendance/start", e.instructor, map[string]any{"graceMinutes": 120}).JSON(&window)
	app.Expect(app.Do(http.MethodPost, "/api/v1/attendance/check-in", e.alice, map[string]any{"token": window.Token}), http.StatusCreated, "")
	app.Expect(app.Do(http.MethodPost, "/api/v1/sessions/"+session.ID+"/attendance/close", e.instructor, nil), http.StatusOK, "")
}

func TestGradePolicyVersioning(t *testing.T) {
	e := setup(t)
	app := e.app
	path := "/api/v1/courses/" + e.course.ID + "/grade-policy"

	var p performance.Policy
	app.Do(http.MethodGet, path, e.alice, nil).JSON(&p)
	if !p.IsDefault || p.QuizWeight != 20 || p.AssignmentWeight != 40 || p.AttendanceWeight != 20 || p.ProjectWeight != 20 {
		t.Fatalf("default policy %+v", p)
	}
	app.Expect(app.Do(http.MethodGet, path, e.outsider, nil), http.StatusNotFound, "not_found")

	body := map[string]any{"quizWeight": 30, "assignmentWeight": 50, "attendanceWeight": 20, "projectWeight": 0, "reason": "no projects this cohort"}
	app.Expect(app.Do(http.MethodPut, path, e.alice, body), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPut, path, e.other, body), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPut, path, e.instructor, map[string]any{"quizWeight": 50, "assignmentWeight": 50, "attendanceWeight": 20, "projectWeight": 0}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPut, path, e.instructor, map[string]any{"quizWeight": 30, "assignmentWeight": 50, "attendanceWeight": 20, "projectWeight": 0, "probationGrade": 70}), http.StatusBadRequest, "validation_error")

	res := app.Do(http.MethodPut, path, e.instructor, body)
	app.Expect(res, http.StatusOK, "")
	res.JSON(&p)
	if p.Version != 1 || p.IsDefault || p.QuizWeight != 30 {
		t.Fatalf("v1 %+v", p)
	}
	body["quizWeight"], body["assignmentWeight"] = 40, 40
	app.Do(http.MethodPut, path, e.instructor, body).JSON(&p)
	if p.Version != 2 {
		t.Fatalf("v2 %+v", p)
	}
	var versions []performance.Policy
	app.Do(http.MethodGet, path+"/versions", e.instructor, nil).JSON(&versions)
	if len(versions) != 2 || versions[0].Version != 2 {
		t.Fatalf("versions %+v", versions)
	}
	app.Expect(app.Do(http.MethodGet, path+"/versions", e.tracker, nil), http.StatusForbidden, "forbidden")

	var audited int
	_ = app.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'grade_policy.updated' AND target_id = $1`, e.course.ID).Scan(&audited)
	if audited != 2 {
		t.Fatalf("policy changes audited: %d", audited)
	}
}

func TestStudentGradesAndStanding(t *testing.T) {
	e := setup(t)
	e.seedWork(t)
	app := e.app

	var g performance.StudentGrades
	res := app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/grades", e.alice, nil)
	app.Expect(res, http.StatusOK, "")
	res.JSON(&g)
	if len(g.Courses) != 1 {
		t.Fatalf("grades %+v", g)
	}
	c := g.Courses[0]
	if *c.QuizAvg != 100 || *c.AssignmentAvg != 23.33 || *c.Attendance != 100 || c.ProjectGrade != nil || c.Missing != 2 {
		t.Fatalf("alice components %+v", c.Result)
	}
	want := (100*20 + 23.33*40 + 100*20) / 80.0
	if *c.Overall < want-0.02 || *c.Overall > want+0.02 || c.Standing != "warning" || g.GPA == nil {
		t.Fatalf("alice overall %v standing %s", *c.Overall, c.Standing)
	}

	app.Do(http.MethodGet, "/api/v1/students/"+e.bobID+"/grades", e.instructor, nil).JSON(&g)
	b := g.Courses[0]
	if *b.QuizAvg != 0 || *b.AssignmentAvg != 0 || *b.Attendance != 0 || b.Standing != "probation" {
		t.Fatalf("bob %+v", b.Result)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/grades", e.bob, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/grades", e.other, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/standing", e.tracker, nil), http.StatusOK, "")

	var list []performance.StudentStanding
	res = app.Do(http.MethodGet, "/api/v1/courses/"+e.course.ID+"/standing?standing=probation", e.tracker, nil)
	app.Expect(res, http.StatusOK, "")
	res.JSON(&list)
	if len(list) != 1 || list[0].StudentID != e.bobID || len(list[0].Factors) == 0 {
		t.Fatalf("probation list %+v", list)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+e.course.ID+"/standing", e.alice, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+e.course.ID+"/standing?standing=bad", e.tracker, nil), http.StatusBadRequest, "validation_error")

	res = app.Do(http.MethodGet, "/api/v1/at-risk", e.tracker, nil)
	res.JSON(&list)
	if len(list) != 2 || res.Header().Get("X-Total-Count") != "2" || list[0].Standing != "probation" {
		t.Fatalf("at-risk %+v", list)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/at-risk", e.alice, nil), http.StatusForbidden, "forbidden")
	res = app.Do(http.MethodGet, "/api/v1/at-risk", e.other, nil)
	res.JSON(&list)
	if len(list) != 0 {
		t.Fatal("unrelated instructor must not see at-risk students")
	}

	app.Do(http.MethodPut, "/api/v1/courses/"+e.course.ID+"/grade-policy", e.instructor, map[string]any{
		"quizWeight": 50, "assignmentWeight": 0, "attendanceWeight": 50, "projectWeight": 0, "warningMissing": 5,
	})
	app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/grades", e.alice, nil).JSON(&g)
	if *g.Courses[0].Overall != 100 || g.Courses[0].Standing != "good" || g.Courses[0].PolicyVersion != 1 {
		t.Fatalf("new policy should apply: %+v", g.Courses[0])
	}

	res = app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/standing/snapshots", e.instructor, nil)
	app.Expect(res, http.StatusCreated, "")
	var snaps int
	_ = app.DB.QueryRow(context.Background(), `SELECT count(*) FROM standing_snapshots WHERE course_id = $1`, e.course.ID).Scan(&snaps)
	if snaps != 2 {
		t.Fatalf("snapshots %d", snaps)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/standing/snapshots", e.tracker, nil), http.StatusForbidden, "forbidden")
}
