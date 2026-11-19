package identity_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/audit"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testkit"
)

func auditCount(t *testing.T, app *testkit.App, action, targetID string) int {
	t.Helper()
	var n int
	if err := app.DB.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE action = $1 AND target_id = $2`, action, targetID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUserManagementRequiresAdmin(t *testing.T) {
	app := testkit.NewApp(t)
	targetID := app.CreateUser("Target", "target@eduflow.dev", "student")
	_, instructor := app.UserWithToken("Inst", "inst@eduflow.dev", "instructor")

	cases := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/v1/users", map[string]any{"name": "X", "email": "x@eduflow.dev", "password": "password1", "roles": []string{"admin"}}},
		{http.MethodPatch, "/api/v1/users/" + targetID, map[string]any{"isActive": false}},
		{http.MethodPut, "/api/v1/users/" + targetID + "/roles", map[string]any{"roles": []string{"admin"}}},
		{http.MethodPost, "/api/v1/users/" + targetID + "/password", map[string]any{"password": "password1"}},
		{http.MethodGet, "/api/v1/users/" + targetID, nil},
		{http.MethodGet, "/api/v1/audit-logs", nil},
	}
	for _, tc := range cases {
		app.Expect(app.Do(tc.method, tc.path, "", tc.body), http.StatusUnauthorized, "unauthorized")
		app.Expect(app.Do(tc.method, tc.path, instructor, tc.body), http.StatusForbidden, "forbidden")
	}
}

func TestAdminCreatesUser(t *testing.T) {
	app := testkit.NewApp(t)
	_, admin := app.UserWithToken("Admin", "admin@eduflow.dev", "admin")

	body := map[string]any{
		"name": "  New Mentor ", "email": "Mentor@EduFlow.dev", "password": "mentorpass1",
		"roles": []string{"student", "mentor", "mentor"}, "university": "Cairo University",
	}
	res := app.Do(http.MethodPost, "/api/v1/users", admin, body)
	app.Expect(res, http.StatusCreated, "")
	var u identity.UserResponse
	res.JSON(&u)
	if u.Name != "New Mentor" || u.Email != "mentor@eduflow.dev" || u.Role != "mentor" || len(u.Roles) != 2 || u.University == nil || *u.University != "Cairo University" {
		t.Fatalf("unexpected user %+v", u)
	}
	if res.Header().Get("Location") != "/api/v1/users/"+u.ID {
		t.Fatal("missing Location header")
	}
	if auditCount(t, app, audit.ActionUserCreated, u.ID) != 1 {
		t.Fatal("expected user.created audit event")
	}

	login := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "mentor@eduflow.dev", "password": "mentorpass1"})
	app.Expect(login, http.StatusOK, "")

	app.Expect(app.Do(http.MethodPost, "/api/v1/users", admin, body), http.StatusConflict, "email_taken")

	bad := map[string]any{"name": "", "email": "nope", "password": "short", "roles": []string{"superuser"}}
	res = app.Do(http.MethodPost, "/api/v1/users", admin, bad)
	app.Expect(res, http.StatusBadRequest, "validation_error")

	weak := map[string]any{"name": "W", "email": "w@eduflow.dev", "password": "onlyletters", "roles": []string{"student"}}
	app.Expect(app.Do(http.MethodPost, "/api/v1/users", admin, weak), http.StatusBadRequest, "validation_error")
}

func TestListUsersPaginationAndFilters(t *testing.T) {
	app := testkit.NewApp(t)
	_, admin := app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	app.CreateUser("Alice Student", "alice@eduflow.dev", "student")
	app.CreateUser("Bob Student", "bob@eduflow.dev", "student")
	app.CreateUser("Carol Mentor", "carol@eduflow.dev", "mentor")

	res := app.Do(http.MethodGet, "/api/v1/users?role=student&limit=1&page=2", admin, nil)
	app.Expect(res, http.StatusOK, "")
	var list []identity.UserResponse
	res.JSON(&list)
	if len(list) != 1 || list[0].Name != "Bob Student" || res.Header().Get("X-Total-Count") != "2" {
		t.Fatalf("page 2: %+v total=%s", list, res.Header().Get("X-Total-Count"))
	}

	res = app.Do(http.MethodGet, "/api/v1/users?q=CAROL", admin, nil)
	res.JSON(&list)
	if len(list) != 1 || list[0].Email != "carol@eduflow.dev" {
		t.Fatalf("search: %+v", list)
	}

	res = app.Do(http.MethodGet, "/api/v1/users?q=%25", admin, nil)
	res.JSON(&list)
	if len(list) != 0 {
		t.Fatalf("wildcard must be literal: %+v", list)
	}

	res = app.Do(http.MethodGet, "/api/v1/users?limit=1000", admin, nil)
	res.JSON(&list)
	if len(list) != 4 {
		t.Fatalf("expected all 4 users, got %d", len(list))
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/users?role=boss", admin, nil), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodGet, "/api/v1/users?isActive=maybe", admin, nil), http.StatusBadRequest, "validation_error")
}

func TestGetUserScope(t *testing.T) {
	app := testkit.NewApp(t)
	_, admin := app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	studentID, student := app.UserWithToken("Stu", "stu@eduflow.dev", "student")
	otherID := app.CreateUser("Other", "other@eduflow.dev", "student")

	app.Expect(app.Do(http.MethodGet, "/api/v1/users/"+studentID, student, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/users/"+otherID, student, nil), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodGet, "/api/v1/users/"+otherID, admin, nil), http.StatusOK, "")
	app.Expect(app.Do(http.MethodGet, "/api/v1/users/00000000-0000-0000-0000-000000000000", admin, nil), http.StatusNotFound, "not_found")
	app.Expect(app.Do(http.MethodGet, "/api/v1/users/not-a-uuid", admin, nil), http.StatusNotFound, "not_found")
}

func TestProfileUpdateOnlyAllowedFields(t *testing.T) {
	app := testkit.NewApp(t)
	id, token := app.UserWithToken("Stu", "stu@eduflow.dev", "student")

	res := app.Do(http.MethodPatch, "/api/v1/users/me", token, map[string]any{
		"name": "Student Renamed", "university": "Ain Shams", "avatar": "https://cdn.example.com/a.png",
		"email": "hacker@eduflow.dev", "roles": []string{"admin"}, "isActive": false,
	})
	app.Expect(res, http.StatusOK, "")
	var u identity.UserResponse
	res.JSON(&u)
	if u.Name != "Student Renamed" || u.Email != "stu@eduflow.dev" || u.Role != "student" || !u.IsActive || u.Avatar == nil {
		t.Fatalf("profile update leaked privileged fields: %+v", u)
	}

	res = app.Do(http.MethodPatch, "/api/v1/users/me", token, map[string]any{"university": "", "avatar": ""})
	res.JSON(&u)
	if u.University != nil || u.Avatar != nil {
		t.Fatalf("expected cleared fields: %+v", u)
	}

	app.Expect(app.Do(http.MethodPatch, "/api/v1/users/me", token, map[string]any{"avatar": "javascript"}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/users/me", token, map[string]any{"name": ""}), http.StatusBadRequest, "validation_error")
	_ = id
}

func TestRoleChangesAndSelfEscalation(t *testing.T) {
	app := testkit.NewApp(t)
	adminID, admin := app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	studentID, student := app.UserWithToken("Stu", "stu@eduflow.dev", "student")

	app.Expect(app.Do(http.MethodPut, "/api/v1/users/"+studentID+"/roles", student, map[string]any{"roles": []string{"admin"}}), http.StatusForbidden, "forbidden")
	app.Expect(app.Do(http.MethodPut, "/api/v1/users/"+adminID+"/roles", admin, map[string]any{"roles": []string{"student"}}), http.StatusForbidden, "self_modification")
	app.Expect(app.Do(http.MethodPut, "/api/v1/users/"+studentID+"/roles", admin, map[string]any{"roles": []string{}}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPut, "/api/v1/users/00000000-0000-0000-0000-000000000000/roles", admin, map[string]any{"roles": []string{"student"}}), http.StatusNotFound, "not_found")

	res := app.Do(http.MethodPut, "/api/v1/users/"+studentID+"/roles", admin, map[string]any{"roles": []string{"tracker", "student"}})
	app.Expect(res, http.StatusOK, "")
	var u identity.UserResponse
	res.JSON(&u)
	if u.Role != "tracker" || len(u.Roles) != 2 {
		t.Fatalf("roles not updated: %+v", u)
	}
	if auditCount(t, app, audit.ActionUserRolesChanged, studentID) != 1 {
		t.Fatal("expected roles_changed audit")
	}

	me := app.Do(http.MethodGet, "/api/v1/auth/me", student, nil)
	me.JSON(&u)
	if u.Role != "tracker" {
		t.Fatal("role change must apply to existing tokens immediately")
	}

	app.Do(http.MethodPut, "/api/v1/users/"+studentID+"/roles", admin, map[string]any{"roles": []string{"student", "tracker"}})
	if auditCount(t, app, audit.ActionUserRolesChanged, studentID) != 1 {
		t.Fatal("no-op role change must not be audited")
	}
}

func TestDeactivateUser(t *testing.T) {
	app := testkit.NewApp(t)
	adminID, admin := app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	studentID := app.CreateUser("Stu", "stu@eduflow.dev", "student")
	login := app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "stu@eduflow.dev", "password": testkit.Password})
	var session struct {
		Token string `json:"token"`
	}
	login.JSON(&session)
	cookie := login.Cookie(identity.RefreshCookieName)

	app.Expect(app.Do(http.MethodPatch, "/api/v1/users/"+adminID, admin, map[string]any{"isActive": false}), http.StatusForbidden, "self_modification")

	res := app.Do(http.MethodPatch, "/api/v1/users/"+studentID, admin, map[string]any{"isActive": false})
	app.Expect(res, http.StatusOK, "")
	if auditCount(t, app, audit.ActionUserDeactivated, studentID) != 1 {
		t.Fatal("expected deactivated audit")
	}
	app.Expect(app.Do(http.MethodGet, "/api/v1/auth/me", session.Token, nil), http.StatusUnauthorized, "unauthorized")
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/refresh", "", nil, cookie), http.StatusUnauthorized, "invalid_refresh_token")

	app.Expect(app.Do(http.MethodPatch, "/api/v1/users/"+studentID, admin, map[string]any{"isActive": true}), http.StatusOK, "")
	if auditCount(t, app, audit.ActionUserActivated, studentID) != 1 {
		t.Fatal("expected activated audit")
	}

	other := app.CreateUser("Other", "other@eduflow.dev", "student")
	app.Expect(app.Do(http.MethodPatch, "/api/v1/users/"+other, admin, map[string]any{"email": "stu@eduflow.dev"}), http.StatusConflict, "email_taken")
}

func TestPasswordChangeAndReset(t *testing.T) {
	app := testkit.NewApp(t)
	_, admin := app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	studentID, student := app.UserWithToken("Stu", "stu@eduflow.dev", "student")

	app.Expect(app.Do(http.MethodPost, "/api/v1/users/me/password", student, map[string]string{"currentPassword": "wrongpass1", "newPassword": "newpass123"}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/users/me/password", student, map[string]string{"currentPassword": testkit.Password, "newPassword": testkit.Password}), http.StatusBadRequest, "validation_error")
	app.Expect(app.Do(http.MethodPost, "/api/v1/users/me/password", student, map[string]string{"currentPassword": testkit.Password, "newPassword": "newpass123"}), http.StatusNoContent, "")

	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "stu@eduflow.dev", "password": testkit.Password}), http.StatusUnauthorized, "invalid_credentials")
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "stu@eduflow.dev", "password": "newpass123"}), http.StatusOK, "")

	app.Expect(app.Do(http.MethodPost, "/api/v1/users/"+studentID+"/password", admin, map[string]string{"password": "resetpass9"}), http.StatusNoContent, "")
	app.Expect(app.Do(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "stu@eduflow.dev", "password": "resetpass9"}), http.StatusOK, "")
	if auditCount(t, app, audit.ActionUserPasswordReset, studentID) != 1 {
		t.Fatal("expected password_reset audit")
	}
}

func TestAuditLogListing(t *testing.T) {
	app := testkit.NewApp(t)
	adminID, admin := app.UserWithToken("Admin", "admin@eduflow.dev", "admin")
	studentID := app.CreateUser("Stu", "stu@eduflow.dev", "student")
	app.Do(http.MethodPut, "/api/v1/users/"+studentID+"/roles", admin, map[string]any{"roles": []string{"mentor"}})
	app.Do(http.MethodPost, "/api/v1/users/"+studentID+"/password", admin, map[string]string{"password": "resetpass9"})

	res := app.Do(http.MethodGet, "/api/v1/audit-logs?targetId="+studentID+"&action=user.roles_changed", admin, nil)
	app.Expect(res, http.StatusOK, "")
	var entries []audit.Entry
	res.JSON(&entries)
	if len(entries) != 1 || entries[0].ActorID == nil || *entries[0].ActorID != adminID || entries[0].ActorName == nil || res.Header().Get("X-Total-Count") != "1" {
		t.Fatalf("unexpected audit entries %+v", entries)
	}
	if entries[0].Context["before"] == nil || entries[0].Context["after"] == nil {
		t.Fatalf("missing before/after: %+v", entries[0].Context)
	}

	res = app.Do(http.MethodGet, "/api/v1/audit-logs?targetId="+studentID+"&page=5", admin, nil)
	res.JSON(&entries)
	if len(entries) != 0 || res.Header().Get("X-Total-Count") != "2" {
		t.Fatalf("out-of-range page: %d total=%s", len(entries), res.Header().Get("X-Total-Count"))
	}

	app.Expect(app.Do(http.MethodGet, "/api/v1/audit-logs?targetId=abc", admin, nil), http.StatusBadRequest, "validation_error")

	var leaked int
	_ = app.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE context::text ILIKE '%resetpass9%'`).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("password leaked into audit context")
	}
}
