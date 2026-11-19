package projects_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/learning"
	"github.com/Akrambahy/EduFlow/backend/internal/performance"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/files"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/server"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
	"github.com/Akrambahy/EduFlow/backend/internal/projects"
)

type fakeGitHub struct{}

func (fakeGitHub) Repository(_ context.Context, owner, name string) (*projects.RepoMetadata, error) {
	if name == "broken" {
		return nil, projects.ErrRepoUnavailable
	}
	return &projects.RepoMetadata{FullName: owner + "/" + name, DefaultBranch: "main", Stars: 7}, nil
}

type env struct {
	app                         *testkit.App
	admin, instructor, other    string
	alice, bob, carol, outsider string
	aliceID, bobID, carolID     string
	course                      learning.Course
}

func setup(t *testing.T) *env {
	app := testkit.NewApp(t, server.Options{GitHub: fakeGitHub{}})
	e := &env{app: app}
	_, e.admin = app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	_, e.instructor = app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")
	_, e.other = app.UserWithToken("Other", "other@eduflow.dev", "instructor")
	e.aliceID, e.alice = app.UserWithToken("Alice", "alice@eduflow.dev", "student")
	e.bobID, e.bob = app.UserWithToken("Bob", "bob@eduflow.dev", "student")
	e.carolID, e.carol = app.UserWithToken("Carol", "carol@eduflow.dev", "student")
	_, e.outsider = app.UserWithToken("Zed", "zed@eduflow.dev", "student")
	app.Do(http.MethodPost, "/api/v1/courses", e.instructor, map[string]any{"title": "Go", "status": "published"}).JSON(&e.course)
	for _, id := range []string{e.aliceID, e.bobID, e.carolID} {
		app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/enrollments", e.instructor, map[string]any{"studentId": id}), http.StatusCreated, "")
	}
	return e
}

func (e *env) project(body map[string]any) projects.Project {
	res := e.app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/projects", e.instructor, body)
	e.app.Expect(res, http.StatusCreated, "")
	var p projects.Project
	res.JSON(&p)
	return p
}

func future() string { return time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339) }

func TestTeamProjectLifecycle(t *testing.T) {
	e := setup(t)
	app := e.app
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/projects", e.alice, map[string]any{"title": "X", "dueAt": future()}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, "/api/v1/courses/"+e.course.ID+"/projects", e.other, map[string]any{"title": "X", "dueAt": future()}), http.StatusNotFound, "not_found")

	p := e.project(map[string]any{"title": "Capstone", "dueAt": future(), "teamSize": 2, "requiresPresentation": true, "allowResubmission": false,
		"criteria": []map[string]any{{"title": "Code quality", "maxPoints": 60}, {"title": "Demo", "maxPoints": 40}}})
	if p.Status != "draft" || len(p.Criteria) != 2 {
		t.Fatalf("project %+v", p)
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/projects/"+p.ID, e.alice, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/projects/"+p.ID, e.instructor, map[string]any{"status": "published"}), http.StatusOK, "")

	app.Expect(app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/submissions", e.alice, map[string]any{"repoUrl": "https://github.com/alice/capstone"}), http.StatusConflict, "team_required")

	res := app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/teams", e.alice, map[string]any{"name": "Gophers"})
	app.Expect(res, http.StatusCreated, "")
	var team projects.Team
	res.JSON(&team)
	app.Expect(app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/teams", e.bob, map[string]any{"name": "gophers"}), http.StatusConflict, "team_name_taken")
	app.Expect(app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/teams", e.alice, map[string]any{"name": "Second"}), http.StatusConflict, "already_in_team")
	app.Expect(app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/teams", e.outsider, map[string]any{"name": "Z"}), http.StatusNotFound, "not_found")

	app.Expect(app.Do(http.MethodPost, "/api/v1/teams/"+team.ID+"/members", e.bob, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodPost, "/api/v1/teams/"+team.ID+"/members", e.carol, nil), http.StatusConflict, "team_full")
	app.Expect(app.Do(http.MethodPost, "/api/v1/teams/"+team.ID+"/members", e.alice, map[string]any{"studentId": e.carolID}), http.StatusForbidden, "forbidden")

	var mine projects.Project
	app.Do(http.MethodGet, "/api/v1/projects/"+p.ID, e.bob, nil).JSON(&mine)
	if mine.MyTeamID == nil || *mine.MyTeamID != team.ID {
		t.Fatalf("myTeamId %+v", mine)
	}

	res = app.Do(http.MethodPut, "/api/v1/teams/"+team.ID+"/repository", e.alice, map[string]any{"url": "https://github.com/alice/capstone.git"})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&team)
	if team.Repository == nil || team.Repository.URL != "https://github.com/alice/capstone" || team.Repository.Metadata == nil || team.Repository.Metadata.Stars != 7 {
		t.Fatalf("repository %+v", team.Repository)
	}
	app.Expect(app.Do(http.MethodPut, "/api/v1/teams/"+team.ID+"/repository", e.alice, map[string]any{"url": "https://evil.com/a/b"}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPut, "/api/v1/teams/"+team.ID+"/repository", e.carol, map[string]any{"url": "https://github.com/carol/x"}), http.StatusForbidden, "forbidden")

	res = app.Do(http.MethodPut, "/api/v1/teams/"+team.ID+"/repository", e.bob, map[string]any{"url": "https://github.com/alice/broken"})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&team)
	if team.Repository.SyncError == nil || team.Repository.Metadata != nil {
		t.Fatalf("GitHub failure must be recorded, not fatal: %+v", team.Repository)
	}

	res = app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/submissions", e.bob, map[string]any{"repoUrl": "https://github.com/alice/broken", "notes": "v1"})
	app.Expect(res, http.StatusCreated, "")
	var sub projects.Submission
	res.JSON(&sub)
	if sub.TeamID != team.ID || sub.Version != 1 || sub.StudentName != "Bob" {
		t.Fatalf("submission %+v", sub)
	}
	app.Expect(app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/submissions", e.alice, map[string]any{"repoUrl": "https://github.com/alice/v2"}), http.StatusConflict, "resubmission_not_allowed")
	app.Expect(app.Do(http.MethodDelete, "/api/v1/teams/"+team.ID+"/members/"+e.bobID, e.bob, nil), http.StatusConflict, "team_locked")

	var list []projects.Submission
	app.Do(http.MethodGet, "/api/v1/project-submissions?projectId="+p.ID, e.alice, nil).JSON(&list)
	if len(list) != 1 {
		t.Fatalf("team member sees team submission: %d", len(list))
	}
	app.Do(http.MethodGet, "/api/v1/project-submissions?projectId="+p.ID, e.carol, nil).JSON(&list)
	if len(list) != 0 {
		t.Fatal("non-member must not see the submission")
	}

	app.Expect(app.Do(http.MethodPut, "/api/v1/teams/"+team.ID+"/presentation", e.alice, map[string]any{"scheduledAt": future()}), http.StatusForbidden, "forbidden")
	res = app.Do(http.MethodPut, "/api/v1/teams/"+team.ID+"/presentation", e.instructor, map[string]any{"scheduledAt": future(), "location": "Room 3"})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&team)
	if team.Presentation == nil || team.Presentation.PresentationDate == "" {
		t.Fatalf("presentation %+v", team.Presentation)
	}

	grade := "/api/v1/project-submissions/" + sub.ID + "/grade"
	app.Expect(app.Do(http.MethodPost, grade, e.alice, map[string]any{"grade": 100}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPost, grade, e.carol, map[string]any{"grade": 100}), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 90, "criteriaScores": map[string]int{"Code quality": 70}}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 90, "criteriaScores": map[string]int{"Speed": 1}}), http.StatusBadRequest, "validation_error")
	res = app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 85, "criteriaScores": map[string]int{"Code quality": 50, "Demo": 35}, "feedback": "Nice"})
	app.Expect(res, http.StatusOK, "")
	res.JSON(&sub)
	if *sub.Grade != 85 || sub.Status != "graded" || sub.CriteriaScores["Demo"] != 35 || sub.PresentationDate == nil {
		t.Fatalf("graded %+v", sub)
	}
	app.Expect(app.Do(http.MethodPost, grade, e.instructor, map[string]any{"grade": 70}), http.StatusBadRequest, "validation_error")

	var g performance.StudentGrades
	app.Do(http.MethodGet, "/api/v1/students/"+e.bobID+"/grades", e.bob, nil).JSON(&g)
	if len(g.Courses) != 1 || g.Courses[0].ProjectGrade == nil || *g.Courses[0].ProjectGrade != 85 {
		t.Fatalf("project grade must feed performance: %+v", g)
	}
	app.Do(http.MethodGet, "/api/v1/students/"+e.carolID+"/grades", e.carol, nil).JSON(&g)
	if g.Courses[0].ProjectGrade != nil || g.Courses[0].Pending != 1 {
		t.Fatalf("carol project should be pending: %+v", g.Courses[0])
	}
}

func TestSoloProjectAndFiles(t *testing.T) {
	e := setup(t)
	app := e.app
	p := e.project(map[string]any{"title": "Solo", "dueAt": future(), "status": "published"})

	var f files.File
	app.Upload(e.alice, "report.pdf", []byte("%PDF-1.7\n%%EOF\n")).JSON(&f)
	app.Expect(app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/submissions", e.alice, map[string]any{}), http.StatusBadRequest, "validation_error")
	res := app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/submissions", e.alice, map[string]any{"fileId": f.ID})
	app.Expect(res, http.StatusCreated, "")
	var sub projects.Submission
	res.JSON(&sub)
	app.Expect(app.Do(http.MethodPost, "/api/v1/projects/"+p.ID+"/submissions", e.alice, map[string]any{"repoUrl": "https://github.com/alice/solo"}), http.StatusCreated, "")

	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.instructor, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/files/"+f.ID+"/download", e.bob, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodPut, "/api/v1/teams/"+sub.TeamID+"/presentation", e.instructor, map[string]any{"scheduledAt": future()}), http.StatusConflict, "presentation_not_required")
}
