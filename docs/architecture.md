# Architecture

EduFlow's backend is a modular monolith: a single Go binary exposing a versioned REST API at `/api/v1`, backed by PostgreSQL.

## Modules (`backend/internal`)

| Module | Responsibility | Main tables |
|---|---|---|
| `identity` | Login, refresh rotation, RBAC middleware, user management, profile extras | users, roles, user_roles, refresh_sessions |
| `learning` | Tracks, courses, modules, lessons, enrollments, progress, course visibility rules | tracks, track_mentors, track_trackers, courses, modules, lessons, enrollments, lesson_progress |
| `sessions` | Class sessions, recordings, course materials | class_sessions, learning_materials |
| `attendance` | QR windows, rotating tokens, check-in, corrections, summaries | attendance_sessions, attendance_records |
| `assessment` | Quizzes, attempts and server-side scoring; assignments, submissions and grading | quizzes, quiz_*, assignments, assignment_submissions |
| `projects` | Projects, teams, submissions, presentations, GitHub linking | projects, project_*, presentations, github_repositories |
| `performance` | Versioned grade policies, computed grades, standing, at-risk | grade_policies, standing_snapshots |
| `certificates` | Requirements, eligibility, issuance, revocation, public verification | certificate_requirements, certificates, certificate_events |
| `communication` | In-app notifications and announcements | notifications, announcements |
| `dashboard` | Role-specific summaries built from the modules above | — |
| `platform` | Config, database, HTTP helpers, audit, files and storage, migrations, test kit | audit_logs, files |

## Request flow

1. Gin router: recovery, structured access log, CORS, and the `/api/v1` group.
2. `identity.RequireAuth` validates the Bearer JWT, then reloads the user and roles from the database. Deactivating a user or changing their roles therefore takes effect on the next request.
3. The handler binds and validates the input. Validation failures return `400 validation_error` with camelCase field details.
4. The service applies authorization and business rules, inside a transaction for critical writes.
5. The repository runs hand-written, parameterized SQL through pgx.

## Authorization model

- `learning.CourseVisibility` is the single SQL predicate deciding which courses a user can see:
  - admins see every course;
  - instructors see their own courses;
  - mentors and trackers see published courses in the tracks they are assigned to;
  - students see published courses they are enrolled in.

  Every module reuses this predicate, so the dashboards and resource endpoints apply the same rules (FR-DASH-006).
- Managing a course means being an admin, or being the course instructor who holds the instructor role.
- Reads outside a user's scope return `404`, so the existence of the resource is not revealed. Writes on a resource the user can see but does not manage return `403`.

## Cross-module hooks

To avoid import cycles, modules plug into each other through narrow interfaces:

| Hook | Provided by | Used by |
|---|---|---|
| `identity.ProfileExtender` and `identity.UserScoper` | `learning` | `identity` |
| `files.AccessRule` | sessions, assessment, projects | file downloads |
| `performance.ItemSource` | `projects` | grade computation |
| `communication.Notify*` | `communication` | other modules, inside their own transactions |

## External integrations

| Integration | Approach |
|---|---|
| Storage | `storage.Storage` interface. The local adapter exists today; an S3-compatible adapter will be added once hosting is chosen. |
| GitHub | `projects.GitHubClient`. It is optional and time-boxed, and failures are recorded rather than raised. |
| Email | Not used in the MVP. Password recovery (FR-AUTH-004) is deferred until a provider is chosen. |
| Blockchain | Out of scope (DEC-15). Certificates are verified through the API alone. |
