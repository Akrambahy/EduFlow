CREATE TABLE grade_policies (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id            uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    version              integer NOT NULL CHECK (version >= 1),
    quiz_weight          integer NOT NULL CHECK (quiz_weight BETWEEN 0 AND 100),
    assignment_weight    integer NOT NULL CHECK (assignment_weight BETWEEN 0 AND 100),
    attendance_weight    integer NOT NULL CHECK (attendance_weight BETWEEN 0 AND 100),
    project_weight       integer NOT NULL CHECK (project_weight BETWEEN 0 AND 100),
    warning_grade        integer NOT NULL CHECK (warning_grade BETWEEN 0 AND 100),
    probation_grade      integer NOT NULL CHECK (probation_grade BETWEEN 0 AND 100),
    warning_attendance   integer NOT NULL CHECK (warning_attendance BETWEEN 0 AND 100),
    probation_attendance integer NOT NULL CHECK (probation_attendance BETWEEN 0 AND 100),
    warning_missing      integer NOT NULL CHECK (warning_missing BETWEEN 1 AND 100),
    reason               text,
    created_by           uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT grade_policies_version_key UNIQUE (course_id, version),
    CHECK (quiz_weight + assignment_weight + attendance_weight + project_weight = 100),
    CHECK (probation_grade <= warning_grade),
    CHECK (probation_attendance <= warning_attendance)
);

CREATE TABLE standing_snapshots (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id  uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    course_id   uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    policy_id   uuid REFERENCES grade_policies (id) ON DELETE RESTRICT,
    overall     numeric(5, 2),
    standing    text NOT NULL CHECK (standing IN ('good', 'warning', 'probation')),
    components  jsonb NOT NULL,
    factors     jsonb NOT NULL,
    computed_by uuid REFERENCES users (id) ON DELETE RESTRICT,
    computed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX standing_snapshots_student_course_idx ON standing_snapshots (student_id, course_id, computed_at DESC);
