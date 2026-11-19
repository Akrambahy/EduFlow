package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/config"
	"github.com/Akrambahy/EduFlow/backend/internal/platform/migrate"
)

const usage = "usage: migrate up | down [n] | force <version> | version"

func main() {
	if len(os.Args) < 2 {
		log.Fatal(usage)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	runner, err := migrate.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer runner.Close()

	if err := run(runner, cfg.Env, os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	v, dirty, err := runner.Version()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("version=%d dirty=%t\n", v, dirty)
}

func run(r *migrate.Runner, env string, args []string) error {
	switch args[0] {
	case "up":
		return r.Up()
	case "down":
		if env == "production" {
			return fmt.Errorf("down is disabled when APP_ENV=production")
		}
		if len(args) > 1 {
			n, err := strconv.Atoi(args[1])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid step count %q", args[1])
			}
			return r.Steps(-n)
		}
		return r.Down()
	case "force":
		if len(args) < 2 {
			return fmt.Errorf(usage)
		}
		v, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid version %q", args[1])
		}
		return r.Force(v)
	case "version":
		return nil
	default:
		return fmt.Errorf(usage)
	}
}
