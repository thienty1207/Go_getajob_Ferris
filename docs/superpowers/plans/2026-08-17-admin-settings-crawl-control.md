# Admin Settings and Crawl Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task with verification checkpoints.

**Goal:** Add a real admin Settings page for an hours-and-minutes crawler interval and a database-backed Crawl Now request queue consumed by the Rust daemon.

**Architecture:** PostgreSQL owns typed operational settings and manual crawl requests. Go exposes authenticated, CSRF-protected admin contracts and bounded status metadata. Rust consumes pending requests, runs the existing scoped crawler, and reloads the interval at runtime. Svelte renders the settings form and Job Link action states without mock data.

**Tech Stack:** PostgreSQL migrations, Go/Gin/pgx, Rust/tokio/tokio-postgres, Svelte 5, TypeScript, Vitest.

## Global Constraints

- Crawl only ACTIVE approved Job Links within their persisted host/path scope.
- Never persist raw CV, full JD, raw HTML, or provider payloads.
- Source/parser/anomaly failures never count as missing jobs.
- Preserve the two healthy missing-cycle close rule.
- Store source currency unchanged and distances in kilometers.
- Settings are admin-only; `.env` remains for secrets and connection aliases.
- Use practical comments only for non-obvious invariants; do not add UI commentary copy.

### Task 1: Add the database contract tests and migrations

**Files:**
- Create: `database/migrations/000008_settings_and_crawl_requests.up.sql`
- Create: `database/migrations/000008_settings_and_crawl_requests.down.sql`
- Modify: `database/tests/validate-schema.ps1`
- Modify: `database/README.md`
- Test: `database/tests/validate-schema.ps1`

**Interfaces:**
- Produces `public.app_settings(setting_key, setting_group, setting_value, updated_at, updated_by)`.
- Produces `public.crawl_requests(id, source_id, status, requested_by, requested_at, started_at, finished_at, source_run_id, error_code)`.
- Produces a partial unique index for one `PENDING` or `RUNNING` request per source.

- [ ] **Step 1: Extend the schema validator with failing assertions.**

Add required table/column/index/status checks for `app_settings` and
`crawl_requests`, including the partial unique index, source foreign key, and
the cascade behavior for queued requests.

- [ ] **Step 2: Run the schema contract and verify the expected failure.**

Run:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\database\tests\validate-schema.ps1
```

Expected: FAIL because migration `000008` and its assertions do not exist yet.

- [ ] **Step 3: Add the transactional up/down migrations.**

Use `jsonb` for the settings value, bounded text keys/groups, the four explicit
request statuses, foreign keys, timestamps, and a partial unique index. Do not
insert runtime rows in the migration.

- [ ] **Step 4: Run the schema contract and verify it passes.**

Run the same PowerShell validator and confirm exit code 0.

- [ ] **Step 5: Update migration documentation.**

Add `000008` to apply and rollback order and document settings ownership,
request lifecycle, and the fact that crawler settings are saved in PostgreSQL.

### Task 2: Implement typed Go settings and crawl request repositories

**Files:**
- Create: `backend/internal/model/settings.go`
- Create: `backend/internal/model/crawl_request.go`
- Create: `backend/internal/repository/settings.go`
- Create: `backend/internal/repository/postgres_settings.go`
- Create: `backend/internal/repository/crawl_requests.go`
- Create: `backend/internal/repository/postgres_crawl_requests.go`
- Test: `backend/internal/repository/postgres_settings_integration_test.go`
- Test: `backend/internal/repository/postgres_crawl_requests_integration_test.go`

**Interfaces:**
- `SettingsRepository.GetCrawlerInterval(ctx) (model.CrawlerInterval, error)`.
- `SettingsRepository.SaveCrawlerInterval(ctx, model.CrawlerInterval, actor string) (model.CrawlerInterval, error)`.
- `CrawlRequestRepository.Enqueue(ctx, sourceID uuid.UUID, actor string) (model.CrawlRequest, error)`.
- `CrawlRequestRepository.ListJobLinkRuntime(ctx, sourceID uuid.UUID) (model.JobLinkRuntime, error)`.
- Rust will consume the same table directly through SQL; it will not depend on Go packages.

- [ ] **Step 1: Write failing repository contract tests.**

The settings test must assert default `21600` seconds when no row exists and
round-trip persistence. The request test must assert the second enqueue for a
source returns the existing pending request, claiming changes it to running,
and deleting the source removes its request.

- [ ] **Step 2: Run the focused tests to verify they fail for missing types/tables.**

Run:

```powershell
go test ./internal/repository -run 'TestPostgres(Settings|CrawlRequest)' -count=1
```

Expected: the focused integration tests skip without `DATABASE_URL` or fail on
the missing repository contract before implementation.

- [ ] **Step 3: Add model types and repository interfaces.**

Represent interval seconds as an integer and expose hours/minutes only at the
HTTP boundary. Represent request status and linked run metadata without raw
source content.

- [ ] **Step 4: Implement settings SQL and request queue SQL.**

Use a single transaction for enqueue dedupe and `FOR UPDATE SKIP LOCKED` for
Rust-compatible claim semantics. Return stable not-found/conflict errors.

- [ ] **Step 5: Run the focused integration tests with local PostgreSQL.**

Run with the local database URL supplied through the current shell environment:

```powershell
go test -tags=integration ./internal/repository -run 'TestPostgres(Settings|CrawlRequest)' -count=1
```

Expected: PASS after applying migration `000008`.

### Task 3: Add Go settings validation and authenticated HTTP contracts

**Files:**
- Create: `backend/internal/service/settings.go`
- Create: `backend/internal/service/crawl_request.go`
- Create: `backend/internal/httpapi/admin_settings_handler.go`
- Modify: `backend/internal/httpapi/admin_job_link_handler.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/cmd/api/main.go`
- Test: `backend/internal/service/settings_test.go`
- Test: `backend/internal/httpapi/admin_settings_handler_test.go`
- Test: `backend/internal/httpapi/admin_job_link_handler_test.go`

**Interfaces:**
- `PATCH /api/v1/admin/settings/crawler` accepts `{interval_hours, interval_minutes}` and returns the canonical saved interval.
- `GET /api/v1/admin/settings` returns the crawler interval and min/max bounds.
- `POST /api/v1/admin/job-links/:id/crawl` returns `202` and a deduplicated request.

- [ ] **Step 1: Write failing service tests for hours/minutes validation.**

Cover `6h 0m -> 21600`, `2h 30m -> 9000`, reject negative values, reject minutes
outside `0..59`, reject totals below 15 minutes and above 7 days, and preserve
the default when no database row exists.

- [ ] **Step 2: Run the service tests and verify the expected failure.**

Run:

```powershell
go test ./internal/service -run 'Test(CrawlerInterval|CrawlRequest)' -count=1
```

Expected: FAIL because the new service contracts do not exist.

- [ ] **Step 3: Write failing HTTP tests for auth, CSRF, response shape, and dedupe.**

Assert unauthenticated requests are rejected, settings writes without CSRF are
rejected, a valid settings write returns canonical hours/minutes, and the Crawl
Now route returns `202` without exposing raw content.

- [ ] **Step 4: Run the HTTP tests and verify the expected failure.**

Run:

```powershell
go test ./internal/httpapi -run 'TestAdmin(Settings|JobLinkCrawl)' -count=1
```

Expected: FAIL because the handlers and routes are not wired.

- [ ] **Step 5: Implement services and handlers.**

Keep validation server-side, derive the admin actor from the authenticated
session, use the existing CSRF middleware, and map errors to bounded codes.

- [ ] **Step 6: Wire repositories/services into `cmd/api` and the authenticated router.**

Keep existing routes compatible. Add the Crawl Now route beside the existing
Job Link CRUD routes and the Settings routes under the protected admin group.

- [ ] **Step 7: Run focused Go tests and verify they pass.**

Run the service and HTTP commands above, then `go test ./... -count=1`.

### Task 4: Extend Job Link runtime metadata and implement Rust request consumption

**Files:**
- Modify: `backend/internal/model/job_link.go`
- Modify: `backend/internal/repository/postgres_job_links.go`
- Modify: `backend/internal/httpapi/admin_job_link_handler.go`
- Modify: `crawler/src/store.rs`
- Modify: `crawler/src/main.rs`
- Modify: `crawler/src/config.rs`
- Modify: `crawler/src/lib.rs` if the new store module needs exports
- Test: `crawler/src/config.rs`
- Test: `crawler/tests/scope_and_reconcile.rs`
- Test: `crawler/tests/postgres_contract.rs`

**Interfaces:**
- Rust claims one pending request atomically and updates its status after the linked crawl run finalizes.
- `JobLink` list responses expose request status and latest run counters.
- `cargo run` executes an immediate full cycle, polls manual requests every five seconds, and reloads the database interval for periodic scheduling.

- [ ] **Step 1: Add failing pure scheduler/config tests.**

Test the six-hour default, a database value overriding the environment fallback,
and one-shot mode handling a pending request before exiting.

- [ ] **Step 2: Run the focused Rust tests and verify the expected failure.**

Run:

```powershell
cargo test scheduler --lib
```

Expected: FAIL because database-backed scheduling and request processing are not implemented.

- [ ] **Step 3: Add Rust SQL helpers for settings and request claim/finalization.**

Claim with `FOR UPDATE SKIP LOCKED`, join only ACTIVE sources, mark ineligible
requests failed with bounded `source_inactive`, and never bypass existing source
scope or structured-observation validation.

- [ ] **Step 4: Extract one-source execution from the existing periodic loop.**

Reuse the current crawl, run-finalization, transactional persistence, recovery,
and missing-reconciliation path for both periodic and manual requests. Avoid a
second crawler implementation.

- [ ] **Step 5: Add the daemon scheduler.**

Poll requests on a short internal interval, run an immediate full cycle at
startup, reload `crawler.interval_seconds` after cycles, and keep `--once`
deterministic for local testing.

- [ ] **Step 6: Run Rust formatting, tests, and check.**

```powershell
cargo fmt --all -- --check
cargo test
cargo check
```

### Task 5: Add the Admin Settings page and Job Link Crawl Now UI

**Files:**
- Create: `frontend/src/routes/admin/settings/+page.svelte`
- Modify: `frontend/src/lib/admin/api/admin-api.ts`
- Modify: `frontend/src/lib/admin/components/AdminShell.svelte`
- Modify: `frontend/src/routes/admin/sources/+page.svelte`
- Modify: `frontend/src/lib/admin/styles/admin.css`
- Test: `frontend/src/lib/admin/api/admin-api.spec.ts`
- Test: `frontend/src/routes/admin/settings/settings-page.spec.ts` if the route-test convention supports it

**Interfaces:**
- `getAdminSettings`, `updateAdminCrawlerSettings`, and `requestAdminJobLinkCrawl` parse only the new bounded JSON shapes.
- Settings form sends hours/minutes and shows loading, disabled, error, and saved states.
- Job Link rows show a disabled Crawl Now button while pending/running and refresh bounded list data until the request resolves.

- [ ] **Step 1: Write failing TypeScript API contract tests.**

Assert credentials/CSRF, PATCH body `{interval_hours, interval_minutes}`,
settings parsing, `202` Crawl Now parsing, and rejection of malformed response
data.

- [ ] **Step 2: Run the focused frontend tests and verify the expected failure.**

```powershell
bun run test -- admin-api.spec.ts
```

Expected: FAIL because the functions and types do not exist.

- [ ] **Step 3: Implement the typed API functions.**

Keep all parsing bounded and use the existing `adminRequest` credentials/CSRF
path. Do not add localStorage or mock fallback data.

- [ ] **Step 4: Add Settings navigation and the compact responsive form.**

Use existing admin tokens/styles, numeric hours/minutes inputs, a single save
button, visible focus, and no decorative explanatory copy.

- [ ] **Step 5: Add Crawl Now and runtime status to Job Link rows.**

Use a local request state only while the API request is active; source truth
comes from the API list. Preserve the URL and show retryable error state.

- [ ] **Step 6: Run frontend verification.**

```powershell
bun run check
bun run test
bun run build
```

### Task 6: Apply migration, update docs, and run end-to-end smoke

**Files:**
- Modify: `backend/README.md`
- Modify: `crawler/README.md`
- Modify: `database/README.md`
- Modify: `frontend/README.md` if the run instructions live there

- [ ] **Step 1: Apply migration `000008` to the local PostgreSQL database after checking the current schema version.**
- [ ] **Step 2: Start backend, crawler, and frontend in separate terminals.**
- [ ] **Step 3: Authenticate in Admin, save an hours/minutes interval, and verify `GET /settings` returns it.**
- [ ] **Step 4: Click Crawl Now for the existing ACTIVE source and verify request state transitions or an explicit source anomaly.**
- [ ] **Step 5: Run schema, Go, Rust, and frontend gates again.**
- [ ] **Step 6: Run Baron trace scoring and record any proof-runner limitation without claiming trusted proof that did not execute.**
