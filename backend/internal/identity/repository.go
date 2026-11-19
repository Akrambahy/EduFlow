package identity

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrSessionInvalid = errors.New("refresh session invalid")
	ErrSessionReused  = errors.New("refresh session reused")
)

type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	AvatarURL    *string
	University   *string
	IsActive     bool
	Roles        []string
	CreatedAt    time.Time
}

type SessionMeta struct {
	UserAgent string
	IP        string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const userColumns = `
	u.id, u.name, u.email, u.password_hash, u.avatar_url, u.university, u.is_active, u.created_at,
	ARRAY(
		SELECT r.name FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = u.id ORDER BY r.name
	)`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.AvatarURL, &u.University, &u.IsActive, &u.CreatedAt, &u.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Roles = SortRoles(u.Roles)
	return &u, nil
}

func (r *Repository) UserByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users u WHERE u.email = $1`, email))
}

func (r *Repository) UserByID(ctx context.Context, id string) (*User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users u WHERE u.id = $1`, id))
}

func (r *Repository) TouchLogin(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, userID string, hash []byte, expires time.Time, meta SessionMeta) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_sessions (user_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, hash, truncate(meta.UserAgent, 512), parseIP(meta.IP), expires)
	return err
}

func (r *Repository) RotateSession(ctx context.Context, oldHash, newHash []byte, expires time.Time, meta SessionMeta) (string, error) {
	var userID string
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var (
			id        string
			expiresAt time.Time
			revokedAt *time.Time
		)
		err := tx.QueryRow(ctx, `
			SELECT id, user_id, expires_at, revoked_at
			FROM refresh_sessions WHERE token_hash = $1 FOR UPDATE`, oldHash).
			Scan(&id, &userID, &expiresAt, &revokedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionInvalid
		}
		if err != nil {
			return err
		}
		if revokedAt != nil {
			return ErrSessionReused
		}
		if !expiresAt.After(time.Now()) {
			return ErrSessionInvalid
		}

		var newID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO refresh_sessions (user_id, token_hash, user_agent, ip_address, expires_at)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			userID, newHash, truncate(meta.UserAgent, 512), parseIP(meta.IP), expires).Scan(&newID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE refresh_sessions SET revoked_at = now(), replaced_by = $2 WHERE id = $1`, id, newID)
		return err
	})
	return userID, err
}

func (r *Repository) RevokeSession(ctx context.Context, hash []byte) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	return err
}

func (r *Repository) RevokeAllSessions(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func parseIP(s string) *netip.Addr {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &addr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
