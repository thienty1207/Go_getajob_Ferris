# Job Link, location, crawler, and promotion fixes — implementation plan

> Execute this plan in the existing workspace. Keep all changes source-controlled and no-mock.

## 1. Lock the contracts and establish failing tests

Files:

- `backend/internal/repository/job_links.go`
- `backend/internal/service/job_link.go`
- `backend/internal/httpapi/admin_job_link_handler.go`
- `backend/internal/httpapi/router.go`
- `frontend/src/lib/admin/api/admin-api.ts`
- `frontend/src/routes/admin/sources/+page.svelte`
- `crawler/src/crawl.rs`
- `crawler/src/store.rs`

Work:

1. Add tests for hard delete versus status change, including the requested status values and invalid input.
2. Add frontend API/component tests for the distinct pause/resume/delete actions and the proxy-compatible URL path.
3. Add crawler fixtures/tests for `.env` fallback configuration and JSON-LD `JobPosting` extraction without persisting description HTML.
4. Run the focused test commands and record the expected failures before implementation.

## 2. Repair Job Link CRUD semantics and local API connectivity

Files:

- `backend/internal/repository/job_links.go`
- `backend/internal/repository/postgres_job_links.go`
- `backend/internal/service/job_link.go`
- `backend/internal/httpapi/admin_job_link_handler.go`
- `backend/internal/httpapi/errors.go`
- `backend/internal/httpapi/router.go`
- `frontend/vite.config.ts`
- `frontend/src/lib/admin/api/admin-api.ts`
- `frontend/src/routes/admin/sources/+page.svelte`

Work:

1. Add repository/service delete methods that remove source-owned runs, matches, cache rows, and the source in one transaction.
2. Add a status service/repository method and `PATCH /job-links/:id/status` for `ACTIVE`/`DISABLED`.
3. Keep delete as `DELETE`, return 404 for an unknown ID, and map conflicts/validation consistently.
4. Add a Vite development proxy for `/api` to the Go API with an environment override.
5. Replace “Xóa” soft-disable behavior with a confirmed hard delete; render “Dừng” or “Khởi động” as the separate state action.
6. Refresh the list after each mutation and keep the real backend error visible when the service is unavailable.

Tests:

- `go test ./internal/repository ./internal/service ./internal/httpapi`
- `bun run test -- --run` (or the repository’s supported Vitest command)
- manual authenticated create, pause, resume, and confirmed delete against PostgreSQL.

## 3. Add canonical locations and job assignment

Files:

- `database/migrations/000005_canonical_locations.up.sql`
- `database/migrations/000005_canonical_locations.down.sql`
- `backend/internal/model/...` (the existing model files that own admin locations/jobs)
- `backend/internal/repository/...`
- `backend/internal/service/...`
- `backend/internal/httpapi/...`
- `frontend/src/lib/admin/api/admin-api.ts`
- `frontend/src/routes/admin/locations/+page.svelte`
- `frontend/src/routes/admin/jobs/+page.svelte`
- `frontend/src/lib/admin/components/JobCacheTable.svelte`

Work:

1. Create `locations` and add nullable `job_cache.location_id` with `ON DELETE SET NULL`.
2. Add admin list/create/update endpoints and a job location assignment endpoint.
3. Return canonical location fields in admin job responses and prefer the canonical name in `active_job_cache`.
4. Replace disabled location controls with real form/list/assignment controls. Use empty states when the database has no rows.
5. Preserve `location_text` as the crawler’s source text; never rewrite it when an admin assigns a canonical location.

Tests:

- migration apply/rollback against the local PostgreSQL database;
- service validation and repository query tests;
- API tests for create, update, assign, clear, and not-found cases;
- frontend typecheck/build and a manual location assignment smoke test.

## 4. Make the crawler self-configuring and real for approved JobPosting pages

Files:

- `crawler/Cargo.toml`
- `crawler/src/config.rs`
- `crawler/src/main.rs`
- `crawler/src/crawl.rs`
- `crawler/src/store.rs`
- `crawler/README.md`

Work:

1. Load `crawler/.env` at startup with exported `DATABASE_URL` precedence and the existing local credential aliases as fallback.
2. Add the JSON-LD parser dependencies and a `JsonLdJobPostingAdapter` implementation.
3. Parse only structured JobPosting JSON-LD from pages already inside the source scope; map salary where present and hash structured content.
4. Wire the adapter into the existing healthy-cycle/delta/reconcile store path. Keep parser errors, HTTP errors, and empty/non-authoritative observations out of missing-cycle counts.
5. Extend persistence for salary and canonical-location-compatible fields without storing raw HTML/JD.
6. Document `cargo run` from `crawler/`, the `.env` precedence, and the requirement that an `ACTIVE` Job Link be approved for the exact source scope.

Tests:

- `cargo test`
- `cargo fmt -- --check`
- `cargo clippy --all-targets --all-features -- -D warnings`
- a live, read-only crawl smoke test against a user-approved server-rendered JobPosting URL; if the registered listing is client-rendered and its API is outside scope, report that as a source-contract limitation rather than widening the crawler.

## 5. Refine client/admin promotion ratio

Files:

- `frontend/src/lib/client/components/PromotionCarousel.svelte`
- `frontend/src/lib/admin/styles/admin.css`
- `frontend/src/lib/admin/components/PromotionSlotCard.svelte`

Work:

1. Remove the client carousel’s outer border.
2. Use a 16:9 aspect-ratio surface and retain readable controls over the image.
3. Match the admin preview to 16:9 and retain existing file-type/size validation.
4. Add only the concise 1920x1080/16:9 upload hint needed to prevent mis-sized artwork; do not add decorative explanatory copy.

Tests:

- frontend typecheck/build;
- responsive visual smoke at desktop and mobile widths;
- verify the carousel still advances and displays the uploaded Cloudinary URL.

## 6. Verification and handoff

1. Apply migrations and run the focused backend, crawler, and frontend checks.
2. Start Go API, frontend dev server, and crawler with the documented commands; verify only one API process owns port 8080.
3. Exercise Job Link create → pause → resume → hard delete and confirm the deleted source-owned rows are gone from PostgreSQL.
4. Exercise location create → assign → clear and confirm `location_text` remains unchanged.
5. Exercise an approved JobPosting crawl and confirm a structured job appears with no raw JD/HTML persisted.
6. Capture desktop/mobile promotion screenshots and record the changed-surface evidence.
7. Run Baron proof/trace review and report any source-permission or live-crawl limitation explicitly.
