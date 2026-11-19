package certificates_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/assessment"
	"github.com/Akrambahy/EduFlow/backend/internal/certificates"
	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/server"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
)

type env struct {
	app               *testkit.App
	admin, instructor string
	alice, bob        string
	aliceID, bobID    string
	course            learning.Course
}

func setup(t *testing.T, opts ...server.Options) *env {
	app := testkit.NewApp(t, opts...)
	e := &env{app: app}
	_, e.admin = app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	_, e.instructor = app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")
	e.aliceID, e.alice = app.UserWithToken("Alice Student", "alice@eduflow.dev", "student")
	e.bobID, e.bob = app.UserWithToken("Bob", "bob@eduflow.dev", "student")
	app.Do(http.MethodPost, "/api/v1/courses", e.instructor, map[string]any{"title": "Go Backend", "status": "published"}).JSON(&e.course)
	for _, id := range []string{e.aliceID, e.bobID} {
		app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/enrollments", e.instructor, map[string]any{"studentId": id})
	}

	var modules []learning.Module
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/modules", e.instructor, map[string]any{"title": "M"}).JSON(&modules)
	app.Do(http.MethodPost, "/api/v1/modules/"+modules[0].ID+"/lessons", e.instructor, map[string]any{"title": "L"}).JSON(&modules)
	app.Expect(app.Do(http.MethodPost, "/api/v1/lessons/"+modules[0].Lessons[0].ID+"/progress", e.alice, map[string]any{"status": "completed"}), http.StatusOK, "")

	var a assessment.Assignment
	app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/assignments", e.instructor, map[string]any{
		"title": "A", "status": "published", "dueAt": time.Now().Add(48 * time.Hour).Format(time.RFC3339), "submissionTypes": []string{"text"},
	}).JSON(&a)
	var sub assessment.Submission
	app.Do(http.MethodPost, "/api/v1/assignments/"+a.ID+"/submissions", e.alice, map[string]any{"text": "work"}).JSON(&sub)
	app.Expect(app.Do(http.MethodPost, "/api/v1/submissions/"+sub.ID+"/grade", e.instructor, map[string]any{"grade": 80}), http.StatusOK, "")
	return e
}

func TestEligibilityAndIssuance(t *testing.T) {
	e := setup(t)
	app := e.app

	var list []certificates.Eligibility
	res := app.Do(http.MethodGet, "/api/v1/courses/"+e.course.ID+"/certificate-eligibility", e.instructor, nil)
	app.Expect(res, http.StatusOK, "")
	res.JSON(&list)
	if len(list) != 2 || !list[0].Eligible || list[0].StudentID != e.aliceID || list[1].Eligible {
		t.Fatalf("eligibility %+v", list)
	}
	if !contains(list[1].Reasons, certificates.ReasonLessons) || !contains(list[1].Reasons, certificates.ReasonNoGrade) {
		t.Fatalf("bob reasons %+v", list[1].Reasons)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/courses/"+e.course.ID+"/certificate-eligibility", e.alice, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/certificate-eligibility?courseId="+e.course.ID, e.alice, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/certificate-eligibility?courseId="+e.course.ID, e.bob, nil), http.StatusForbidden, "forbidden")

	issue := map[string]any{"courseId": e.course.ID, "studentId": e.bobID}
	app.Expect(app.Do(http.MethodPost, "/api/v1/certificates/issue", e.instructor, issue), http.StatusForbidden, "forbidden")
	res = app.Do(http.MethodPost, "/api/v1/certificates/issue", e.admin, issue)
	app.Expect(res, http.StatusConflict, "not_eligible")
	if !strings.Contains(res.Body.String(), certificates.ReasonLessons) {
		t.Fatalf("reasons missing: %s", res.Body.String())
	}

	var result certificates.IssueResult
	res = app.Do(http.MethodPost, "/api/v1/certificates/issue", e.admin, map[string]any{"courseId": e.course.ID})
	app.Expect(res, http.StatusCreated, "")
	res.JSON(&result)
	if len(result.Issued) != 1 || len(result.Skipped) != 1 {
		t.Fatalf("batch %+v", result)
	}
	cert := result.Issued[0]
	if cert.StudentID != e.aliceID || cert.Status != "issued" || cert.CourseName != "Go Backend" || cert.Grade == nil || !strings.HasPrefix(cert.VerificationCode, "EF-") {
		t.Fatalf("certificate %+v", cert)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/certificates/issue", e.admin, map[string]any{"courseId": e.course.ID, "studentId": e.aliceID}), http.StatusConflict, "already_issued")
	app.Do(http.MethodPost, "/api/v1/certificates/issue", e.admin, map[string]any{"courseId": e.course.ID}).JSON(&result)
	if len(result.Issued) != 0 {
		t.Fatal("batch issuance must be idempotent")
	}

	var certs []certificates.Certificate
	app.Do(http.MethodGet, "/api/v1/certificates", e.alice, nil).JSON(&certs)
	if len(certs) != 1 {
		t.Fatalf("alice certs %d", len(certs))
	}
	app.Do(http.MethodGet, "/api/v1/certificates", e.bob, nil).JSON(&certs)
	if len(certs) != 0 {
		t.Fatal("bob must not see alice's certificate")
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/"+cert.ID, e.bob, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/"+cert.ID, e.instructor, nil), http.StatusOK, "")

	res = app.Do(http.MethodGet, "/api/v1/certificates/verify/"+strings.ToLower(cert.VerificationCode), "", nil)
	app.Expect(res, http.StatusOK, "")
	body := res.Body.String()
	if !strings.Contains(body, "Alice Student") || strings.Contains(body, "alice@eduflow.dev") || strings.Contains(body, "grade") || strings.Contains(body, e.aliceID) {
		t.Fatalf("public verification must expose minimal data: %s", body)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/verify/EF-2026-ZZZZZZZZ", "", nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/verify/%27%3B%20DROP%20TABLE%20x%3B--", "", nil), http.StatusNotFound, "not_found")

	app.Expect(app.Do(http.MethodPost, "/api/v1/certificates/"+cert.ID+"/revoke", e.admin, map[string]any{}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/certificates/"+cert.ID+"/revoke", e.instructor, map[string]any{"reason": "plagiarism"}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/certificates/"+cert.ID+"/revoke", e.admin, map[string]any{"reason": "plagiarism found"}), http.StatusOK, "")
	app.Expect(app.Do(http.MethodPost, "/api/v1/certificates/"+cert.ID+"/revoke", e.admin, map[string]any{"reason": "again"}), http.StatusConflict, "invalid_certificate_state")

	var pub certificates.PublicCertificate
	app.Do(http.MethodGet, "/api/v1/certificates/verify/"+cert.VerificationCode, "", nil).JSON(&pub)
	if pub.Status != "revoked" || pub.RevokedAt == nil {
		t.Fatalf("revoked verification %+v", pub)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/certificates/"+cert.ID+"/reinstate", e.admin, map[string]any{"reason": "appeal accepted"}), http.StatusOK, "")

	var events []certificates.Event
	app.Do(http.MethodGet, "/api/v1/certificates/"+cert.ID+"/events", e.admin, nil).JSON(&events)
	if len(events) != 3 || events[0].Action != "issued" || events[1].Action != "revoked" || events[2].Action != "reinstated" {
		t.Fatalf("events %+v", events)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/"+cert.ID+"/events", e.alice, nil), http.StatusForbidden, "forbidden")
}

func TestRequirementsAndVerifyRateLimit(t *testing.T) {
	e := setup(t, server.Options{VerifyLimit: httpx.NewRateLimiter(time.Hour, 2).Middleware()})
	app := e.app
	path := "/api/v1/courses/" + e.course.ID + "/certificate-requirements"

	var r certificates.Requirements
	app.Do(http.MethodGet, path, e.alice, nil).JSON(&r)
	if !r.IsDefault || r.MinGrade != 60 || r.MinAttendance != 75 || !r.RequireAllLessons {
		t.Fatalf("defaults %+v", r)
	}
	body := map[string]any{"minGrade": 90, "minAttendance": 0, "requireAllLessons": false, "allowProbation": false}
	app.Expect(app.Do(http.MethodPut, path, e.instructor, body), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPut, path, e.admin, map[string]any{"minGrade": 101}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPut, path, e.admin, body), http.StatusOK, "")

	var el certificates.Eligibility
	app.Do(http.MethodGet, "/api/v1/students/"+e.aliceID+"/certificate-eligibility?courseId="+e.course.ID, e.alice, nil).JSON(&el)
	if el.Eligible || !contains(el.Reasons, certificates.ReasonGrade) {
		t.Fatalf("stricter grade requirement should apply: %+v", el)
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/verify/EF-2026-AAAAAAAA", "", nil), http.StatusNotFound, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/verify/EF-2026-AAAAAAAB", "", nil), http.StatusNotFound, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/certificates/verify/EF-2026-AAAAAAAC", "", nil), http.StatusTooManyRequests, "rate_limited")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
