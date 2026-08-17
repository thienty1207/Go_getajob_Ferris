# Admin Management And Crawler Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task with verification after each task.

**Goal:** Make Job Link, Job Location, Job Cache, sidebar navigation, and crawler Settings truthful and operational with server-side pagination, inline canonical-location management, search, and persisted crawler runtime evidence.

**Architecture:** Keep the existing Go route families and PostgreSQL repositories. Add bounded page/query contracts, a singleton crawler-runtime row, and an admin location-options endpoint for Job Cache selects. Keep `job_cache.location_id` as the single location write boundary. The Rust daemon persists heartbeat and cycle timestamps; Go exposes the runtime state and marks stale heartbeats offline.

**Tech Stack:** Go/Gin/pgx, PostgreSQL migrations, Rust/Tokio/tokio-postgres, Svelte 5, TypeScript, Vitest/Bun.

## Global Constraints

- First market is Vietnam; canonical locations and kilometres remain the product contract.
- `job_cache.location_id` is the canonical location used by admin correction and future client filtering/matching.
- Admin Job Link, Job Location, and Job Cache pages use a server-enforced maximum of 10 rows per page.
- No mock rows, raw CV/JD persistence, source-specific crawler adapter, or arbitrary crawl expansion.
- The countdown is based on persisted `next_cycle_at` and heartbeat freshness; the browser must not invent a successful crawl.
- Preserve existing admin authentication, CSRF, no-store, and structured error behavior.

---

### Task 1: Add crawler runtime and paginated admin schema contracts

**Files:**

- Create `database/migrations/000009_admin_management_runtime.up.sql`.
- Create `database/migrations/000009_admin_management_runtime.down.sql`.
- Modify `database/tests/validate-schema.ps1`.

**Interfaces:** Add singleton `public.crawler_runtime` with `runtime_key`, `status`, heartbeat/cycle timestamps, `next_cycle_at`, current source, last error, and `updated_at`.

**Steps:**

1. Add failing schema assertions for the runtime columns, the singleton key constraint, and allowed runtime statuses. Run the schema validator and capture the expected failure.
2. Add the transactional migration and seed the single `default` row with `OFFLINE` status.
3. Add a down migration that removes only this migration's objects.
4. Apply the migration to the configured PostgreSQL database and rerun schema validation.
5. Record a Baron continuity checkpoint with migration and validation evidence.

### Task 2: Add bounded Job Cache search and paged Job Location/Job Link repository contracts

**Files:**

- `backend/internal/repository/jobs.go`
- `backend/internal/repository/postgres_jobs.go`
- `backend/internal/repository/locations.go`
- `backend/internal/repository/postgres_locations.go`
- `backend/internal/repository/job_links.go`
- `backend/internal/repository/postgres_job_links.go`
- Focused repository tests.

**Interfaces:** Extend `AdminJobFilter` with a trimmed search query. Add `AdminLocationPage`; keep the existing active-location read for public/client contracts and add an admin options read for Job Cache selects.

**Steps:**

1. Add failing repository/query tests for search, page limits, location pages, and active/all location options.
2. Add parameterized search over job title, role, company, location text, source identity, and original URL. Do not concatenate user input into SQL.
3. Add count/page queries for locations and keep public active-location behavior unchanged.
4. Bound repository page sizes to the admin contract and cover empty and last-page cases.
5. Run focused Go repository tests and record the result.

### Task 3: Expose Go admin API contracts

**Files:**

- `backend/internal/model/location.go`
- `backend/internal/model/settings.go`
- `backend/internal/service/settings.go`
- `backend/internal/httpapi/admin_job_handler.go`
- `backend/internal/httpapi/admin_location_handler.go`
- `backend/internal/httpapi/admin_job_link_handler.go`
- `backend/internal/httpapi/admin_settings_handler.go`
- Relevant router and handler tests.

**Interfaces:**

- Job Cache accepts `q`, `location_id`, and `unresolved_location`, with a maximum page size of 10.
- Job Link and Job Location list endpoints return explicit page metadata and enforce page size 10.
- `GET /api/v1/admin/locations/options` returns canonical location options for the Job Cache select.
- Settings returns persisted crawler runtime data in addition to crawler interval settings.
- Existing `PATCH /api/v1/admin/jobs/:id/location` remains the only assignment write boundary.

**Steps:**

1. Add failing handler tests for query parsing, page-size rejection, paged locations, options, and runtime response shape.
2. Implement request parsing and response mapping without changing auth/CSRF middleware.
3. Add stale-heartbeat handling so a runtime with no heartbeat or an expired heartbeat is returned as `OFFLINE`.
4. Run all Go tests and `go vet ./...`.

### Task 4: Persist Rust crawler heartbeat and cycle timing

**Files:**

- `crawler/src/store.rs`
- `crawler/src/main.rs`
- `crawler/src/lib.rs`
- Focused crawler runtime tests.

**Interfaces:** Add storage operations for heartbeat, cycle start/finish, source progress, and recoverable runtime errors.

**Steps:**

1. Add failing pure transition tests for `OFFLINE`, `IDLE`, `RUNNING`, and `ERROR` state updates.
2. Implement PostgreSQL runtime writes using UTC timestamps and the singleton row.
3. Wire the daemon heartbeat into its existing poll loop, mark a cycle `RUNNING` before execution, persist `next_cycle_at` from the actual configured interval, and mark `IDLE`/`ERROR` after completion.
4. Preserve source allowlist, delta/reconcile, missing-cycle, and anomaly semantics; do not add a site-specific extractor.
5. Run `cargo test --manifest-path crawler/Cargo.toml` and `cargo check --manifest-path crawler/Cargo.toml`.

### Task 5: Update frontend API clients and grouped admin navigation

**Files:**

- `frontend/src/lib/admin/api/admin-api.ts`
- `frontend/src/lib/admin/components/AdminShell.svelte`
- `frontend/src/lib/admin/styles/admin.css`
- Focused frontend contract tests.

**Interfaces:** Frontend API clients use page size 10, serialize the Job Cache search/location filters, parse location pages/options, and parse crawler runtime fields.

**Steps:**

1. Add failing client/parser tests for the new query and response contracts.
2. Implement typed API helpers and error handling.
3. Replace the flat navigation with `Manage Job` → Job Link, Job Location, Job Cache and `Manage User` → Users, CV Profiles. Auto-open the active group and preserve desktop/mobile drawer behavior.
4. Increase only the sidebar toggle hit area as needed; do not add explanatory UI copy.
5. Run frontend type checks and focused tests.

### Task 6: Move location assignment into Job Cache and fix list UIs

**Files:**

- `frontend/src/routes/admin/jobs/+page.svelte`
- `frontend/src/lib/admin/components/JobCacheTable.svelte`
- `frontend/src/routes/admin/locations/+page.svelte`
- `frontend/src/routes/admin/sources/+page.svelte`
- `frontend/src/lib/admin/styles/admin.css`
- Focused UI tests.

**Steps:**

1. Add failing UI behavior tests for one lifecycle badge, inline location selection, page size 10, Job Cache search/filter requests, and removal of the old assignment panel.
2. Implement server-backed search and location filters; do not filter a partial page in the browser.
3. Add an inline location select to each Job Cache row, disable it while saving, call the existing assignment endpoint, and refresh the current page.
4. Render only the job lifecycle badge in the Lifecycle column; remove the duplicate source approval badge.
5. Remove job assignment controls from Job Location; keep only canonical location create/edit/active management and its paged list.
6. Set Job Link page size to 10 and add spacing rules so toolbar/table text cannot overlap.
7. Run frontend tests and type checks.

### Task 7: Add truthful Settings runtime card and visual verification

**Files:**

- `frontend/src/routes/admin/settings/+page.svelte`
- `frontend/src/lib/admin/styles/admin.css`
- Runtime display tests and screenshots.

**Steps:**

1. Add failing runtime-display tests for `IDLE`, `RUNNING`, `ERROR`, `OFFLINE`, and a missing next-cycle timestamp.
2. Add settings polling and a one-second presentation tick. Show a live countdown only while the server reports a fresh schedule; show offline/error truthfully otherwise.
3. Clear timers on unmount and respect reduced-motion preferences.
4. Run `bun run --cwd frontend check` and `bun run --cwd frontend build`.
5. Capture real desktop and mobile screenshots for Job Cache, Job Link, Job Location, Settings, and the grouped sidebar.

### Task 8: Full verification and handoff

**Steps:**

1. Run the full gates:

   ```text
   go test ./... -count=1
   go vet ./...
   cargo test --manifest-path crawler/Cargo.toml
   cargo check --manifest-path crawler/Cargo.toml
   bun run --cwd frontend check
   bun run --cwd frontend test:unit
   bun run --cwd frontend build
   ```

2. Run a database-backed authenticated smoke check for page-size limits, Job Cache search, inline assignment, and runtime updates.
3. Inspect the final diff and record any remaining findings; no Git commit is assumed because this workspace is not a Git repository.
4. Record proof receipts where available, run Baron trace scoring, and checkpoint the final state with remaining risks explicitly listed.
