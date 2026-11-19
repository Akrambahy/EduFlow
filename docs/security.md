# EduFlow Security Notes

## Authentication

| Item | Decision |
|---|---|
| Account creation | Admin-managed only (DEC-01). There is no public sign-up endpoint. |
| Password storage | bcrypt, cost 12. Passwords must be 8–72 characters and contain a letter and a digit. |
| Access token | HS256 JWT signed with `JWT_SECRET` (at least 32 bytes). Claims: `iss=eduflow`, `sub=<user id>`, `roles`, `iat`, `nbf`, `exp`. The client sends it as `Authorization: Bearer <token>` and stores it in `localStorage` under `eduflow_token`. |
| Access token lifetime | `ACCESS_TOKEN_TTL`, default `2h` for the MVP (see "Refresh strategy" below). |
| Refresh token | 32 random bytes, base64url. Sent only as the `eduflow_refresh` cookie: `HttpOnly`, `SameSite=Strict`, `Path=/api/v1/auth`, and `Secure` when `COOKIE_SECURE=true` (required in production). |
| Refresh storage | Only the SHA-256 hash is stored, in `refresh_sessions`, together with the user agent, IP address and expiry. |
| Refresh lifetime | `REFRESH_TOKEN_TTL`, default `168h` (7 days). |

### Endpoints

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/auth/login` | none, rate-limited | Returns `{token, tokenType, expiresIn, user}` and sets the refresh cookie. |
| POST | `/api/v1/auth/refresh` | refresh cookie, rate-limited | Rotates the refresh token and returns a new access token. |
| POST | `/api/v1/auth/logout` | refresh cookie (optional) | Revokes the session and clears the cookie. Always returns 204. |
| GET | `/api/v1/auth/me` | Bearer | Returns the current user. |

### Refresh strategy

Every refresh rotates the token:
1. The old session is marked revoked and linked to the new one through `replaced_by`.
2. If a revoked token is presented again, the server treats it as reuse. It revokes every session for that user, and the user must sign in again.

The frontend does not yet call `/auth/refresh` when it gets a 401. Until it does, the access token lifetime is 2 hours, so a working session does not end every few minutes.

This is a deliberate MVP trade-off: a stolen access token remains valid for up to 2 hours. To limit the damage, every authenticated request reloads the user and their roles from the database:
- deactivating an account takes effect immediately;
- role changes take effect immediately, even though the token still carries the old roles.

Once the frontend adds refresh-on-401, lower `ACCESS_TOKEN_TTL` to `15m`.

The refresh cookie only reaches the API when it shares the frontend's origin. That is true for the Vite proxy in development and for a reverse proxy in production. A cross-origin setup would also need `credentials: "include"` on the frontend.

### Error behaviour

| Case | Status | Code |
|---|---|---|
| Unknown email or wrong password | 401 | `invalid_credentials` |
| Correct password, deactivated account | 403 | `account_disabled` |
| Missing, malformed, expired or forged access token | 401 | `unauthorized` |
| Role not allowed | 403 | `forbidden` |
| Bad, expired, revoked or reused refresh token | 401 | `invalid_refresh_token` (the cookie is cleared) |
| Rate limit exceeded | 429 | `rate_limited` (includes a `Retry-After` header) |

An unknown email and a wrong password produce the same response body. An unknown email still runs a bcrypt comparison, so response timing does not reveal whether an account exists.

## Rate limiting

Limits are in-memory, per client IP:

| Endpoint | Limit |
|---|---|
| Login | burst of 5, then 1 request per 12 seconds |
| Refresh | burst of 10, then 1 request per 6 seconds |

Gin is configured with no trusted proxies, so `X-Forwarded-For` cannot be spoofed to bypass the limit. If production runs more than one instance, or sits behind a load balancer, this needs a shared store and an explicit trusted-proxy list.

## Secrets

- `.env` is git-ignored; `.env.example` holds placeholders only.
- `JWT_SECRET` must be generated per environment, for example with `openssl rand -hex 32`.
- Demo accounts are created by `go run ./cmd/seed`, never by a migration. The seed refuses to run when `APP_ENV=production`.

## Audit

Audit events go to `audit_logs` with actor, action, target, safe context, IP address and time. Passwords and tokens are never written to audit context or to logs.
