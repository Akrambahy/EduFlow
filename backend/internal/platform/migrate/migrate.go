package migrate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

type Runner struct {
	m *migrate.Migrate
}

func New(databaseURL string) (*Runner, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, driverURL(databaseURL))
	if err != nil {
		return nil, err
	}
	return &Runner{m: m}, nil
}

func (r *Runner) Up() error {
	return ignoreNoChange(r.m.Up())
}

func (r *Runner) Down() error {
	return ignoreNoChange(r.m.Down())
}

func (r *Runner) Steps(n int) error {
	return ignoreNoChange(r.m.Steps(n))
}

func (r *Runner) Force(version int) error {
	return r.m.Force(version)
}

func (r *Runner) Version() (uint, bool, error) {
	v, dirty, err := r.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return v, dirty, err
}

func (r *Runner) Close() error {
	srcErr, dbErr := r.m.Close()
	if srcErr != nil {
		return srcErr
	}
	return dbErr
}

func driverURL(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(databaseURL, prefix) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, prefix)
		}
	}
	return databaseURL
}

func ignoreNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
