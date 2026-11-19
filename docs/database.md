# Database

The database is PostgreSQL 16. The schema is defined by the versioned migrations in `backend/migrations`. Those migrations are embedded in the binaries and applied with `cmd/migrate`.

| Version | Content |
|---|---|
| 000001 | roles (seeded), users, user_roles, refresh_sessions, audit_logs |
| 000002 | tracks, track_mentors, track_trackers, courses, modules, lessons, enrollments, lesson_progress |
| 000003 | files, class_sessions, learning_materials |
| 000004 | attendance_sessions, attendance_records |
| 000005 | quizzes, quiz_questions, quiz_options, quiz_attempts, quiz_answers, assignments, assignment_submissions |
| 000006 | grade_policies, standing_snapshots |
| 000007 | projects, project_teams, project_team_members, project_submissions, presentations, github_repositories |
| 000008 | certificate_requirements, certificates, certificate_events |
| 000009 | notifications, announcements |

## Conventions

**Keys and types**
- Primary keys are `uuid DEFAULT gen_random_uuid()`.
- All timestamps are `timestamptz`, stored in UTC.
- Emails are stored in lowercase, with a unique index.

**Deletes and history**
- Foreign keys use `ON DELETE RESTRICT`, so academic records are never cascade-deleted (SRS 8.3).
- Records are deactivated, archived, withdrawn, superseded or revoked rather than deleted.

**Invariants enforced in the database**

| Rule | Constraint |
|---|---|
| One current enrollment per student and course | Partial unique index `enrollments_current_key` |
| One attendance record per student and window | `UNIQUE (attendance_session_id, student_id)` |
| One open quiz attempt per student | Partial unique index `quiz_attempts_one_open` |
| A quiz answer belongs to the quiz's own questions and options | Composite foreign keys |
| One current submission per student or team | Partial unique indexes on `status <> 'superseded'` |
| One active certificate per student and course | Partial unique index `certificates_one_active` |
| Grade policy weights sum to 100 | Check constraint |

**Ordering and timestamps**
- Ordered children use `UNIQUE … DEFERRABLE INITIALLY DEFERRED`, so reordering can happen inside a single transaction.
- `updated_at` is maintained by the `set_updated_at()` trigger.

## Migration policy

- Never edit a migration once it is merged. Add a new one instead.
- Every `up` migration has a matching `down`. The migration test runs up, then down, then up.
- `migrate down` is refused when `APP_ENV=production`.
- Seed data for development lives in `cmd/seed`, never in migrations. The only exception is the five roles.

## Retention (open before production, SRS 5.4)

These records are kept indefinitely until retention periods are agreed:
- audit logs;
- submissions;
- files;
- attendance;
- certificate events.

Refresh sessions can be purged once they are 30 days past expiry.
