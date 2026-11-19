package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Akrambahy/EduFlow/backend/internal/identity"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/config"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type demoUser struct {
	name       string
	email      string
	role       string
	university string
}

var demoUsers = []demoUser{
	{"Sara Admin", "admin@eduflow.dev", identity.RoleAdmin, "EduFlow Institute"},
	{"Ahmed Khalil", "instructor@eduflow.dev", identity.RoleInstructor, "EduFlow Institute"},
	{"Layla Hassan", "mentor@eduflow.dev", identity.RoleMentor, "EduFlow Institute"},
	{"Omar Nabil", "tracker@eduflow.dev", identity.RoleTracker, "EduFlow Institute"},
	{"Marwan Ashour", "student@eduflow.dev", identity.RoleStudent, "EduFlow Institute"},
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.Env == "production" {
		log.Fatal("seed refuses to run when APP_ENV=production")
	}

	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		log.Fatal("SEED_PASSWORD is required")
	}
	hash, err := identity.HashPassword(password)
	if err != nil {
		log.Fatalf("SEED_PASSWORD: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	created, err := seedUsers(ctx, pool, hash)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("users: %d created, %d already present\n", created, len(demoUsers)-created)

	var seeded bool
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		var err error
		seeded, err = seedLearning(ctx, tx)
		return err
	})
	if err != nil {
		log.Fatalf("learning: %v", err)
	}
	if seeded {
		fmt.Printf("learning: %d tracks and %d courses created\n", len(demoTracks), len(demoCourses))
	} else {
		fmt.Println("learning: demo data already present")
	}
}

func seedUsers(ctx context.Context, pool *pgxpool.Pool, hash string) (int, error) {
	created := 0
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for _, u := range demoUsers {
			var id string
			err := tx.QueryRow(ctx, `
				INSERT INTO users (name, email, password_hash, university)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (email) DO NOTHING
				RETURNING id`, u.name, u.email, hash, u.university).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return fmt.Errorf("insert %s: %w", u.email, err)
			}
			tag, err := tx.Exec(ctx, `
				INSERT INTO user_roles (user_id, role_id)
				SELECT $1, id FROM roles WHERE name = $2`, id, u.role)
			if err != nil {
				return fmt.Errorf("assign role %s: %w", u.email, err)
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("role %q not found; run migrations first", u.role)
			}
			created++
		}
		return nil
	})
	return created, err
}
