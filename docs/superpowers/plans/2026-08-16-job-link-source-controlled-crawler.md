# Job Link and source-controlled crawler implementation plan

> Execute this plan inline in the current workspace. Keep the implementation
> no-mock and preserve the existing admin auth/database boundaries.

**Goal:** Make Job Link CRUD real in the Go API and add a Rust crawler core that
can only crawl approved URL/path scopes and can reconcile structured observations
without persisting raw job documents.

**Architecture:** Go owns the authenticated admin HTTP contract, URL
normalization, source registry SQL, and latest crawl metadata projection. Rust
owns crawler scope enforcement, deterministic crawl/reconcile domain logic,
and the worker-side transactional adapter for `source_crawl_runs` and
`job_cache`. Both use the existing PostgreSQL schema as the source of truth;
no second database or local persistence is introduced.

**Tech stack:** Go 1.25, Gin, pgx/v5, PostgreSQL; Rust, Spider, Tokio,
tokio-postgres, URL/JSON/hash libraries; existing SvelteKit/Bun admin client.

## Task 1: Lock the Go Job Link contract with failing tests

**Files:** `backend/internal/httpapi/admin_job_link_handler_test.go`,
`backend/internal/service/job_link_test.go`, `backend/internal/repository/job_links_test.go`

1. Add handler tests for unauthenticated GET, missing/invalid CSRF on writes,
   valid create/list/update/delete JSON, bounded pagination, and no raw fields.
2. Add service tests for URL normalization, scheme/userinfo rejection, host/path
   scope normalization, derived display name, stable generated source key, and
   update/delete semantics.
3. Add repository query-contract tests asserting parameterized SQL and the
   `DISABLED` soft-delete behavior.
4. Run the focused Go tests and confirm they fail for the missing Job Link
   implementation, not because of an unrelated compile error.

## Task 2: Implement source registry model, service, repository, and handlers

**Files:**

- `backend/internal/model/job_link.go`
- `backend/internal/repository/job_links.go`
- `backend/internal/repository/postgres_job_links.go`
- `backend/internal/service/job_link.go`
- `backend/internal/httpapi/admin_job_link_handler.go`
- `backend/internal/httpapi/router.go`
- `backend/cmd/api/main.go`

1. Add the bounded request/response model and repository interface.
2. Implement service-owned URL validation and deterministic metadata derivation.
3. Implement parameterized PostgreSQL insert/update/list/disable statements,
   including latest crawl-run projection.
4. Add protected `/job-links` routes under the existing admin session group;
   require the existing CSRF middleware for POST/PATCH/DELETE.
5. Wire the repository/service/handler into the production API without changing
   existing constructor behavior used by current tests.
6. Run focused Go tests, then the full backend test suite.

## Task 3: Connect the existing Job Link UI to the API

**Files:** `frontend/src/lib/admin/api/admin-api.ts`,
`frontend/src/routes/admin/sources/+page.svelte`

1. Add typed list/create/update/disable API functions with response validation.
2. Load real links on page entry and render the real empty state when PostgreSQL
   has no source rows.
3. Enable one URL input, edit mode, and delete confirmation only after the API
   contract succeeds; keep no mock rows or local fake persistence.
4. Preserve the existing admin CSRF token and session-expiry handling.
5. Run Svelte check, frontend tests, and a production build.

## Task 4: Add the Rust crawler boundary and pure tests first

**Files:** `crawler/Cargo.toml`, `crawler/src/lib.rs`,
`crawler/src/scope.rs`, `crawler/src/reconcile.rs`, `crawler/src/main.rs`

1. Add failing tests for same-scope acceptance, host/scheme/port/path
   rejection, fragment handling, and the two-healthy-cycle state transition.
2. Implement the normalized source scope guard and reconciliation state machine.
3. Add a crawler executable that loads `ACTIVE` sources from PostgreSQL, exits
   cleanly when none exist, and does not create mock observations.
4. Configure Spider with robots support, bounded depth/limit, and the scope
   callback; keep page processing in memory and expose an adapter boundary for
   the reviewed source-specific parser.
5. Run Cargo format/check/test and fix compile/API drift against the installed
   Spider release.

## Task 5: Document the boundary and perform security/quality verification

**Files:** `backend/README.md`, `crawler/README.md`,
`docs/baron/architecture/PROJECT_STRUCTURE.md`

1. Update local run commands and state that `tool/` was replaced by the root
   `crawler/` module in the current architecture contract.
2. Verify no URL input reaches a fetch sink without the server-side source
   scope guard, no raw page/job body is persisted, and no admin route bypasses
   session/CSRF enforcement.
3. Run Go format/test/vet, Cargo format/test/check, frontend check/build, and
   focused source audits.
4. Run the mandatory Baron code-reviewer, security-auditor, and test-engineer
   gates; record proof receipts, trace score, continuity checkpoint, and
   remaining unknowns before reporting completion.
