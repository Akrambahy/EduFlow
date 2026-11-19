CREATE TABLE tracks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    created_by  uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tracks_name_key ON tracks (lower(name));

CREATE TRIGGER tracks_set_updated_at
    BEFORE UPDATE ON tracks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE track_mentors (
    track_id    uuid NOT NULL REFERENCES tracks (id) ON DELETE RESTRICT,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    assigned_by uuid REFERENCES users (id) ON DELETE RESTRICT,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (track_id, user_id)
);

CREATE INDEX track_mentors_user_idx ON track_mentors (user_id);

CREATE TABLE track_trackers (
    track_id    uuid NOT NULL REFERENCES tracks (id) ON DELETE RESTRICT,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    assigned_by uuid REFERENCES users (id) ON DELETE RESTRICT,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (track_id, user_id)
);

CREATE INDEX track_trackers_user_idx ON track_trackers (user_id);

CREATE TABLE courses (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    track_id          uuid REFERENCES tracks (id) ON DELETE RESTRICT,
    instructor_id     uuid REFERENCES users (id) ON DELETE RESTRICT,
    title             text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    description       text NOT NULL DEFAULT '',
    category          text NOT NULL DEFAULT 'General' CHECK (length(category) BETWEEN 1 AND 50),
    color             text NOT NULL DEFAULT 'teal' CHECK (color IN ('teal', 'emerald', 'sky', 'amber', 'violet', 'rose')),
    duration_weeks    integer NOT NULL DEFAULT 1 CHECK (duration_weeks BETWEEN 1 AND 104),
    sequential_unlock boolean NOT NULL DEFAULT false,
    status            text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    published_at      timestamptz,
    created_by        uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX courses_track_idx ON courses (track_id);
CREATE INDEX courses_instructor_idx ON courses (instructor_id);
CREATE INDEX courses_status_idx ON courses (status);

CREATE TRIGGER courses_set_updated_at
    BEFORE UPDATE ON courses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE modules (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id  uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    title      text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    position   integer NOT NULL CHECK (position >= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT modules_course_position_key UNIQUE (course_id, position) DEFERRABLE INITIALLY DEFERRED
);

CREATE TRIGGER modules_set_updated_at
    BEFORE UPDATE ON modules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE lessons (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id        uuid NOT NULL REFERENCES modules (id) ON DELETE RESTRICT,
    title            text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    type             text NOT NULL DEFAULT 'text' CHECK (type IN ('video', 'text', 'project')),
    duration_minutes integer NOT NULL DEFAULT 10 CHECK (duration_minutes BETWEEN 1 AND 1440),
    content          text NOT NULL DEFAULT '',
    video_url        text,
    position         integer NOT NULL CHECK (position >= 1),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT lessons_module_position_key UNIQUE (module_id, position) DEFERRABLE INITIALLY DEFERRED
);

CREATE TRIGGER lessons_set_updated_at
    BEFORE UPDATE ON lessons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE enrollments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    course_id    uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'withdrawn')),
    enrolled_by  uuid REFERENCES users (id) ON DELETE RESTRICT,
    enrolled_at  timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    withdrawn_at timestamptz,
    CHECK ((status = 'withdrawn') = (withdrawn_at IS NOT NULL))
);

CREATE UNIQUE INDEX enrollments_current_key
    ON enrollments (student_id, course_id)
    WHERE status IN ('active', 'completed');

CREATE INDEX enrollments_course_idx ON enrollments (course_id) WHERE status IN ('active', 'completed');

CREATE TABLE lesson_progress (
    student_id   uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    lesson_id    uuid NOT NULL REFERENCES lessons (id) ON DELETE RESTRICT,
    started_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    PRIMARY KEY (student_id, lesson_id)
);

CREATE INDEX lesson_progress_lesson_idx ON lesson_progress (lesson_id);
