# Go Backend API Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first real Go backend for the client CV scan flow with PostgreSQL persistence, strict upload handling, and the existing frontend API contract without mock results.

**Architecture:** `backend/cmd/api` wires configuration, a pgx pool, a PostgreSQL scan repository, a scan service, an explicit unavailable processor, and Gin HTTP routes. The service/repository/processor boundaries keep DeepSeek, crawler, geocoding, deterministic matching, and future authentication replaceable without inventing provider behavior in the initial runtime.

**Tech Stack:** Go 1.25+, Gin, pgx/v5, google/uuid, self-hosted PostgreSQL, SvelteKit client contract in `frontend/src/lib/client/api/client-api.ts`.

## Global Constraints

- Do not add seed data, sample jobs, sample CVs, fallback profiles, or fake matches.
- Raw CV files are temporary and deleted after processing; only structured profiles may be persisted by a future processor.
- Full job descriptions and source payloads are not persisted; public job reads use `active_job_cache`.
- Distances use kilometers and source salary currency is preserved without conversion.
- Required CV Match weights remain 35% required skills, 25% role relevance, 15% experience, 15% seniority, and 10% preferred skills/domain; this slice does not compute them yet.
- Use parameterized PostgreSQL queries and generic public errors; do not expose SQL, DSNs, file paths, parser output, or stack traces.
- Bind to localhost by default, use explicit CORS origins, cap uploads, rate-limit both intake and scan-status reads, and reject non-loopback binds until scan ownership/authentication exists.
- Scan ownership/authentication is an unresolved contract and must be disclosed as a pre-production limitation.
- Because the repository has no Git metadata, use Baron checkpoints and test receipts instead of inventing commit evidence.

---

### Task 1: Backend module and configuration contract

**Files:**
- Create: `backend/go.mod`
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

**Interfaces:**
- Produces `config.Config` with `Address`, `DatabaseURL`, `Environment`, `MaxCVBytes`, `MaxRadiusKm`, `AllowedOrigins`, and `RateLimitPerMinute`.
- `config.Load()` reads `API_ADDR`, `DATABASE_URL`, `APP_ENV`, `MAX_CV_BYTES`, `MAX_RADIUS_KM`, `CORS_ALLOWED_ORIGINS`, and `RATE_LIMIT_PER_MINUTE`, with safe local defaults except for `DATABASE_URL`.

- [x] **Step 1: Write failing configuration tests**

Test that missing `DATABASE_URL` returns a configuration error; local defaults bind to `127.0.0.1:8080`, cap CV files at 10 MiB, cap radius at 500 km, and use explicit localhost origins; test that configured numeric values and comma-separated origins are parsed and trimmed.

- [x] **Step 2: Run the focused test and verify it fails**

Run `Set-Location backend; go test ./internal/config -run TestLoad -count=1`.
Expected result: compilation or test failure because the module and configuration package do not yet exist.

- [x] **Step 3: Add the module and minimal configuration implementation**

Declare Gin, pgx/v5, and google/uuid dependencies in `backend/go.mod`. Implement typed environment parsing, positive-number validation, a required non-empty `DATABASE_URL`, origin parsing, and errors that identify only the invalid setting name.

- [x] **Step 4: Run the focused test and verify it passes**

Run `Set-Location backend; gofmt -w internal/config; go test ./internal/config -run TestLoad -count=1`.
Expected result: PASS.

### Task 2: Domain models and scan repository contract

**Files:**
- Create: `backend/internal/model/scan.go`
- Create: `backend/internal/repository/scans.go`
- Create: `backend/internal/repository/postgres_scans.go`
- Test: `backend/internal/repository/postgres_scans_test.go`

**Interfaces:**
- `repository.ScanRepository` exposes `CreateScan(context.Context, string, float64) (uuid.UUID, error)`, `SetStatus(context.Context, uuid.UUID, model.ScanStatus, *string) error`, and `GetScan(context.Context, uuid.UUID) (model.Scan, error)`.
- `repository.PostgresScanRepository` uses `*pgxpool.Pool`, only parameterized SQL, transactional lifecycle entry with bounded commit recovery, conditional/idempotent transitions, and a repeatable-read snapshot for completed scan reads.
- `model.Scan` contains the scan UUID, lifecycle status, and public `[]model.JobMatch`; it does not contain raw CV bytes/path or full JD text.

- [x] **Step 1: Write failing repository contract tests**

Use a small repository test double only inside the test package to prove the interface can create a scan, update status, retrieve a failed scan, and return a completed scan with at most the fields required by the client response. Add a source audit test that rejects raw-CV/full-JD field names in the model package.

- [x] **Step 2: Run the focused test and verify it fails**

Run `Set-Location backend; go test ./internal/repository -count=1`.
Expected result: failure because model and repository types are absent.

- [x] **Step 3: Implement models and PostgreSQL queries**

Define uppercase database statuses `RECEIVED`, `PARSING`, `MATCHING`, `COMPLETED`, and `FAILED`, plus the public match fields. Implement a transaction containing `RECEIVED` insert and guarded `PARSING` update, verify a locally known ID for ambiguous commit outcomes, use guarded/idempotent legal transitions, and use a repeatable-read `GetScan` snapshot: first read the scan state, then for `COMPLETED` read `scan_matches JOIN active_job_cache`, ordered by score and distance with `LIMIT 100`. Cast PostgreSQL numeric score/distance values to `double precision` in SQL for stable Go scanning. Treat no row as a typed not-found error.

- [x] **Step 4: Run tests and static SQL checks**

Run `Set-Location backend; gofmt -w internal/model internal/repository; go test ./internal/repository -count=1` and `rg -n "fmt\.Sprintf|\+.*SELECT|job_cache[^_]" internal/repository`.
Expected result: repository tests pass and the SQL audit finds no string-built query or direct public read from base `job_cache`.

### Task 3: Temporary CV intake and processor boundary

**Files:**
- Create: `backend/internal/processor/processor.go`
- Create: `backend/internal/processor/unavailable.go`
- Create: `backend/internal/service/scan_service.go`
- Create: `backend/internal/service/upload.go`
- Test: `backend/internal/service/scan_service_test.go`
- Test: `backend/internal/service/upload_test.go`

**Interfaces:**
- `processor.ScanProcessor` is `Process(context.Context, uuid.UUID, string, string, float64) error`.
- `service.ScanService` exposes `Start(context.Context, service.ScanInput) (uuid.UUID, error)` and `Get(context.Context, uuid.UUID) (model.Scan, error)`.
- `service.ScanInput` contains `*multipart.FileHeader`, location text, and radius in kilometers.

- [x] **Step 1: Write failing upload and lifecycle tests**

Cover: `.pdf`, `.docx`, and `.txt` are the only accepted extensions; a PDF with the wrong signature is rejected; a file larger than the configured maximum is rejected after actual copy rather than trusting `FileHeader.Size`; a valid upload creates a scan and passes a temporary path to a processor; that path is absent after `Start` returns; processor errors persist `FAILED` with a code; and an unavailable processor never creates a completed match.

- [x] **Step 2: Run the focused tests and verify they fail**

Run `Set-Location backend; go test ./internal/service -count=1`.
Expected result: failure because the service and processor boundaries do not exist.

- [x] **Step 3: Implement bounded temporary-file handling**

Open the multipart file, create a random OS temporary file, copy at most `MaxCVBytes+1` bytes, reject overflow, seek to the beginning, and validate PDF `%PDF-` or DOCX `PK` signatures. Never use the client filename as a filesystem path. Defer removal for every exit path, including repository and processor errors. Trim and bound location text and enforce `0 < radius_km <= MaxRadiusKm`.

- [x] **Step 4: Implement lifecycle orchestration and explicit unavailable processing**

Atomically create the scan in `PARSING`, invoke the injected processor, and on processor error retry the `FAILED` transition once using a stable error code. The default processor returns `parser_not_configured`; it does not return a profile or match. A future successful processor must own profile/match persistence and final `COMPLETED` transition before returning nil.

- [x] **Step 5: Run focused tests and audits**

Run `Set-Location backend; gofmt -w internal/processor internal/service; go test ./internal/service -count=1` and `rg -n "os\.CreateTemp|Remove|raw|description|mock|seed|sample" internal/service internal/processor`.
Expected result: tests pass; temporary cleanup is visible; no raw CV or fake result path exists in production code.

### Task 4: HTTP handlers, contract mapping, CORS, and rate limiting

**Files:**
- Create: `backend/internal/httpapi/errors.go`
- Create: `backend/internal/httpapi/handler.go`
- Create: `backend/internal/httpapi/middleware.go`
- Create: `backend/internal/httpapi/router.go`
- Test: `backend/internal/httpapi/handler_test.go`
- Test: `backend/internal/httpapi/middleware_test.go`

**Interfaces:**
- `httpapi.NewRouter(config.Config, *httpapi.Handler) *gin.Engine` creates the public routes.
- `httpapi.Handler` depends on `service.ScanService` and a database health checker.

- [x] **Step 1: Write failing API contract tests**

Test `POST /api/v1/client/scans` returns `202` with snake_case `scan_id` and `processing` for a valid multipart request; invalid extension, missing fields, malformed radius, and oversize body return stable non-2xx `{code,message}` without internal details; `GET` returns processing, failed, completed, and 404 shapes; completed matches expose no more than three skill tags; invalid UUIDs never reach the repository; and generic repository failures do not leak their error text.

- [x] **Step 2: Run focused API tests and verify they fail**

Run `Set-Location backend; go test ./internal/httpapi -count=1`.
Expected result: failure because routes and handlers are absent.

- [x] **Step 3: Implement stable handler responses**

Use `http.MaxBytesReader` before multipart parsing, parse only the named `cv`, `location`, and `radius_km` fields, and map domain errors to codes such as `invalid_upload`, `invalid_scan_request`, `scan_not_found`, `scan_unavailable`, and `internal_error`. Set `gin.ReleaseMode` in router construction. Map `RECEIVED`/`PARSING`/`MATCHING` to `processing`, preserve the frontend `completed` response field names, and never serialize repository errors.

- [x] **Step 4: Add explicit CORS and intake/read rate limiting**

Allow only configured origins, methods `GET, POST, OPTIONS`, and headers needed for multipart requests. Do not allow credentials. Add separate per-IP fixed-window limiters for POST intake and GET status reads, return `429` with a stable error shape, and cap active buckets at 10,000 while cleaning expired entries.

- [x] **Step 5: Run focused API and security tests**

Run `Set-Location backend; gofmt -w internal/httpapi; go test ./internal/httpapi -count=1` and `rg -n "err\.Error\(\)|gin\.DebugMode|Access-Control-Allow-Origin.*\*|c\.JSON\([^\n]*err" internal/httpapi`.
Expected result: tests pass and the audit finds no raw error response, debug mode, wildcard CORS, or direct error serialization.

### Task 5: Runtime wiring, database readiness, and operator documentation

**Files:**
- Create: `backend/internal/database/pool.go`
- Create: `backend/cmd/api/main.go`
- Create: `backend/README.md`
- Create: `frontend/.env.example`
- Modify: `database/README.md`
- Test: `backend/internal/database/pool_test.go`

**Interfaces:**
- `database.Open(context.Context, string) (*pgxpool.Pool, error)` opens and pings PostgreSQL.
- The API binary wires `repository.NewPostgresScanRepository`, `service.NewScanService`, and `processor.UnavailableProcessor`.

- [x] **Step 1: Write failing database readiness tests**

Test that the pool helper rejects a blank URL without opening a connection and that health mapping returns generic `503` behavior when a pinger fails. Keep the test independent of a live database.

- [x] **Step 2: Run the focused test and verify it fails**

Run `Set-Location backend; go test ./internal/database -count=1`.
Expected result: failure because the pool helper and readiness handler are absent.

- [x] **Step 3: Implement pool and main wiring**

Create a pgx pool from `DATABASE_URL`, ping during startup, close on shutdown, configure a release-mode Gin server, and handle interrupt/termination signals with a bounded shutdown context. Log internal startup/database errors only to the server process; return generic health errors to clients.

- [x] **Step 4: Document real local setup**

Document environment variables, applying `database/migrations/000001_initial_schema.up.sql` through pgAdmin or `psql`, running `go test ./...`, starting `go run ./cmd/api`, and the explicit parser/authentication limitation. State that no credentials belong in the repository and that scan ownership is required before public deployment.

- [x] **Step 5: Run the full verification suite**

Run from `backend/`: `gofmt -w .`, `go test ./... -count=1`, `go vet ./...`. From the repository root run `pwsh -NoProfile -File database/tests/validate-schema.ps1`, a no-mock/raw-data audit, and the frontend test/build suite. If PostgreSQL tooling is available, run the migration against an empty database and call `/healthz`; otherwise record the missing live-server proof explicitly.

### Task 6: Review gates and completion evidence

**Files:**
- Review: all changed `backend/` files and backend docs.
- Evidence: Baron proof receipt, gate records, continuity checkpoint, and autopilot review.

- [x] **Step 1: Run code, test, and security review gates**

Review public API compatibility, dependency direction, upload cleanup, SQL parameterization, CORS/rate limiting, verbose errors, and the unresolved scan-ownership boundary. Keep any concrete finding open until its fix and verification are present.

- [x] **Step 2: Re-run all verification after review fixes**

Run `gofmt -l .`, `go test ./... -count=1`, `go vet ./...`, the schema contract check, and the no-mock/raw-CV audit from their documented working directories.

- [x] **Step 3: Record Baron proof and trace**

Record the actual execution receipt with `baron proof record`, record mandatory gates with `baron control-plane record-gate`, run `baron trace score`, then run `baron autopilot review` with the proof state and remaining auth/parser risks before the final handoff.
