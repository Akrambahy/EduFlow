CREATE TABLE quizzes (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id          uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    title              text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    description        text NOT NULL DEFAULT '',
    status             text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    time_limit_minutes integer CHECK (time_limit_minutes BETWEEN 1 AND 600),
    opens_at           timestamptz,
    due_at             timestamptz,
    max_attempts       integer NOT NULL DEFAULT 1 CHECK (max_attempts BETWEEN 1 AND 20),
    pass_percent       integer NOT NULL DEFAULT 50 CHECK (pass_percent BETWEEN 0 AND 100),
    reveal_answers     text NOT NULL DEFAULT 'after_close' CHECK (reveal_answers IN ('after_submission', 'after_close', 'never')),
    created_by         uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (opens_at IS NULL OR due_at IS NULL OR due_at > opens_at)
);

CREATE INDEX quizzes_course_idx ON quizzes (course_id);

CREATE TRIGGER quizzes_set_updated_at
    BEFORE UPDATE ON quizzes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE quiz_questions (
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id  uuid NOT NULL REFERENCES quizzes (id) ON DELETE RESTRICT,
    text     text NOT NULL CHECK (length(btrim(text)) BETWEEN 1 AND 2000),
    points   integer NOT NULL DEFAULT 1 CHECK (points BETWEEN 1 AND 100),
    position integer NOT NULL CHECK (position >= 1),
    CONSTRAINT quiz_questions_position_key UNIQUE (quiz_id, position),
    CONSTRAINT quiz_questions_quiz_id_key UNIQUE (quiz_id, id)
);

CREATE TABLE quiz_options (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id uuid NOT NULL REFERENCES quiz_questions (id) ON DELETE RESTRICT,
    text        text NOT NULL CHECK (length(btrim(text)) BETWEEN 1 AND 500),
    is_correct  boolean NOT NULL DEFAULT false,
    position    integer NOT NULL CHECK (position >= 1),
    CONSTRAINT quiz_options_position_key UNIQUE (question_id, position),
    CONSTRAINT quiz_options_question_id_key UNIQUE (question_id, id)
);

CREATE UNIQUE INDEX quiz_options_one_correct ON quiz_options (question_id) WHERE is_correct;

CREATE TABLE quiz_attempts (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id        uuid NOT NULL REFERENCES quizzes (id) ON DELETE RESTRICT,
    student_id     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    attempt_number integer NOT NULL CHECK (attempt_number >= 1),
    status         text NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'submitted', 'expired')),
    started_at     timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz,
    submitted_at   timestamptz,
    score          integer,
    max_score      integer,
    percentage     integer CHECK (percentage BETWEEN 0 AND 100),
    passed         boolean,
    CONSTRAINT quiz_attempts_number_key UNIQUE (quiz_id, student_id, attempt_number)
);

CREATE UNIQUE INDEX quiz_attempts_one_open ON quiz_attempts (quiz_id, student_id) WHERE status = 'in_progress';
CREATE INDEX quiz_attempts_student_idx ON quiz_attempts (student_id);

CREATE TABLE quiz_answers (
    attempt_id  uuid NOT NULL REFERENCES quiz_attempts (id) ON DELETE RESTRICT,
    quiz_id     uuid NOT NULL,
    question_id uuid NOT NULL,
    option_id   uuid NOT NULL,
    PRIMARY KEY (attempt_id, question_id),
    FOREIGN KEY (quiz_id, question_id) REFERENCES quiz_questions (quiz_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (question_id, option_id) REFERENCES quiz_options (question_id, id) ON DELETE RESTRICT
);

CREATE TABLE assignments (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id            uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    title                text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    description          text NOT NULL DEFAULT '',
    status               text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    due_at               timestamptz NOT NULL,
    max_grade            integer NOT NULL DEFAULT 100 CHECK (max_grade BETWEEN 1 AND 1000),
    submission_types     text[] NOT NULL DEFAULT '{file}'
        CHECK (cardinality(submission_types) >= 1 AND submission_types <@ ARRAY['file', 'url', 'text']),
    late_policy          text NOT NULL DEFAULT 'reject' CHECK (late_policy IN ('reject', 'accept', 'penalty')),
    late_until           timestamptz,
    late_penalty_percent integer NOT NULL DEFAULT 0 CHECK (late_penalty_percent BETWEEN 0 AND 100),
    created_by           uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CHECK (late_until IS NULL OR late_until > due_at)
);

CREATE INDEX assignments_course_idx ON assignments (course_id);

CREATE TRIGGER assignments_set_updated_at
    BEFORE UPDATE ON assignments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE assignment_submissions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id uuid NOT NULL REFERENCES assignments (id) ON DELETE RESTRICT,
    student_id    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    version       integer NOT NULL CHECK (version >= 1),
    status        text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'graded', 'returned', 'superseded')),
    submitted_at  timestamptz NOT NULL DEFAULT now(),
    is_late       boolean NOT NULL DEFAULT false,
    file_id       uuid REFERENCES files (id) ON DELETE RESTRICT,
    url           text,
    text_body     text,
    grade         numeric(7, 2) CHECK (grade >= 0),
    final_grade   numeric(7, 2) CHECK (final_grade >= 0),
    feedback      text,
    graded_by     uuid REFERENCES users (id) ON DELETE RESTRICT,
    graded_at     timestamptz,
    CONSTRAINT assignment_submissions_version_key UNIQUE (assignment_id, student_id, version),
    CHECK (file_id IS NOT NULL OR url IS NOT NULL OR text_body IS NOT NULL)
);

CREATE UNIQUE INDEX assignment_submissions_current
    ON assignment_submissions (assignment_id, student_id) WHERE status <> 'superseded';
CREATE INDEX assignment_submissions_student_idx ON assignment_submissions (student_id);
CREATE INDEX assignment_submissions_file_idx ON assignment_submissions (file_id);
