# Testing

## Layers

| Layer | Where | Examples |
|---|---|---|
| Unit | `*_test.go` next to the pure rules | QR token expiry and tampering; quiz scoring; late penalties; grade and standing calculation; certificate eligibility; repository URL parsing; file type sniffing |
| HTTP integration | `<module>/*_http_test.go` | Real Gin router and real PostgreSQL. Covers 401, 403, 404, 409, 400, 413, 415 and 429 for every module |
| Concurrency | `attendance`, `learning`, `assessment` | 10 parallel check-ins create one record; parallel enrollment; parallel quiz starts |
| Contract | `platform/server/openapi_test.go` | Every registered route must appear in `docs/api/openapi.yaml`, and every documented operation must exist |
| Migrations | `platform/migrate` | Runs up, then down, then up on an empty database |

## How the integration tests work

- `testdb.New(t)` builds a template database once per run, applying every migration (`eduflow_tmpl_<hash of migrations>`). It then clones a fresh database for each test with `CREATE DATABASE … TEMPLATE`, and drops it afterwards.
- Tests run in parallel and never share state.
- `testkit.NewApp(t)` wires the full server with:
  - a temporary storage folder;
  - an offline GitHub stub;
  - fast password hashing for fixture users.

## Running

```bash
cd backend
/d/Tools/pgsql/bin/createdb.exe -U eduflow -h localhost eduflow_test
go test ./...
```

If `TEST_DATABASE_URL` is unset, database tests are skipped.

On Windows, creating databases dominates the run time; the full suite takes about 2 minutes. To speed up a cluster that is only used for development and testing, never production:

```bash
/d/Tools/pgsql/bin/psql.exe -U eduflow -h localhost -d eduflow -c "ALTER SYSTEM SET fsync = off" -c "ALTER SYSTEM SET synchronous_commit = off"
/d/Tools/pgsql/bin/pg_ctl.exe -D /d/Tools/pgdata restart
```

CI applies the same settings to its throwaway Postgres service.
