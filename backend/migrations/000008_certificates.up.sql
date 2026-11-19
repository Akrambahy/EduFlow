CREATE TABLE certificate_requirements (
    course_id           uuid PRIMARY KEY REFERENCES courses (id) ON DELETE RESTRICT,
    min_grade           integer NOT NULL DEFAULT 60 CHECK (min_grade BETWEEN 0 AND 100),
    min_attendance      integer NOT NULL DEFAULT 75 CHECK (min_attendance BETWEEN 0 AND 100),
    require_all_lessons boolean NOT NULL DEFAULT true,
    allow_probation     boolean NOT NULL DEFAULT false,
    updated_by          uuid REFERENCES users (id) ON DELETE RESTRICT,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE certificates (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id        uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    course_id         uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    verification_code text NOT NULL UNIQUE CHECK (verification_code ~ '^EF-[0-9]{4}-[A-Z2-9]{8}$'),
    status            text NOT NULL DEFAULT 'issued' CHECK (status IN ('issued', 'revoked')),
    student_name      text NOT NULL,
    course_title      text NOT NULL,
    final_grade       numeric(5, 2),
    issued_by         uuid REFERENCES users (id) ON DELETE RESTRICT,
    issued_at         timestamptz NOT NULL DEFAULT now(),
    revoked_by        uuid REFERENCES users (id) ON DELETE RESTRICT,
    revoked_at        timestamptz,
    revoke_reason     text,
    CHECK ((status = 'revoked') = (revoked_at IS NOT NULL))
);

CREATE UNIQUE INDEX certificates_one_active ON certificates (student_id, course_id) WHERE status = 'issued';
CREATE INDEX certificates_course_idx ON certificates (course_id);

CREATE TABLE certificate_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    certificate_id uuid NOT NULL REFERENCES certificates (id) ON DELETE RESTRICT,
    action         text NOT NULL CHECK (action IN ('issued', 'revoked', 'reinstated')),
    actor_id       uuid REFERENCES users (id) ON DELETE RESTRICT,
    reason         text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX certificate_events_certificate_idx ON certificate_events (certificate_id, created_at);
