CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE roles (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL UNIQUE CHECK (name = lower(name)),
    description text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO roles (name, description) VALUES
    ('admin', 'Manages users, roles, tracks, courses, settings, reports, certificates and audit events'),
    ('instructor', 'Manages assigned courses, sessions, attendance, assessments, submissions and grades'),
    ('mentor', 'Supports assigned students and reviews progress, projects and feedback'),
    ('tracker', 'Monitors attendance, progress, missing work, standing and at-risk indicators'),
    ('student', 'Learns, attends, submits work and views own grades and certificates');

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name          text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    email         text NOT NULL CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
    password_hash text NOT NULL,
    avatar_url    text,
    university    text,
    is_active     boolean NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_key ON users (email);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_roles (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    granted_by uuid REFERENCES users (id) ON DELETE RESTRICT,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_roles_role_id_idx ON user_roles (role_id);

CREATE TABLE refresh_sessions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    token_hash  bytea NOT NULL UNIQUE,
    user_agent  text,
    ip_address  inet,
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    replaced_by uuid REFERENCES refresh_sessions (id) ON DELETE RESTRICT,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_sessions_user_active_idx
    ON refresh_sessions (user_id)
    WHERE revoked_at IS NULL;

CREATE TABLE audit_logs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    uuid REFERENCES users (id) ON DELETE RESTRICT,
    action      text NOT NULL,
    target_type text NOT NULL,
    target_id   uuid,
    context     jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip_address  inet,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at DESC);
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id);
