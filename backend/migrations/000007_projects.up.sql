CREATE TABLE projects (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id             uuid NOT NULL REFERENCES courses (id) ON DELETE RESTRICT,
    title                 text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 160),
    description           text NOT NULL DEFAULT '',
    requirements          text NOT NULL DEFAULT '',
    status                text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    due_at                timestamptz NOT NULL,
    max_grade             integer NOT NULL DEFAULT 100 CHECK (max_grade BETWEEN 1 AND 1000),
    team_size             integer NOT NULL DEFAULT 1 CHECK (team_size BETWEEN 1 AND 10),
    requires_presentation boolean NOT NULL DEFAULT false,
    allow_resubmission    boolean NOT NULL DEFAULT true,
    criteria              jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(criteria) = 'array'),
    created_by            uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX projects_course_idx ON projects (course_id);

CREATE TRIGGER projects_set_updated_at
    BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE project_teams (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    name       text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 80),
    created_by uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT project_teams_project_id_key UNIQUE (project_id, id)
);

CREATE UNIQUE INDEX project_teams_name_key ON project_teams (project_id, lower(name));

CREATE TABLE project_team_members (
    team_id    uuid NOT NULL,
    project_id uuid NOT NULL,
    student_id uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    joined_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, student_id),
    CONSTRAINT project_team_members_one_team UNIQUE (project_id, student_id),
    FOREIGN KEY (project_id, team_id) REFERENCES project_teams (project_id, id) ON DELETE RESTRICT
);

CREATE TABLE project_submissions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    team_id         uuid NOT NULL REFERENCES project_teams (id) ON DELETE RESTRICT,
    version         integer NOT NULL CHECK (version >= 1),
    status          text NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'graded', 'returned', 'superseded')),
    submitted_by    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    submitted_at    timestamptz NOT NULL DEFAULT now(),
    is_late         boolean NOT NULL DEFAULT false,
    repo_url        text,
    file_id         uuid REFERENCES files (id) ON DELETE RESTRICT,
    notes           text,
    grade           numeric(7, 2) CHECK (grade >= 0),
    criteria_scores jsonb,
    feedback        text,
    graded_by       uuid REFERENCES users (id) ON DELETE RESTRICT,
    graded_at       timestamptz,
    CONSTRAINT project_submissions_version_key UNIQUE (team_id, version),
    CHECK (repo_url IS NOT NULL OR file_id IS NOT NULL)
);

CREATE UNIQUE INDEX project_submissions_current ON project_submissions (team_id) WHERE status <> 'superseded';
CREATE INDEX project_submissions_file_idx ON project_submissions (file_id);

CREATE TABLE presentations (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects (id) ON DELETE RESTRICT,
    team_id      uuid NOT NULL UNIQUE REFERENCES project_teams (id) ON DELETE RESTRICT,
    scheduled_at timestamptz NOT NULL,
    location     text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'done', 'cancelled')),
    feedback     text,
    updated_by   uuid REFERENCES users (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER presentations_set_updated_at
    BEFORE UPDATE ON presentations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE github_repositories (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id        uuid NOT NULL UNIQUE REFERENCES project_teams (id) ON DELETE RESTRICT,
    owner          text NOT NULL,
    name           text NOT NULL,
    url            text NOT NULL,
    linked_by      uuid REFERENCES users (id) ON DELETE RESTRICT,
    linked_at      timestamptz NOT NULL DEFAULT now(),
    metadata       jsonb,
    last_synced_at timestamptz,
    sync_error     text
);
