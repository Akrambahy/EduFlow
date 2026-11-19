package migrate_test

import (
	"context"
	"testing"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/migrate"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/testdb"
	"github.com/jackc/pgx/v5"
)

func TestMigrationsUpDownUp(t *testing.T) {
	url := testdb.EmptyURL(t)
	ctx := context.Background()

	r, err := migrate.New(url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if err := r.Down(); err != nil {
		t.Fatalf("initial down: %v", err)
	}
	if err := r.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	var roles int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM roles`).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if roles != 5 {
		t.Fatalf("roles = %d, want 5", roles)
	}

	var users int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Fatalf("migrations must not seed users, found %d", users)
	}

	if _, err := conn.Exec(ctx, `INSERT INTO users (name, email, password_hash) VALUES ('A', 'Upper@x.dev', 'h')`); err == nil {
		t.Fatal("expected email lowercase check to fail")
	}
	if _, err := conn.Exec(ctx, `INSERT INTO users (name, email, password_hash) VALUES ('A', 'dup@x.dev', 'h')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO users (name, email, password_hash) VALUES ('B', 'dup@x.dev', 'h')`); err == nil {
		t.Fatal("expected duplicate email to fail")
	}

	if err := r.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if v, _, _ := r.Version(); v != 0 {
		t.Fatalf("version after down = %d", v)
	}
	if err := r.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
	if _, dirty, _ := r.Version(); dirty {
		t.Fatal("database left dirty")
	}
}
