package identity

import (
	"context"
	"errors"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type UserFilter struct {
	Role     string
	Query    string
	IsActive *bool
	Scoped   bool
	IDs      []string
}

type NewUser struct {
	Name         string
	Email        string
	PasswordHash string
	University   *string
	AvatarURL    *string
	Roles        []string
}

type UserPatch struct {
	Name       *string
	Email      *string
	University *string
	AvatarURL  *string
	IsActive   *bool
}

func (p UserPatch) Fields() []string {
	var out []string
	if p.Name != nil {
		out = append(out, "name")
	}
	if p.Email != nil {
		out = append(out, "email")
	}
	if p.University != nil {
		out = append(out, "university")
	}
	if p.AvatarURL != nil {
		out = append(out, "avatar")
	}
	if p.IsActive != nil {
		out = append(out, "isActive")
	}
	return out
}

const listWhere = `
	WHERE ($1 = '' OR EXISTS (
		SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = u.id AND r.name = $1))
	  AND ($2 = '' OR u.name ILIKE $2 ESCAPE '\' OR u.email ILIKE $2 ESCAPE '\')
	  AND ($3::boolean IS NULL OR u.is_active = $3)
	  AND (NOT $4 OR u.id = ANY($5::uuid[]))`

func (r *Repository) ListUsers(ctx context.Context, f UserFilter, page httpx.Page) ([]User, int, error) {
	pattern := ""
	if q := strings.TrimSpace(f.Query); q != "" {
		pattern = "%" + escapeLike(q) + "%"
	}
	rows, err := r.db.Query(ctx, `SELECT `+userColumns+` FROM users u `+listWhere+`
		ORDER BY u.name, u.id LIMIT $6 OFFSET $7`, f.Role, pattern, f.IsActive, f.Scoped, f.IDs, page.Limit, page.Offset)
	if err != nil {
		return nil, 0, err
	}
	users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (User, error) {
		u, err := scanUser(row)
		if err != nil {
			return User{}, err
		}
		return *u, nil
	})
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM users u `+listWhere, f.Role, pattern, f.IsActive, f.Scoped, f.IDs).Scan(&total); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *Repository) insertUser(ctx context.Context, db database.DBTX, u NewUser) (string, error) {
	var id string
	err := db.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash, university, avatar_url)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		u.Name, u.Email, u.PasswordHash, u.University, u.AvatarURL).Scan(&id)
	if database.IsUniqueViolation(err, "users_email_key") {
		return "", ErrEmailTaken
	}
	return id, err
}

func (r *Repository) replaceRoles(ctx context.Context, db database.DBTX, userID string, roles []string, grantedBy string) error {
	if _, err := db.Exec(ctx, `
		DELETE FROM user_roles ur USING roles r
		WHERE ur.role_id = r.id AND ur.user_id = $1 AND NOT (r.name = ANY($2))`, userID, roles); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, granted_by)
		SELECT $1, r.id, $3 FROM roles r WHERE r.name = ANY($2)
		ON CONFLICT (user_id, role_id) DO NOTHING`, userID, roles, nullableID(grantedBy))
	return err
}

func (r *Repository) lockUser(ctx context.Context, db database.DBTX, id string) (*User, error) {
	return scanUser(db.QueryRow(ctx, `SELECT `+userColumns+` FROM users u WHERE u.id = $1 FOR UPDATE`, id))
}

func (r *Repository) updateUser(ctx context.Context, db database.DBTX, id string, p UserPatch) error {
	_, err := db.Exec(ctx, `
		UPDATE users SET
			name       = COALESCE($2, name),
			email      = COALESCE($3, email),
			university = CASE WHEN $4::text IS NULL THEN university ELSE NULLIF($4, '') END,
			avatar_url = CASE WHEN $5::text IS NULL THEN avatar_url ELSE NULLIF($5, '') END,
			is_active  = COALESCE($6, is_active)
		WHERE id = $1`, id, p.Name, p.Email, p.University, p.AvatarURL, p.IsActive)
	if database.IsUniqueViolation(err, "users_email_key") {
		return ErrEmailTaken
	}
	return err
}

func (r *Repository) setPassword(ctx context.Context, db database.DBTX, id, hash string) error {
	_, err := db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)
	return err
}

func (r *Repository) revokeAll(ctx context.Context, db database.DBTX, userID string) error {
	_, err := db.Exec(ctx, `UPDATE refresh_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (r *Repository) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, r.db, fn)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func nullableID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

var ErrEmailTaken = errors.New("email already in use")
