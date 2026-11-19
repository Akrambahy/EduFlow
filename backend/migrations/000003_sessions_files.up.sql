CREATE TABLE files (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id      uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    original_name text NOT NULL CHECK (length(original_name) BETWEEN 1 AND 255),
    content_type  text NOT NULL,
    extension     text NOT NULL,
    size_bytes    bigint NOT NULL CHECK (size_bytes > 0),
    sha256        bytea NOT NULL,
    storage_key   text NOT NULL UNIQUE,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX files_owner_idx ON files (owner_id);

CREATE TABLE class_sessions (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id         uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    instructor_id     uuid REFERENCES users (id) ON DELETE RESTRICT,
    title             text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    description       text NOT NULL DEFAULT '',
    type              text NOT NULL DEFAULT 'lecture' CHECK (type IN ('lecture', 'lab', 'workshop', 'review')),
    location          text NOT NULL DEFAULT '',
    starts_at         timestamptz NOT NULL,
    ends_at           timestamptz NOT NULL,
    cancelled_at      timestamptz,
    recording_url     text,
    recording_file_id uuid REFERENCES files (id) ON DELETE RESTRICT,
    created_by        uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at),
    CHECK (ends_at - starts_at <= interval '12 hours')
);

CREATE INDEX class_sessions_course_starts_idx ON class_sessions (course_id, starts_at);
CREATE INDEX class_sessions_starts_idx ON class_sessions (starts_at);

CREATE TRIGGER class_sessions_set_updated_at
    BEFORE UPDATE ON class_sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE learning_materials (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id   uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    session_id  uuid REFERENCES class_sessions (id) ON DELETE RESTRICT,
    title       text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    description text NOT NULL DEFAULT '',
    file_id     uuid REFERENCES files (id) ON DELETE RESTRICT,
    url         text,
    created_by  uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK ((file_id IS NULL) <> (url IS NULL))
);

CREATE INDEX learning_materials_course_idx ON learning_materials (course_id);
CREATE INDEX learning_materials_file_idx ON learning_materials (file_id);

CREATE TRIGGER learning_materials_set_updated_at
    BEFORE UPDATE ON learning_materials
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
