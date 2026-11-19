package testdb

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/migrate"
	"github.com/Akrambahy/EduFlow/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func URL(t *testing.T) string {
	t.Helper()
	loadEnv()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return u
}

func EmptyURL(t *testing.T) string {
	t.Helper()
	return create(t, "")
}

func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	tmpl, err := template(URL(t))
	if err != nil {
		t.Fatalf("testdb template: %v", err)
	}
	dbURL := create(t, tmpl)
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func create(t *testing.T, tmpl string) string {
	t.Helper()
	base := URL(t)
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("testdb connect: %v", err)
	}
	defer conn.Close(ctx)

	name := "eduflow_t_" + randomSuffix(t)
	stmt := "CREATE DATABASE " + pgx.Identifier{name}.Sanitize()
	if tmpl != "" {
		stmt += " TEMPLATE " + pgx.Identifier{tmpl}.Sanitize()
	}
	if _, err := conn.Exec(ctx, stmt); err != nil {
		t.Fatalf("testdb create: %v", err)
	}
	t.Cleanup(func() { drop(base, name) })
	return withDatabase(t, base, name)
}

var (
	templateOnce sync.Once
	templateName string
	templateErr  error
)

func template(base string) (string, error) {
	templateOnce.Do(func() {
		templateName, templateErr = ensureTemplate(base)
	})
	return templateName, templateErr
}

func ensureTemplate(base string) (string, error) {
	ctx := context.Background()
	digest, err := migrationsDigest()
	if err != nil {
		return "", err
	}
	name := "eduflow_tmpl_" + digest[:12]

	conn, err := pgx.Connect(ctx, base)
	if err != nil {
		return "", err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(727274)`); err != nil {
		return "", err
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock(727274)`)

	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return name, nil
	}

	building := name + "_build"
	_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{building}.Sanitize()+" WITH (FORCE)")
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{building}.Sanitize()); err != nil {
		return "", err
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = "/" + building
	r, err := migrate.New(u.String())
	if err != nil {
		return "", err
	}
	upErr := r.Up()
	_ = r.Close()
	if upErr != nil {
		return "", upErr
	}
	if _, err := conn.Exec(ctx, "ALTER DATABASE "+pgx.Identifier{building}.Sanitize()+" RENAME TO "+pgx.Identifier{name}.Sanitize()); err != nil {
		return "", err
	}

	rows, err := conn.Query(ctx, `SELECT datname FROM pg_database WHERE datname LIKE 'eduflow_tmpl_%' AND datname <> $1`, name)
	if err == nil {
		stale, _ := pgx.CollectRows(rows, pgx.RowTo[string])
		for _, s := range stale {
			_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{s}.Sanitize()+" WITH (FORCE)")
		}
	}
	return name, nil
}

func migrationsDigest() (string, error) {
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return "", err
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		b, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			return "", err
		}
		h.Write([]byte(n))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func drop(base, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := pgx.Connect(ctx, base)
	if err != nil {
		return
	}
	defer c.Close(ctx)
	_, _ = c.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
}

func withDatabase(t *testing.T, base, name string) string {
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func randomSuffix(t *testing.T) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func loadEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
			return
		}
		dir = filepath.Dir(dir)
	}
}
