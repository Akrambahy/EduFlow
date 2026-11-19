package assessment_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/assessment"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/files"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
)

type env struct {
	app                         *testkit.App
	admin, instructor, other    string
	student, student2, outsider string
	studentID, student2ID       string
	course                      learning.Course
}

func setup(t *testing.T) *env {
	app := testkit.NewApp(t)
	e := &env{app: app}
	_, e.admin = app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	_, e.instructor = app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")
	_, e.other = app.UserWithToken("Other", "other@eduflow.dev", "instructor")
	e.studentID, e.student = app.UserWithToken("Alice", "alice@eduflow.dev", "student")
	e.student2ID, e.student2 = app.UserWithToken("Bob", "bob@eduflow.dev", "student")
	_, e.outsider = app.UserWithToken("Zed", "zed@eduflow.dev", "student")
	res := app.Do(http.MethodPost, "/api/v1/courses", e.instructor, map[string]any{"title": "Go", "status": "published"})
	res.JSON(&e.course)
	for _, id := range []string{e.studentID, e.student2ID} {
		app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/enrollments", e.instructor, map[string]any{"studentId": id}), http.StatusCreated, "")
	}
	return e
}

var twoQuestions = map[string]any{"questions": []map[string]any{
	{"text": "2+2?", "points": 1, "options": []map[string]any{{"text": "3"}, {"text": "4", "correct": true}}},
	{"text": "Go keyword for goroutine?", "points": 2, "options": []map[string]any{{"text": "go", "correct": true}, {"text": "async"}, {"text": "spawn"}}},
}}

func (e *env) quiz(body map[string]any) assessment.Quiz {
	res := e.app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/quizzes", e.instructor, body)
	e.app.Expect(res, http.StatusCreated, "")
	var q assessment.Quiz
	res.JSON(&q)
	return q
}

func (e *env) publishedQuiz(body map[string]any) (assessment.Quiz, []assessment.Question) {
	q := e.quiz(body)
	res := e.app.Do(http.MethodPut, "/api/v1/quizzes/"+q.ID+"/questions", e.instructor, twoQuestions)
	e.app.Expect(res, http.StatusOK, "")
	var qs []assessment.Question
	res.JSON(&qs)
	e.app.Expect(e.app.Do(http.MethodPatch, "/api/v1/quizzes/"+q.ID, e.instructor, map[string]any{"status": "published"}), http.StatusOK, "")
	return q, qs
}

func correctAnswers(qs []assessment.Question) []map[string]string {
	var out []map[string]string
	for _, q := range qs {
		for _, o := range q.Options {
			if o.IsCorrect != nil && *o.IsCorrect {
				out = append(out, map[string]string{"questionId": q.ID, "optionId": o.ID})
			}
		}
	}
	return out
}

func TestQuizAuthoring(t *testing.T) {
	e := setup(t)
	app := e.app
	path := "/api/v1/courses/" + e.course.ID + "/quizzes"
	app.Expect(app.Do(http.MethodPost, path, e.student, map[string]any{"title": "Q"}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, path, e.other, map[string]any{"title": "Q"}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPost, path, e.instructor, map[string]any{"description": "no title"}), http.StatusBadRequest, "validation_error")

	q := e.quiz(map[string]any{"title": "Basics", "timeLimit": 20, "maxAttempts": 2})
	if q.Status != "draft" || q.TimeLimit == nil || *q.TimeLimit != 20 {
		t.Fatalf("quiz %+v", q)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/quizzes/"+q.ID, e.student, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/quizzes/"+q.ID, e.instructor, map[string]any{"status": "published"}), http.StatusBadRequest, "validation_error")

	bad := map[string]any{"questions": []map[string]any{{"text": "x", "options": []map[string]any{{"text": "a", "correct": true}, {"text": "b", "correct": true}}}}}
	app.Expect(app.Do(http.MethodPut, "/api/v1/quizzes/"+q.ID+"/questions", e.instructor, bad), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPut, "/api/v1/quizzes/"+q.ID+"/questions", e.other, twoQuestions), http.StatusNotFound, "not_found")
	res := app.Do(http.MethodPut, "/api/v1/quizzes/"+q.ID+"/questions", e.instructor, twoQuestions)
	app.Expect(res, http.StatusOK, "")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/quizzes/"+q.ID, e.instructor, map[string]any{"status": "published"}), http.StatusOK, "")

	res = app.Do(http.MethodGet, "/api/v1/quizzes?courseId="+e.course.ID, e.student, nil)
	var list []assessment.Quiz
	res.JSON(&list)
	if len(list) != 1 || list[0].Status != "open" || list[0].TotalQuestions != 2 || list[0].TotalPoints != 3 || list[0].AttemptsUsed == nil {
		t.Fatalf("student quiz list %+v", list)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/quizzes/"+q.ID+"/questions", e.student, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/quizzes/"+q.ID, e.outsider, nil), http.StatusNotFound, "not_found")
}

func TestQuizAttemptScoringAndLimits(t *testing.T) {
	e := setup(t)
	app := e.app
	q, qs := e.publishedQuiz(map[string]any{"title": "Basics", "maxAttempts": 1, "revealAnswers": "after_close"})

	app.Expect(app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.instructor, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.outsider, nil), http.StatusNotFound, "not_found")

	res := app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.student, nil)
	app.Expect(res, http.StatusCreated, "")
	if strings.Contains(res.Body.String(), "isCorrect") || strings.Contains(res.Body.String(), "correct") {
		t.Fatalf("attempt leaks answers: %s", res.Body.String())
	}
	var a assessment.Attempt
	res.JSON(&a)
	if len(a.Questions) != 2 || a.AttemptNumber != 1 || a.Status != "in_progress" {
		t.Fatalf("attempt %+v", a)
	}
	again := app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.student, nil)
	app.Expect(again, http.StatusOK, "")

	answers := correctAnswers(qs)
	crossed := []map[string]string{{"questionId": qs[0].ID, "optionId": qs[1].Options[0].ID}}
	app.Expect(app.Do(http.MethodPost, "/api/v1/attempts/"+a.ID+"/submit", e.student, map[string]any{"answers": crossed}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/attempts/"+a.ID+"/submit", e.student2, map[string]any{"answers": answers}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPost, "/api/v1/attempts/"+a.ID+"/submit", e.student, map[string]any{"answers": []map[string]string{answers[0], answers[0]}}), http.StatusBadRequest, "validation_error")

	res = app.Do(http.MethodPost, "/api/v1/attempts/"+a.ID+"/submit", e.student, map[string]any{"answers": []map[string]any{
		{"questionId": answers[0]["questionId"], "optionId": answers[0]["optionId"]}, {"questionId": qs[1].ID, "optionId": qs[1].Options[1].ID},
	}, "score": 100})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&a)
	if a.Status != "submitted" || *a.Score != 1 || *a.Total != 3 || *a.Percentage != 33 || *a.Passed {
		t.Fatalf("scored attempt %+v", a)
	}
	for _, ans := range a.Answers {
		if ans.CorrectOptionID != nil {
			t.Fatal("answers must stay hidden until the quiz closes")
		}
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/attempts/"+a.ID+"/submit", e.student, map[string]any{"answers": answers}), http.StatusConflict, "attempt_closed")
	app.Expect(app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.student, nil), http.StatusConflict, "attempt_limit_reached")
	app.Expect(app.Do(http.MethodPut, "/api/v1/quizzes/"+q.ID+"/questions", e.instructor, twoQuestions), http.StatusConflict, "quiz_has_attempts")

	if _, err := app.DB.Exec(context.Background(), `UPDATE quizzes SET due_at = now() - interval '1 minute' WHERE id = $1`, q.ID); err != nil {
		t.Fatal(err)
	}
	res = app.Do(http.MethodGet, "/api/v1/attempts/"+a.ID, e.student, nil)
	res.JSON(&a)
	revealed := 0
	for _, ans := range a.Answers {
		if ans.CorrectOptionID != nil {
			revealed++
		}
	}
	if revealed != 2 {
		t.Fatalf("answers should be revealed after close: %+v", a.Answers)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.student2, nil), http.StatusConflict, "quiz_not_open")
	app.Expect(app.Do(http.MethodGet, "/api/v1/attempts/"+a.ID, e.student2, nil), http.StatusNotFound, "not_found")

	res = app.Do(http.MethodGet, "/api/v1/quizzes/"+q.ID+"/attempts", e.instructor, nil)
	var attempts []assessment.Attempt
	res.JSON(&attempts)
	if len(attempts) != 1 {
		t.Fatalf("staff attempts %d", len(attempts))
	}
}

func TestQuizAttemptExpiry(t *testing.T) {
	e := setup(t)
	q, qs := e.publishedQuiz(map[string]any{"title": "Timed", "timeLimit": 5, "maxAttempts": 2})
	res := e.app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.student, nil)
	var a assessment.Attempt
	res.JSON(&a)
	if a.ExpiresAt == nil {
		t.Fatal("timed attempt needs expiresAt")
	}
	if _, err := e.app.DB.Exec(context.Background(), `UPDATE quiz_attempts SET expires_at = now() - interval '10 minutes' WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	e.app.Expect(e.app.Do(http.MethodPost, "/api/v1/attempts/"+a.ID+"/submit", e.student, map[string]any{"answers": correctAnswers(qs)}), http.StatusConflict, "attempt_expired")
	res = e.app.Do(http.MethodGet, "/api/v1/attempts/"+a.ID, e.student, nil)
	res.JSON(&a)
	if a.Status != "expired" || *a.Score != 0 {
		t.Fatalf("expired attempt %+v", a)
	}
	e.app.Expect(e.app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.student, nil), http.StatusCreated, "")
}

func TestConcurrentAttemptStarts(t *testing.T) {
	e := setup(t)
	q, _ := e.publishedQuiz(map[string]any{"title": "Race", "maxAttempts": 3})
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.app.Do(http.MethodPost, "/api/v1/quizzes/"+q.ID+"/attempts", e.student, nil)
		}()
	}
	wg.Wait()
	var n int
	_ = e.app.DB.QueryRow(context.Background(), `SELECT count(*) FROM quiz_attempts WHERE quiz_id = $1`, q.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("expected one attempt, got %d", n)
	}
}

func (e *env) assignment(body map[string]any) assessment.Assignment {
	res := e.app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/assignments", e.instructor, body)
	e.app.Expect(res, http.StatusCreated, "")
	var a assessment.Assignment
	res.JSON(&a)
	return a
}

var pdf = []byte("%PDF-1.7\n%%EOF\n")

func TestAssignmentSubmissionsAndGrading(t *testing.T) {
	e := setup(t)
	app := e.app
	due := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	a := e.assignment(map[string]any{"title": "Build an API", "dueAt": due, "maxGrade": 100, "submissionTypes": []string{"file", "url"}})
	if a.Status != "draft" {
		t.Fatalf("assignment %+v", a)
	}
	sub := "/api/v1/assignments/" + a.ID + "/submissions"
	app.Expect(app.Do(http.MethodPost, sub, e.student, map[string]any{"url": "https://github.com/x/y"}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/assignments/"+a.ID, e.instructor, map[string]any{"status": "published"}), http.StatusOK, "")

	app.Expect(app.Do(http.MethodPost, sub, e.instructor, map[string]any{"url": "https://x.dev"}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, sub, e.student, map[string]any{"text": "my answer"}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, sub, e.student, map[string]any{}), http.StatusBadRequest, "validation_error")

	var theirs files.File
	app.Upload(e.student2, "bob.pdf", pdf).JSON(&theirs)
	app.Expect(app.Do(http.MethodPost, sub, e.student, map[string]any{"fileId": theirs.ID}), http.StatusBadRequest, "validation_error")

	var mine files.File
	app.Upload(e.student, "alice.pdf", pdf).JSON(&mine)
	res := app.Do(http.MethodPost, sub, e.student, map[string]any{"fileId": mine.ID})
	app.Expect(res, http.StatusCreated, "")
	var s1 assessment.Submission
	res.JSON(&s1)
	if s1.Version != 1 || s1.Status != "submitted" || s1.FileName == nil || *s1.FileName != "alice.pdf" || s1.IsLate {
		t.Fatalf("submission %+v", s1)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+mine.ID+"/download", e.instructor, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+mine.ID+"/download", e.student2, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+mine.ID+"/download", e.other, nil), http.StatusNotFound, "not_found")

	res = app.Do(http.MethodPost, sub, e.student, map[string]any{"url": "https://github.com/alice/api"})
	var s2 assessment.Submission
	res.JSON(&s2)
	if s2.Version != 2 {
		t.Fatalf("resubmission %+v", s2)
	}
	var list []assessment.Submission
	app.Do(http.MethodGet, "/api/v1/submissions?assignmentId="+a.ID, e.student, nil).JSON(&list)
	if len(list) != 1 || list[0].ID != s2.ID {
		t.Fatalf("current submissions %+v", list)
	}
	app.Do(http.MethodGet, "/api/v1/submissions?assignmentId="+a.ID+"&allVersions=true", e.student, nil).JSON(&list)
	if len(list) != 2 {
		t.Fatalf("all versions %d", len(list))
	}
	app.Do(http.MethodGet, "/api/v1/submissions", e.student2, nil).JSON(&list)
	if len(list) != 0 {
		t.Fatal("other student must not see submissions")
	}

	grade := "/api/v1/submissions/" + s2.ID + "/grade"
	app.Expect(app.Do(http.MethodPost, grade, e.student, map[string]any{"grade": 100}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, grade, e.other, map[string]any{"grade": 100}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 101}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/submissions/"+s1.ID+"/grade", e.instructor, map[string]any{"grade": 50}), http.StatusConflict, "not_latest_submission")

	res = app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 80, "feedback": "Good work"})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&s2)
	if s2.Status != "graded" || *s2.Grade != 80 || *s2.FinalGrade != 80 || *s2.Feedback != "Good work" {
		t.Fatalf("graded %+v", s2)
	}
	app.Expect(app.Do(http.MethodPost, sub, e.student, map[string]any{"url": "https://github.com/alice/v3"}), http.StatusConflict, "already_graded")
	app.Expect(app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 90}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 90, "reason": "rubric recount", "returnForRevision": true}), http.StatusOK, "")

	var changed int
	_ = app.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action = 'grade.changed' AND target_id = $1
		AND (context->>'before')::numeric = 80 AND context->>'reason' = 'rubric recount'`, s2.ID).Scan(&changed)
	if changed != 1 {
		t.Fatal("grade change must be audited with before and reason")
	}
	app.Expect(app.Do(http.MethodPost, sub, e.student, map[string]any{"url": "https://github.com/alice/v3"}), http.StatusCreated, "")
}

func TestLatePolicies(t *testing.T) {
	e := setup(t)
	app := e.app
	past := time.Now().Add(-time.Hour).UTC()
	strict := e.assignment(map[string]any{"title": "Strict", "dueAt": past.Format(time.RFC3339), "status": "published", "submissionTypes": []string{"text"}})
	app.Expect(app.Do(http.MethodPost, "/api/v1/assignments/"+strict.ID+"/submissions", e.student, map[string]any{"text": "late"}), http.StatusConflict, "deadline_passed")
	if strict.Status != "closed" {
		t.Fatalf("strict status %s", strict.Status)
	}

	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/assignments", e.instructor, map[string]any{
		"title": "Bad", "dueAt": past.Format(time.RFC3339), "latePolicy": "reject", "lateUntil": time.Now().Add(time.Hour).Format(time.RFC3339),
	}), http.StatusBadRequest, "validation_error")

	soft := e.assignment(map[string]any{"title": "Soft", "dueAt": past.Format(time.RFC3339), "status": "published", "submissionTypes": []string{"text"},
		"latePolicy": "penalty", "latePenaltyPercent": 10, "lateUntil": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)})
	if soft.Status != "open" {
		t.Fatalf("soft status %s", soft.Status)
	}
	res := app.Do(http.MethodPost, "/api/v1/assignments/"+soft.ID+"/submissions", e.student, map[string]any{"text": "late but here"})
	app.Expect(res, http.StatusCreated, "")
	var s assessment.Submission
	res.JSON(&s)
	if !s.IsLate {
		t.Fatal("submission should be marked late")
	}
	res = app.Do(http.MethodPost, "/api/v1/submissions/"+s.ID+"/grade", e.instructor, map[string]any{"grade": 80})
	res.JSON(&s)
	if *s.Grade != 80 || *s.FinalGrade != 72 {
		t.Fatalf("penalty grade %+v", s)
	}
}
