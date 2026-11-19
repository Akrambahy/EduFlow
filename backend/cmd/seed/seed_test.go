package main

import (
	"context"
	"testing"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/testdb"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func TestSeedIsIdempotent(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("seedpass1"), bcrypt.MinCost)

	for run := 0; run < 2; run++ {
		created, err := seedUsers(ctx, pool, string(hash))
		if err != nil {
			t.Fatal(err)
		}
		if want := map[int]int{0: 5, 1: 0}[run]; created != want {
			t.Fatalf("run %d: created %d users, want %d", run, created, want)
		}
		var seeded bool
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			seeded, err = seedLearning(ctx, tx)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if seeded != (run == 0) {
			t.Fatalf("run %d: seeded=%t", run, seeded)
		}
	}

	var tracks, courses, lessons, enrollments int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM tracks), (SELECT count(*) FROM courses),
		(SELECT count(*) FROM lessons), (SELECT count(*) FROM enrollments)`).Scan(&tracks, &courses, &lessons, &enrollments); err != nil {
		t.Fatal(err)
	}
	if tracks != 3 || courses != 6 || lessons != 57 || enrollments != 4 {
		t.Fatalf("tracks=%d courses=%d lessons=%d enrollments=%d", tracks, courses, lessons, enrollments)
	}
}
