package audit

import (
	"context"
	"encoding/json"
	"net/netip"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ActionUserCreated       = "user.created"
	ActionUserUpdated       = "user.updated"
	ActionUserActivated     = "user.activated"
	ActionUserDeactivated   = "user.deactivated"
	ActionUserRolesChanged  = "user.roles_changed"
	ActionUserPasswordReset = "user.password_reset"
	ActionPasswordChanged   = "user.password_changed"
)

type Event struct {
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	Context    map[string]any
	IP         string
}

type Entry struct {
	ID         string         `json:"id"`
	ActorID    *string        `json:"actorId"`
	ActorName  *string        `json:"actorName"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   *string        `json:"targetId"`
	Context    map[string]any `json:"context"`
	IPAddress  *string        `json:"ipAddress"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type Filter struct {
	Action     string
	TargetType string
	TargetID   string
	ActorID    string
}

func Record(ctx context.Context, db database.DBTX, e Event) error {
	payload := e.Context
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, context, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		nullable(e.ActorID), e.Action, e.TargetType, nullable(e.TargetID), raw, parseIP(e.IP))
	return err
}

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) List(ctx context.Context, f Filter, page httpx.Page) ([]Entry, int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT a.id, a.actor_id, u.name, a.action, a.target_type, a.target_id, a.context,
		       host(a.ip_address), a.created_at, count(*) OVER()
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_id
		WHERE ($1 = '' OR a.action = $1)
		  AND ($2 = '' OR a.target_type = $2)
		  AND ($3::uuid IS NULL OR a.target_id = $3::uuid)
		  AND ($4::uuid IS NULL OR a.actor_id = $4::uuid)
		ORDER BY a.created_at DESC, a.id
		LIMIT $5 OFFSET $6`,
		f.Action, f.TargetType, nullable(f.TargetID), nullable(f.ActorID), page.Limit, page.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries := []Entry{}
	total := 0
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Action, &e.TargetType, &e.TargetID, &e.Context, &e.IPAddress, &e.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(entries) == 0 && page.Offset > 0 {
		err = s.db.QueryRow(ctx, `
			SELECT count(*) FROM audit_logs a
			WHERE ($1 = '' OR a.action = $1)
			  AND ($2 = '' OR a.target_type = $2)
			  AND ($3::uuid IS NULL OR a.target_id = $3::uuid)
			  AND ($4::uuid IS NULL OR a.actor_id = $4::uuid)`,
			f.Action, f.TargetType, nullable(f.TargetID), nullable(f.ActorID)).Scan(&total)
		if err != nil {
			return nil, 0, err
		}
	}
	return entries, total, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func parseIP(s string) *netip.Addr {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &addr
}
