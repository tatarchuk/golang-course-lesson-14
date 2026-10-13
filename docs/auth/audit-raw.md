<!-- Verbatim output of the AI security review (Task 3.1), produced by an independent agent with a fresh context and read-only access, prompt 3.1 from prompts.txt. Triage and decisions: ../../security_audit.md -->

# Security review: golang-course-lesson-9 (branch `lesson-9-auth`)

Repository root: `/Users/oleksandr.tatarchuk/repo/golang-course-lesson-9`. Read-only review; nothing was modified. Verified with `go build ./...` and `go vet ./...` (both clean), by reading the GORM v1.31.2 / pgx v5.10.0 sources in the module cache, and by one HMAC check of a committed token (no files created).

## Summary

The authentication and authorization design is sound in the places that usually go wrong: every query is parameterized (no `Raw`/`Exec`/string-built SQL anywhere), request bodies are decoded into narrow input DTOs and mapped to GORM models explicitly (no mass assignment; `Role` is hard-coded to `member` on register and `PUT /users/{id}/role` touches only the `role` column), every mutating route is wrapped in `Auth` + `RequireRole`, `PasswordHash` carries `json:"-"`, login is constant-time and uniform, the request logger never logs headers or bodies, and the DSN password cannot leak through connection errors. The genuine problems are at the edges of the design rather than in the handlers: (1) the GORM logger is configured without `ParameterizedQueries`, so the bcrypt hash of every registered password and every login e-mail are written to the server log (Info is the default in `docker-compose.yml` and in CI, and the Warn/Error paths inline parameters too, so `DB_LOG_LEVEL=warn` does not protect the bootstrap command either); (2) the role is trusted from a 24 h stateless token, so a demoted or deleted staff/admin keeps their privileges until expiry and no token can be revoked; (3) there is no rate limiting on `/auth/login` and `/auth/register` (online brute force plus a bcrypt CPU amplifier for unauthenticated callers); (4) the HS256 secret is only checked for being non-empty while the documented quickstart installs a secret that is public in the repository (and the local `.env` on this machine still uses it). Three Low findings follow: the register endpoint is an account-enumeration oracle that contradicts the careful login design, a currently valid admin JWT signed with the public secret is committed in `docs/auth/token-flow.txt`, and the compose file publishes PostgreSQL on all interfaces with the default password.

## Findings

| # | File:line | Severity | Category | Finding |
|---|-----------|----------|----------|---------|
| 1 | `internal/repository/db.go:33-38` (+ `:89-90`, `cmd/api/main.go:47`, `docker-compose.yml:31`, `cmd/bootstrap/main.go:32,49`) | Medium | 4 | GORM logger inlines bound parameters: bcrypt hashes (INSERT on register/bootstrap) and e-mails are written to the server log at Info; slow (Warn) and failed (Error) statements are logged with parameters at every level except `silent` |
| 2 | `internal/middleware/rbac.go:23` (+ `internal/auth/token.go:12,47-60`, `internal/handler/user_handler.go:64`) | Medium | 3 | Role is read from a 24 h stateless JWT: demotion/removal is not effective until expiry; no revocation or logout; a deleted account still passes `RequireRole` |
| 3 | `internal/app/app.go:49-50` (+ `internal/handler/auth_handler.go:41-61,77-105`, no limiter in `internal/middleware`) | Medium | other | No rate limiting on `/auth/login` and `/auth/register`: unlimited online password guessing, credential stuffing, bcrypt CPU amplification from anonymous clients, unbounded account creation |
| 4 | `cmd/api/main.go:40-43` (+ `internal/app/app.go:27-29`, `.env.example:9`, `README.md:34`, `internal/auth/token.go:32-43`) | Medium | 3 / other | JWT secret is only checked for non-emptiness; the quickstart installs a secret that is public in the repo; tokens carry no `iss`/`aud`, so any environment sharing a secret cross-validates |
| 5 | `internal/handler/auth_handler.go:52-55` | Low | other | `POST /auth/register` answers "email already registered" (422) vs 201: an unauthenticated, unlimited account-enumeration oracle, undoing the login endpoint's enumeration defence |
| 6 | `docs/auth/token-flow.txt:28,33,38` (also `:10,15,43,50`) | Low | 4 | Full, unredacted JWTs committed; the admin token (`user_id 1, role admin`) is valid until 2026-10-14 08:19:23 UTC and is signed with the public `.env.example` secret (verified) |
| 7 | `docker-compose.yml:9,12` | Low | other | PostgreSQL published on `0.0.0.0:5432` with default password `albums`; anyone on the host's network can rewrite `users.password_hash` |

## Details

### 1. GORM logger writes password hashes and e-mails to the log (Medium, category 4)

Files: `internal/repository/db.go`, `cmd/api/main.go`, `docker-compose.yml`, `cmd/bootstrap/main.go`.

Evidence:
- `db.go:33-38` builds the logger with `logger.New(log.New(os.Stdout, ...), logger.Config{SlowThreshold: 200ms, LogLevel: parseLogLevel(cfg.LogLevel), IgnoreRecordNotFoundError: true, Colorful: false})`. `ParameterizedQueries` is not set, so it is `false`.
- `db.go:89-90`: an empty/unknown `DB_LOG_LEVEL` means `logger.Info`. `cmd/api/main.go:47` passes `os.Getenv("DB_LOG_LEVEL")`; `docker-compose.yml:31` sets `DB_LOG_LEVEL: ${DB_LOG_LEVEL:-info}`; the CI job starts the API with no `DB_LOG_LEVEL` at all (`.github/workflows/theme8-check.yml:294`).
- GORM v1.31.2 (module cache): `callbacks.go:140-146` passes `stmt.Vars` to `Logger.Trace` unless `ParamsFilter` strips them; `logger/logger.go` `ParamsFilter` returns the params whenever `ParameterizedQueries` is false; `Trace` calls `fc()` (which runs `Dialector.Explain(sql, vars...)`) in all three branches: Error (`err != nil && LogLevel >= Error`), Warn (`elapsed > SlowThreshold`), Info; `logger/sql.go:117-118` inlines every string var verbatim between single quotes.
- `internal/repository/user_repository.go:36-42` (`Create(&user)`) therefore logs `INSERT INTO "users" ("email","role","password_hash",...) VALUES ('alice@...','member','$2a$10$...',...)` on every registration at Info. `user_repository.go:55` logs `... WHERE email = '<e-mail>'` on every login attempt (PII, including failed attempts). `user_repository.go:87-99` (`UpsertAdmin`) carries the admin hash.
- `cmd/bootstrap/main.go:32,49` states "The GORM logger stays at warn so the hash is never printed". That is not guaranteed: the same INSERT is printed with its parameters if it takes more than 200 ms (first statement against a cold container) or fails (CHECK/UNIQUE error). Likewise, in the API a duplicate-e-mail registration (`uq_users_email` violation, the "email already registered" path) logs the INSERT including the freshly computed hash even at `DB_LOG_LEVEL=error`.
- The repository's own CI check `.github/workflows/theme8-check.yml:381-382` (`no_hash_in_log() { ! grep -q '\$2a\$' app.log; }`) runs after `POST /auth/register` (`:353`) against an API started at Info level (`:294`). By code reading this check must fail; the branch has no CI run yet (only `main` runs are listed by `gh run list`), so this is not observed, only derived.

Attacker impact: anyone who can read the log (`docker compose logs`, which `scripts/smoke.sh:15` consumes; CI job output; any log shipper) obtains bcrypt hashes for offline cracking and the complete list of account e-mails and login attempts. Weak user passwords (the policy is 8 bytes, no complexity) are crackable from a cost-10 hash.

Proposed fix (a few lines in `db.go:33-38`):
```go
gormLogger := logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
    SlowThreshold:             200 * time.Millisecond,
    LogLevel:                  parseLogLevel(cfg.LogLevel),
    IgnoreRecordNotFoundError: true,
    ParameterizedQueries:      true, // log $1, $2 ... never the bound values
    Colorful:                  false,
})
```
The statements are still logged, so the SELECT counting in `scripts/smoke.sh:166` and the N+1 evidence keep working. Optionally default `DB_LOG_LEVEL` to `warn` in `docker-compose.yml:31` and use `silent` in `cmd/bootstrap/main.go:49`.

### 2. Role trusted from a 24 h stateless token; no revocation (Medium, category 3)

Files: `internal/middleware/rbac.go`, `internal/auth/token.go`, `internal/handler/user_handler.go`, `internal/handler/auth_handler.go`.

Evidence:
- `rbac.go:18-26`: `claims, ok := auth.UserFromContext(...)`; `if !slices.Contains(allowed, claims.Role)` - the decision uses only the role embedded in the token.
- `token.go:12` `TokenTTL = 24 * time.Hour`; `token.go:47-60` validates signature and `exp` only; no `jti`, no server-side state, no issued-before check.
- `user_handler.go:64` / `user_repository.go:69-85` change the DB row only. The only handler that re-reads the account is `auth_handler.go:116-132` (`/me`, which correctly 401s for a deleted account), but `RequireRole` never does.
- `docs/auth/token-flow.txt:48` ("a fresh login carries the new role") shows the promotion direction was considered; the demotion direction is the security-relevant one.

Attacker impact: an admin who demotes a compromised or departing `staff`/`admin` account (or removes it from the DB) does not actually remove its access: the old token keeps writing to the catalogue and, for an ex-admin, keeps listing users and changing roles for up to 24 h. A stolen token cannot be invalidated short of rotating `JWT_SECRET`, which logs everyone out.

Proposed fix: make `RequireRole` (or `Auth`) authoritative by loading the account once per request and using the stored role; the users table is tiny and the lookup is a primary-key hit:
```go
// in Auth, after ParseToken:
user, err := users.Get(r.Context(), claims.UserID)
if err != nil { WriteError(w, 401, CodeUnauthenticated, "invalid or expired token"); return }
claims.Role = user.Role
```
Alternatives/additions: a `token_version` (or `password_changed_at`) column compared against `iat`, a shorter TTL (e.g. 1 h) with refresh, and a `jti` denylist if explicit logout is required.

### 3. No rate limiting on login/register; bcrypt CPU amplification (Medium, category other)

Files: `internal/app/app.go`, `internal/handler/auth_handler.go`, `internal/middleware/*`.

Evidence:
- `app.go:49-50` registers `POST /api/v1/auth/register` and `POST /api/v1/auth/login` with no middleware other than Logging/Recover/CORS; `internal/middleware` contains no limiter and `go.mod` has no `golang.org/x/time` (grep: nothing).
- `auth_handler.go:95` runs one bcrypt compare (cost 10) per attempt, by design even for unknown e-mails (`:87-94`); `auth_handler.go:46` runs one bcrypt hash per registration. The password policy is length 8..72 only (`internal/model/user.go:53-58`).
- Registration validates the e-mail only as non-blank (`user.go:53-54`), the column is `TEXT` (`migrations/000001_init_schema.up.sql:73`), and the body limit is 1 MiB (`request.go:13`).

Attacker impact: unlimited online password guessing per account and credential stuffing across accounts; a cheap CPU-exhaustion vector (each anonymous request costs the server roughly 50-100 ms of bcrypt on one core); unlimited account creation with arbitrary up-to-1 MiB "e-mail" strings.

Proposed fix: a small token-bucket limiter keyed by client IP and by normalized e-mail in front of both routes (e.g. `golang.org/x/time/rate` in a map with expiry, or at the reverse proxy), progressive delay/lockout after N failures per account, and in `RegisterInput.Validate` a length cap (254) plus `net/mail.ParseAddress`. Wire it in `app.go` like the other groups: `mux.Handle("POST /api/v1/auth/login", limited(authn.Login))`.

### 4. JWT secret strength is not enforced; the quickstart installs a public secret (Medium, category 3/other)

Files: `cmd/api/main.go`, `internal/app/app.go`, `.env.example`, `README.md`, `internal/auth/token.go`.

Evidence:
- `cmd/api/main.go:40-43` and `app.go:27-29` only reject an empty `JWT_SECRET`.
- `.env.example:9` ships `JWT_SECRET=dev-only-change-me-9f1c7b2e4a6d8c0e`; `README.md:34` tells the user to `cp .env.example .env`; `docker-compose.yml:32` requires the variable but cannot tell the example from a real secret. The local `.env` on this machine still contains that example value (checked with a grep count; value not printed), and the tokens committed in `docs/auth/token-flow.txt` verify under it (see #6), proving the server ran with it.
- `token.go:32-43`: HS256 with `[]byte(secret)`, claims `user_id`, `role`, `exp`, `iat` only; `token.go:50-52` validates no `iss`/`aud`.

Attacker impact: with HS256, knowledge of the secret equals the ability to mint `{"user_id":1,"role":"admin"}` tokens, i.e. full administrative access without any password. A short or guessable secret is also brute-forceable offline from any captured token. Without `iss`/`aud`, a token minted for one deployment is valid on every deployment that shares the secret.

Proposed fix, in `run()` next to the existing check:
```go
if len(secret) < 32 || secret == "dev-only-change-me-9f1c7b2e4a6d8c0e" {
    return errors.New("JWT_SECRET must be at least 32 random bytes (openssl rand -base64 48) and not the .env.example value")
}
```
Add `Issuer`/`Audience` to `RegisteredClaims` and parse with `jwt.WithIssuer(...)`, `jwt.WithAudience(...)`; rotate the local secret. Severity note: this is Low if the deployment process guarantees a generated secret; the code itself has no guard, hence Medium.

### 5. `POST /auth/register` is an account-enumeration oracle (Low, category other)

File: `internal/handler/auth_handler.go`.

Evidence: `auth_handler.go:52-55` answers 422 `"email already registered"` for a known e-mail and 201 for a new one, whereas login (`:27-29`, `:63-66`, `:87-98`) was deliberately made uniform and constant-time. The CI workflow asserts this exact message (`.github/workflows/theme8-check.yml:358-359`), so it is intended behaviour.

Attacker impact: combined with #3, an anonymous client can test any list of e-mails for the existence of an account and then concentrate password guessing or phishing on them. The login enumeration defence is bypassed for free.

Proposed fix: either accept and document the trade-off and rate-limit registration (see #3), or respond uniformly (`202`, "check your e-mail") and complete registration through an e-mail verification link. If the 422 is kept, it should at least be per-IP rate-limited.

### 6. Valid admin JWT committed in `docs/auth/token-flow.txt` (Low, category 4)

File: `docs/auth/token-flow.txt`.

Evidence: lines 8 and 26 redact the tokens, but lines 10, 15, 43, 50 (member/staff tokens) and 28, 33, 38 (admin token) contain them in full. The admin token decodes to `{"user_id":1,"role":"admin","exp":1791965963,"iat":1791879563}` (`exp` = 2026-10-14 08:19:23 UTC; review time 2026-10-13 09:41 UTC, so still valid), and its HMAC-SHA256 under `dev-only-change-me-9f1c7b2e4a6d8c0e` equals the committed signature.

Attacker impact: until it expires, the token is an admin credential for any instance that runs with the example secret. After expiry it is harmless. Strictly dominated by #4, but it shows transcripts are committed unredacted and the secret has been used for real tokens.

Proposed fix: redact tokens in committed transcripts the way lines 8/26 already do; rotate the local secret; add a CI grep for `eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` under `docs/`.

### 7. PostgreSQL published on all interfaces with the default password (Low, category other)

File: `docker-compose.yml`.

Evidence: `docker-compose.yml:9` `POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-albums}`, `:12` `ports: - "${POSTGRES_PORT:-5432}:5432"` (no host IP, so Docker binds `0.0.0.0` and, on Linux, bypasses the usual host firewall rules); `.env.example:19-22` repeats the defaults; `.env.example:4` uses `sslmode=disable`.

Attacker impact: anyone on the same network as the developer's machine (or on the Internet if the compose file is reused on a VM) can connect as `albums/albums` and, for example, set `users.password_hash` of the admin to a hash they know, which is full application takeover independent of every other control.

Proposed fix: `- "127.0.0.1:${POSTGRES_PORT:-5432}:5432"`; keep the default password and `sslmode=disable` only inside the compose network.

## Informational (not counted as findings)

- `scripts/smoke.sh:34-35` and `docs/CURL.md:39` put the bearer token and passwords on the `curl` command line (visible in `ps` on shared hosts). `cmd/bootstrap/main.go:24,34-36` already avoids this for `ADMIN_PASSWORD`. Acceptable for a local smoke script; `curl -H @file` or `--config` would avoid it.
- `internal/handler/respond.go:10-16`: no `X-Content-Type-Options: nosniff` and no `Cache-Control: no-store` on the login response. `encoding/json` HTML-escapes output, so JSON reflection is not an XSS vector; adding both headers in `WriteJSON` is cheap hardening.
- `internal/handler/request.go:18-24`: `json.Unmarshal` ignores unknown fields, so a `"role":"admin"` posted to `/auth/register` is silently dropped (safe). `DisallowUnknownFields` would make such attempts visible as 422s.
- 401 responses carry no `WWW-Authenticate: Bearer` header (RFC 6750); contract, not security.
- `internal/handler/user_handler.go:55-70`: an admin can demote themselves or the last admin (lockout, not a vulnerability).

## Checked and found correct

Category 1, SQL injection:
- `internal/repository/user_repository.go:55` `First(&user, "email = ?", email)`; `:46`, `:72`, `:79` `First(&user, id)` with an `int64` (GORM's primary-key path; ids come only from `parseID`, `internal/handler/request.go:37-43`, which `ParseUint`s them, so no string ever reaches an inline condition); `:63` `Order("id")` constant; `:75` `Update("role", role)` constant column, bound value, validated by `internal/model/user.go:82-88` and `chk_users_role`; `:89-97` `OnConflict` with constant columns and `gorm.Expr("now()")` without user input.
- `internal/repository/album_repository.go:87` `Where("genre_id = ?")`, `:89` integer `Offset`/`Limit`, `:120` `Where("album_id = ?")`, `:134` `Delete(&model.Album{}, id)`, `:158` `Where("id = ?")`, `:166` `Where("id IN ?", []int64)`, `:201` `Preload` with constant names.
- `internal/repository/copy_repository.go:44`, `:57-59` (`Select` with a `?` placeholder for the status), `internal/repository/repository.go:29`.
- grep of `internal/repository` and `internal/handler` for `.Raw(`, `.Exec(`, `fmt.Sprintf`, string concatenation: none in SQL; the only concatenations are the `pgx5://` scheme rewrite (`migrate.go:47`) and error messages.

Category 2, mass assignment:
- `internal/model/user.go:47-50` `RegisterInput` (email, password), `:62-65` `LoginInput`, `:77-79` `RoleInput` - none can carry `Role`/`PasswordHash`/`ID`; `user_repository.go:37` hard-codes `Role: member`; `user_repository.go:75` updates only `role`.
- `internal/model/album.go:40-47` `AlbumInput` mapped explicitly in `album_repository.go:144-152`; `:54` `Omit(clause.Associations)` and `:114-116` `Select("title","label","genre_id","release_year","notes")` prevent GORM from upserting artists or touching id/timestamps; `internal/model/copy.go:33-36` mapped at `copy_repository.go:31`; genre/artist inputs mapped at `genre_repository.go:25`, `artist_repository.go:25`. No handler decodes a request into a GORM model.

Category 3, authentication and RBAC:
- `internal/app/app.go:48-58` public routes are reads, health, register, login; `:61` `/me` under `Auth`; `:64-69` every catalogue write under `staff` (= Auth + RequireRole(staff, admin)); `:72-73` user management under `admin`; `:37-43` `RequireRole` is always nested inside `Auth`; `internal/middleware/rbac.go:18-22` answers 401 when claims are missing (defence in depth).
- `internal/middleware/auth.go:17-27`: scheme check, generic 401 messages, token never echoed or logged. `internal/auth/token.go:50-52`: `WithValidMethods([HS256])` (rejects `alg: none`/RS256 confusion) and `WithExpirationRequired`; `:56` rejects `user_id <= 0` and empty role. `internal/auth/context.go:6` unexported context key.
- `UpsertAdmin` is only called from `cmd/bootstrap/main.go:65` (grep); it is not reachable over HTTP.
- CORS `internal/middleware/cors.go:9-11`: `Access-Control-Allow-Origin: *` is safe here because credentials are a bearer header and `Allow-Credentials` is not set, so a foreign origin cannot make the victim's browser attach a token; answering preflights before `Auth` is intended. (Must be revisited if cookie auth is ever introduced.)

Category 4, sensitive data exposure:
- `internal/model/user.go:28` `json:"-"` on `PasswordHash` (covered by `internal/model/user_test.go:84`, `scripts/smoke.sh:71`, CI `:355-356`); `internal/handler/auth_handler.go:104` returns only `{"token"}`; `internal/handler/user_handler.go:35` returns `model.User` values (hash excluded by the tag).
- `internal/middleware/logging.go:26` logs method, path, status, duration only (no query string, no headers, no body); `internal/middleware/recover.go:17` stack trace to the server log only, generic 500 to the client; `internal/handler/errors.go:56-58` generic 500 for unknown errors; `internal/handler/request.go:27-33` hides Go type names.
- `internal/repository/db.go:55` logs the open error only; pgx v5.10.0 `pgconn/errors.go:136-145` (`ParseConfigError` uses `redactPW`) and `:68-76` (`ConnectError` prints only user and database), so the DSN password does not reach the log.
- `.env` is git-ignored (`.gitignore:16`) and untracked (`git ls-files` / `git check-ignore` verified); `.dockerignore:9` keeps it out of the image; `Dockerfile:18,22` runs as the non-root `app` user; `cmd/bootstrap/main.go:34-36` reads the password from `ADMIN_PASSWORD` and `:69` prints only e-mail and id; CI uses its own `JWT_SECRET` (`theme8-check.yml:29,180`). No bcrypt hash is committed anywhere (grep for `$2a$` hits only tests and the `DummyHash` constant).
- Login `internal/handler/auth_handler.go:82-98`: bcrypt runs on every attempt (`DummyHash` for unknown accounts and for seed accounts whose hash is `''`, `migrations/000003_add_auth_fields.up.sql:4`), evaluation order `!CheckPassword(...) || !known` guarantees the compare always executes, and one 401 body is used for every failure.
- Passwords: `bcrypt.DefaultCost` (10) `internal/auth/password.go:14`; byte-length limits 8..72 (`internal/model/user.go:16-20,53-58`) consistent with bcrypt's 72-byte input; login caps at 72 (`user.go:68-74`) so an oversized password is a 422, not a 500.
- Hardening present: 1 MiB body cap (`request.go:13,19`), `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout` (`cmd/api/main.go:64-67`), graceful shutdown. Dependencies: `golang-jwt/jwt/v5 v5.3.1` (newer than v5.2.2, which fixed the header-parsing allocation issue), `golang.org/x/crypto v0.53.0`, `gorm v1.31.2`, `pgx v5.10.0`; `go build ./...` and `go vet ./...` pass.

Not covered: the Swagger UI dependency (`swaggo/http-swagger v2.0.2`) was not audited for known issues; it is served publicly at `/swagger/` (`app.go:75`), which is acceptable for an API whose spec is not secret.
