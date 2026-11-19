CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    type       text NOT NULL CHECK (type IN ('assignment', 'grade', 'session', 'announcement', 'certificate', 'attendance', 'system')),
    title      text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
    message    text NOT NULL DEFAULT '',
    link       text,
    dedupe_key text NOT NULL,
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notifications_dedupe_key UNIQUE (user_id, dedupe_key)
);

CREATE INDEX notifications_user_created_idx ON notifications (user_id, created_at DESC);
CREATE INDEX notifications_user_unread_idx ON notifications (user_id) WHERE read_at IS NULL;

CREATE TABLE announcements (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title        text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 200),
    content      text NOT NULL CHECK (length(content) BETWEEN 1 AND 20000),
    author_id    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    course_id    uuid REFERENCES courses (id) ON DELETE RESTRICT,
    target_roles text[] NOT NULL DEFAULT '{}'
        CHECK (target_roles <@ ARRAY['admin', 'instructor', 'mentor', 'tracker', 'student']),
    created_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz
);

CREATE INDEX announcements_created_idx ON announcements (created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX announcements_course_idx ON announcements (course_id);
