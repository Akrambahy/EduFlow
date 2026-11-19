# EduFlow

EduFlow is a learning management system built for the DEPI (رواد مصر) Go track. It has a Go + Gin REST API, PostgreSQL, and a React frontend.

| Area | Location |
|---|---|
| Backend (Go 1.27, Gin, pgx v5, golang-migrate) | `backend/` |
| Frontend (React + Vite) | `frontend/` |
| API contract | `docs/api/openapi.yaml` |
| Architecture, data model, security, testing, decisions | `docs/` |

## Backend quick start (Git Bash on Windows, no Docker)

**Prerequisites**
- Go 1.27.4 or newer. With `GOTOOLCHAIN=auto`, the default, Go downloads the pinned toolchain automatically.
- PostgreSQL 16. A portable install works; this guide assumes `D:\Tools\pgsql`.

**1. Create a local cluster and databases.** Run this once. Trust auth is used, and it listens on localhost only.

```bash
/d/Tools/pgsql/bin/initdb.exe -D /d/Tools/pgdata -U eduflow --auth=trust -E UTF8 --locale=C
/d/Tools/pgsql/bin/pg_ctl.exe -D /d/Tools/pgdata -l /d/Tools/pgdata/server.log -o "-c listen_addresses=localhost" start
/d/Tools/pgsql/bin/createdb.exe -U eduflow -h localhost eduflow
/d/Tools/pgsql/bin/createdb.exe -U eduflow -h localhost eduflow_test
```

**2. Configure the backend.**

```bash
cd backend
sed "s|^JWT_SECRET=.*|JWT_SECRET=$(openssl rand -hex 32)|" .env.example > .env
```

**3. Migrate, seed demo data, and run.**

```bash
go run ./cmd/migrate up
go run ./cmd/seed
go run ./cmd/server
```

Check that it is up:

```bash
curl -s localhost:8080/api/v1/health
```

**Demo accounts.** Use the password in `SEED_PASSWORD`.

| Role | Email |
|---|---|
| Admin | admin@eduflow.dev |
| Instructor | instructor@eduflow.dev |
| Mentor | mentor@eduflow.dev |
| Tracker | tracker@eduflow.dev |
| Student | student@eduflow.dev |

The seed refuses to run when `APP_ENV=production`. Migrations never create users.

**Starting and stopping Postgres later**

```bash
/d/Tools/pgsql/bin/pg_ctl.exe -D /d/Tools/pgdata -l /d/Tools/pgdata/server.log start
/d/Tools/pgsql/bin/pg_ctl.exe -D /d/Tools/pgdata stop
```

## Docker

```bash
JWT_SECRET=$(openssl rand -hex 32) docker compose --profile api up --build
```

This starts PostgreSQL, applies migrations, and serves the API on `:8080`. Without `--profile api`, only PostgreSQL starts.

## Commands

| Command | Purpose |
|---|---|
| `go run ./cmd/migrate up` | Apply all migrations |
| `go run ./cmd/migrate down 1` | Roll back one migration (disabled in production) |
| `go run ./cmd/migrate version` | Show the current version |
| `go run ./cmd/seed` | Create the demo users and learning data (idempotent) |
| `go test ./...` | Unit and HTTP integration tests (needs `TEST_DATABASE_URL`) |
| `go vet ./...` and `gofmt -l .` | Static checks |

## Environment variables

| Name | Default | Notes |
|---|---|---|
| `APP_ENV` | `development` | `production` enables secure cookies and blocks the seed and `migrate down` |
| `PORT` | `8080` | |
| `DATABASE_URL` | required | e.g. `postgres://eduflow:eduflow@localhost:5432/eduflow?sslmode=disable` |
| `ALLOWED_ORIGINS` | `http://localhost:5173` | Comma-separated CORS origins |
| `JWT_SECRET` | required | At least 32 bytes |
| `ACCESS_TOKEN_TTL` | `2h` | Lower it to `15m` once the frontend refreshes on 401 |
| `REFRESH_TOKEN_TTL` | `168h` | |
| `COOKIE_SECURE` | `true` in production | |
| `STORAGE_DIR` | `./storage` | Local file storage (git-ignored) |
| `MAX_UPLOAD_MB` | `20` | |
| `APP_TIMEZONE` | `Africa/Cairo` | Used for `date`, `startTime` and `dueDate` display fields |
| `ATTENDANCE_TOKEN_TTL` | `30s` | QR rotation |
| `ATTENDANCE_GRACE_MINUTES` | `10` | Check-ins after this become `late` |
| `GITHUB_TOKEN` | empty | Optional; raises the GitHub API rate limit for repository metadata |
| `SEED_PASSWORD` | — | Password for the demo accounts |
| `TEST_DATABASE_URL` | — | Database used to create a throwaway database for each test |

## Frontend integration

- Set `VITE_API_BASE_URL=/api/v1`. The Vite proxy already forwards `/api` to `localhost:8080`.
- Login is `POST /auth/login` and returns `{token, user}`. Store the token as `eduflow_token` and send it as `Authorization: Bearer <token>`.
- Responses have no envelope. Errors look like `{message, code, details?}`. Lists return arrays, with the total in the `X-Total-Count` header.

## Documentation

- `docs/api/openapi.yaml`: every endpoint, with its roles and error codes. A test fails if a route is missing from it.
- `docs/architecture.md`: modules and request flow.
- `docs/database.md`: tables, constraints and migration policy.
- `docs/security.md`: authentication, QR attendance and the threat model.
- `docs/testing.md`: test strategy and how to run the tests.
- `docs/decisions.md`: answers to the open decisions in SRS section 15.
- `docs/deployment.md`: CI, containers, backups.
