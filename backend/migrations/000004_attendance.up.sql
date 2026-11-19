CREATE TABLE attendance_sessions (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id        uuid NOT NULL UNIQUE REFERENCES class_sessions (id) ON DELETE RESTRICT,
    status            text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    token_secret      bytea NOT NULL,
    token_ttl_seconds integer NOT NULL DEFAULT 30 CHECK (token_ttl_seconds BETWEEN 10 AND 300),
    grace_minutes     integer NOT NULL DEFAULT 10 CHECK (grace_minutes BETWEEN 0 AND 240),
    opened_by         uuid REFERENCES users (id) ON DELETE RESTRICT,
    opened_at         timestamptz NOT NULL DEFAULT now(),
    closed_by         uuid REFERENCES users (id) ON DELETE RESTRICT,
    closed_at         timestamptz,
    CHECK ((status = 'closed') = (closed_at IS NOT NULL))
);

CREATE TABLE attendance_records (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    attendance_session_id uuid NOT NULL REFERENCES attendance_sessions (id) ON DELETE RESTRICT,
    student_id            uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    status                text NOT NULL CHECK (status IN ('present', 'late', 'absent', 'excused')),
    method                text NOT NULL CHECK (method IN ('qr', 'manual', 'auto')),
    checked_in_at         timestamptz,
    recorded_by           uuid REFERENCES users (id) ON DELETE RESTRICT,
    reason                text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT attendance_records_once UNIQUE (attendance_session_id, student_id)
);

CREATE INDEX attendance_records_student_idx ON attendance_records (student_id);

CREATE TRIGGER attendance_records_set_updated_at
    BEFORE UPDATE ON attendance_records
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
