# Deployment and operations

## CI (`.github/workflows/backend.yml`)

The workflow runs on pull requests and on pushes to `main` that touch `backend/` or `docs/api/`.

| Job | Steps |
|---|---|
| `test` | `gofmt`, `go vet`, `staticcheck`, `go test -race` against a Postgres 16 service, `go build`, `govulncheck` |
| `secrets` | gitleaks over the full git history (`.gitleaks.toml` allows the documented placeholders) |
| `docker` | Builds the backend image |

## Container

- `backend/Dockerfile` is a multi-stage build that produces static binaries (`server`, `migrate`, `seed`) on a distroless, non-root image.
- `docker compose --profile api up --build` runs Postgres, then the migrations, then the API.
- Uploaded files are written to `STORAGE_DIR`. Mount a volume there, or switch to object storage before running more than one instance.

## Before production (SRS 5.4 and 12.3)

- Choose the hosting target and add the S3-compatible storage adapter.
- Set `APP_ENV=production`, a fresh `JWT_SECRET`, HTTPS, and `ALLOWED_ORIGINS` limited to the real frontend origin.
- Put the API behind a reverse proxy on the same origin as the frontend, so the refresh cookie works with `SameSite=Strict`.
- The rate limiters are kept in memory. With more than one instance, move them to a shared store and configure trusted proxies.
- Backups: schedule `pg_dump` (or managed snapshots), keep them for the agreed retention period, and test a restore before launch:

```bash
pg_dump -Fc -h <host> -U eduflow eduflow > eduflow-$(date +%F).dump
pg_restore -c -d eduflow_restore eduflow-YYYY-MM-DD.dump
```

- Monitoring: watch `/api/v1/health` (it checks the database), error rates, and attendance check-in failures.
- Disaster recovery cannot be called complete until a restore has been tested.
