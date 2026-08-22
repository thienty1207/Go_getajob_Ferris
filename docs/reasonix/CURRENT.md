# Reasonix — Continuity Current

## 2026-08-22 — Authoritative handoff after upload/CV reliability verification

- Status: **IMPLEMENTED AND VERIFIED; ready to commit/push**.
- Scope completed: admin Home image upload through Cloudinary, durable metadata and cleanup queue, client-authenticated CV upload, strict DeepSeek parsing, location-scoped matching, structured CV history, owner-scoped delete, stable CSRF, PostgreSQL recovery retry, and no-raw-CV retention.
- Live Home gate: `AdminLogin PASS`, `MultipartUpload PASS`, `DatabaseMetadata PASS`, `PublicHomeRead PASS`, `CloudinaryDelivery PASS (HTTP 200)`, `DurableAssetCleanup PASS`.
- Live CV gate: `MissingCSRF PASS (403)`, `AuthenticatedUpload PASS (202)`, `DeepSeekParse PASS`, `LocationScopedMatch PASS`, `StructuredHistory PASS`, `DeleteAndProfileCleanup PASS`, `RawCVRetention PASS (0 files)`.
- Direct verification: backend `go test ./... -count=1` and `go vet ./...` pass; frontend 24 files/101 tests, `bun run check` 0 errors/0 warnings, and production build pass; crawler PostgreSQL contracts and `cargo check` pass; schema validation pass; isolated full migration apply plus `000014` rollback/re-apply pass.
- Cleanup proof after live gates: `home_asset_cleanup_queue=0`, temporary admin users `=0`, temporary client users `=0`.
- Current local runtime at the time of handoff: PostgreSQL accepting connections, backend health `200`, public Home API `200`, frontend `200`.
- No mock data, raw CV, OAuth secret, Cloudinary credential, or DeepSeek key is tracked. Local `.env` files remain ignored.
- Three mandatory independent reviewer agents were requested but the environment rejected all three because their usage limit was exhausted. Direct code/test/security checks above are recorded; do not claim reviewer receipts exist.
- Next action: stage only the intended source, migration, test, and handoff files; inspect the staged diff and secret scan; commit and push `main` to `origin`.

- Updated: 2026-08-20
- Task: Implement the approved Sugoi-oniichan client experience, owner-scoped CV scan/history, and admin-managed Home sections.
- Status: **IMPLEMENTED THROUGH TRANCHES 0–4 — AUTOMATED GATES GREEN — EXTERNAL LIVE PROOF PENDING**
- Active Baron plan: `client-experience-home-cms-and-cv-history`
- Detailed plan: `docs/superpowers/plans/2026-08-20-client-experience-home-cms-cv-history.md`
- Harness intent: `intent-fffbdb23db59cb15` (`confirmed`)

## Latest runtime incident — 2026-08-20

- Active incident plan: `database-recovery-and-api-readiness`.
- PostgreSQL service is running but currently rejects connections with `FATAL: the database system is in recovery mode`. Logs show an earlier unclean shutdown followed by automatic recovery; no data reset or force-kill is approved.
- The API health endpoint correctly returns `503 database_unavailable`. Before this fix, a cookie-bearing `/api/v1/admin/auth/me` request mapped the storage failure to a generic `500`, which made the admin UI report only that the workspace was unavailable.
- Admin auth storage failures now return `503` with the stable `database_unavailable` code for session lookup, CSRF refresh, login, and logout. Regression tests cover all four response paths.
- The live database also reported `scans.client_user_id` missing during the previous recovery window. Do not apply migrations blindly: after PostgreSQL accepts connections, inspect the schema and apply only the missing migrations in numeric order (`000011`, `000012`, `000013` as applicable).
- Current blocker: this session cannot restart the Windows PostgreSQL service because the service-control operation requires Administrator permission. Operator next action: run `Restart-Service postgresql-x64-18` from an elevated PowerShell, verify `pg_isready` says `accepting connections`, then run the read-only schema query and backend/crawler smoke checks.

## Current implementation state

- Google client OAuth, separate client session/CSRF boundary, read-only Google profile/avatar, profile route, logout, and reload-safe auth state are implemented.
- Client Home is location-only: no radius field or active radius matching branch. The Home result placeholder was removed; a submitted scan redirects to `/client/scans/:scan_id` and polls real backend state.
- CV uploads are authenticated and owner-scoped. PostgreSQL stores structured profile/history and match metadata only; the temporary raw file is cleaned up by the backend.
- Admin Users/CV pages use real paginated APIs. Client CV history is read/delete only and never exposes raw CV content.
- Home CMS uses fixed slots 1–4, plain text/validated URLs, Cloudinary metadata only, and no mock rows. Admin route: `/admin/sections/home`; public route: `/api/v1/client/home-sections`.
- PostgreSQL migrations `000012` and `000013` are applied locally. The four fixed Home slots currently remain inactive/empty until an admin uploads real content.
- SSR regression fixed in `frontend/src/lib/client/components/ClientLocationSelect.svelte`: browser-only document cleanup is now guarded for server rendering.
- Location-only scan processing now accepts the intentionally omitted radius value. `Start` returns `202` and the backend worker owns the bounded DeepSeek/matching timeout and temporary-file cleanup; `Close` cancels and joins workers during API shutdown.
- Home no longer renders the old static hero copy when promotions are empty; the public page stays limited to real promotions, upload, and active CMS sections.
- Home media deletion keeps the database metadata when Cloudinary deletion fails, so an operator can retry instead of silently losing the record.

## Latest non-tech Home CMS simplification

- Updated: 2026-08-20
- Admin Home sections 1–3 now expose only `Tiêu đề`, `Nội dung`, `Ảnh section`, and the publication toggle. `Eyebrow` and user-entered `Alt text` are no longer part of the admin workflow.
- Section 4 accepts at most 10 real uploaded images. The editor only exposes image selection, visibility, and deletion; the backend derives image metadata automatically.
- Public Home no longer renders legacy eyebrow text or target-link CTA fields. Section 4 is a single horizontal marquee on desktop and deterministic two-row marquee on mobile, with hover pause and reduced-motion support.
- Legacy database/API fields remain readable for compatibility, but they are no longer required or rendered by the current UI.
- Verification for this tranche: frontend tests 23 files / 96 tests pass; `bun run check` passes with 0 errors and 0 warnings; `bun run build` passes with a trusted Baron receipt; focused `go vet` passes. Direct focused Go test execution was attempted but Windows Application Control blocked the generated `service.test.exe`, `repository.test.exe`, and `httpapi.test.exe` before assertions ran. Authenticated browser viewport proof remains pending.

## Latest admin Home section editor fix

- Root cause confirmed from the route/API contract: `GET /api/v1/admin/home-sections` legitimately returns `sections: []` before an admin has saved any CMS content. The admin route only iterated persisted sections and only rendered the media-strip editor when slot 4 existed, so the page showed its heading and then nothing.
- `frontend/src/lib/shared/types/home-section.ts` now provides `createEditableHomeSectionSlots`, which fills only the editor's fixed slots 1–4 with empty form state. It does not create database rows, images, copy, or public content.
- `frontend/src/lib/admin/api/admin-api.ts` applies that normalization only to the admin editor response. Public Home parsing remains real-data-only and continues to omit inactive/empty CMS sections.
- The admin Home route therefore always shows the four real management controls, including the empty Section 4 media-strip panel, even on a fresh database. Loading and API error states remain visible.

## Latest admin Home section editor verification

- RED test reproduced the old failure: the empty database response had no editor-slot normalizer and the focused test failed because `createEditableHomeSectionSlots` was missing.
- Focused tests: `home-section.spec.ts` and `admin-api.spec.ts` — 2 files / 9 tests pass, including the empty-database editor contract.
- `bun run check` — 0 errors, 0 warnings.
- `bun run test` — 22 files / 93 tests pass.
- `bun run build` — production client/server build pass; adapter-auto informational warning only.
- Authenticated browser smoke was not available in this session: opening `/admin/sections/home` redirected to `/admin/login`, so no admin credentials were guessed or bypassed. The route/API fix is covered by the focused contract and production build.

## Latest Home CMS public/admin boundary fix

- Code review found the adjacent PostgreSQL predicate was inverted: `ListPublic(true)` used `WHERE $1::boolean`, which included inactive rows, while `ListAdmin(false)` applied the active-only filter and could hide saved drafts.
- `backend/internal/repository/postgres_home_sections.go` now uses `WHERE NOT $1::boolean OR (...)`. Admin reads include all four persisted slots/drafts; public reads include only active sections and active media for the strip.
- `backend/internal/repository/postgres_home_sections_test.go` locks this contract so the public/admin boundary cannot silently reverse again.
- The review finding `finding-db94f98c6c71b15d` is closed with targeted repository and HTTP API verification.
- A second review pass found the same polarity bug in Section 4 media loading. `ListHomeSections` now maps `publicOnly` through `includeInactiveForHomeRead`: public reads exclude inactive media, admin reads retain inactive media drafts. Finding `finding-9ea84b541a6d8741` is closed.

## Latest Home CMS backend verification

- RED repository contract reproduced the old `WHERE $1::boolean` failure.
- `go test ./internal/repository -run TestListHomeSectionsQueryKeepsAdminDraftsAndFiltersPublicRows -count=1` — pass.
- `go test ./internal/httpapi -run TestPublicHomeSectionsReturnsCloudinaryMetadataWithoutStorageIdentifiers -count=1` — pass.
- `go vet ./...` — pass.
- `go test ./internal/repository -run 'TestListHomeSectionsQueryKeepsAdminDraftsAndFiltersPublicRows|TestHomeSectionMediaReadScopeKeepsDraftMediaAdminOnly' -count=1` — pass after the media fix.
- A full `go test ./... -count=1` run reached all relevant packages but Windows Application Control blocked temporary `cloudinary.test.exe` and `service.test.exe`; this is an environment execution policy failure, not a Go assertion failure. The targeted changed-boundary tests passed.

## Latest Home scroll fix

- Root cause confirmed in browser: `.client-page { overflow: hidden; }` prevented document growth from being exposed, while the absolutely positioned ambient decoration increased the internal scroll height to 790px against a 754px page box. The viewport was 720px high, so the user could scroll only a small decorative tail.
- `.client-page` now uses `overflow-x: clip; overflow-y: visible;` so real Home content can extend vertically without creating horizontal overflow.
- Ambient decoration is now `position: fixed`, so it cannot create fake document height or hide the end of real content.
- The public Home still has no visible CMS section because the API reports all four fixed slots inactive/empty. This remains intentional no-mock behavior, not a CSS clipping failure.

## Latest Home scroll verification

- Focused regression: `client-entry-layout.spec.ts` — 5 tests pass, including scroll/overflow and ambient-height guards.
- Browser layout measurement: `document.scrollHeight = 754`, `clientHeight = 720`, `.client-page overflow-x = clip`, `.client-page overflow-y = visible`, ambient position `fixed`.
- Browser screenshot at desktop confirms the existing promotion/upload surface remains within its container and the page has a normal vertical scrollbar.

## Latest UI refinement checkpoint

- Login now uses the shared client header as the only brand surface. The login card no longer repeats the Sugoi-oniichan lockup, and the login header hides the redundant login action.
- Client CV history keeps exactly one scan CTA: the red `Quét CV mới` link. The duplicate `Bắt đầu quét` action was removed.
- Home section components now have safe accessible labels when an admin has not entered a CMS title. Content sections use a responsive alternating layout with a 16:9 media treatment; the media strip keeps its real-data-only rendering and reduced-motion behavior.
- The public Home still has no CMS section blocks because PostgreSQL currently reports all four fixed slots as inactive/empty. This is intentional: no fabricated marketing copy, images, or fixture rows were added. The real next action for visible sections is to publish content through `/admin/sections/home`.

## Latest verification

- Browser smoke at `http://127.0.0.1:5173/client/login`: one Sugoi-oniichan header lockup, one `Đăng nhập` heading, one visible Google CTA, and no duplicate login-card brand.
- Browser smoke at `http://127.0.0.1:5173/client/cv`: one `Quét CV mới` link; no `Bắt đầu quét` action.
- Browser smoke at `http://127.0.0.1:5173/client`: real promotion and upload UI render; inactive Home CMS slots stay absent.
- `bun run check` — 0 errors, 0 warnings.
- `bun run test` — 21 files, 91 tests pass.
- `bun run build` — production client/server build pass; adapter-auto informational warning only.
- `git diff --check` — no diff errors; only Git CRLF normalization warnings.

## Verification checkpoint

- `go test ./... -count=1` — pass.
- `go vet ./...` — pass.
- `bun run check` — 0 errors, 0 warnings.
- `bun run test` — 20 files, 86 tests pass.
- `bun run build` — client and server production build pass; adapter-auto informational warning only.
- `cargo test` — 31 unit tests and 5 non-DB integration tests pass; 2 DB contracts remain ignored unless explicitly run against PostgreSQL.
- `cargo check` — pass.
- Live crawler PostgreSQL contracts `location_ownership_contract` and `postgres_contract` — pass when run with a process-local `DATABASE_URL` built from the backend environment.
- Go repository integration contracts (`go test -tags integration ./internal/repository -run "TestPostgres" -count=1`) — pass against local PostgreSQL.
- `database/tests/validate-schema.ps1` — Schema contract passed.
- `git diff --check` — no diff errors; only Git CRLF normalization warnings.
- Local public smoke: `GET /client` returned `200`, contained `Sugoi-oniichan`, and did not contain the removed static hero copy; `GET /api/v1/client/home-sections` returned the four real inactive slots.
- Browser smoke on `http://127.0.0.1:5173`: real promotions and Job Locations rendered; mobile location listbox opened; login page exposed a visible `Tiếp tục với Google` button. A secondary `5174` server is not a supported origin because backend CORS is intentionally allowlisted to `5173`.

## Exact next action

1. Run the remaining live proof with the developer's configured Google test account: login → callback → `/me` → avatar/profile → logout.
2. Run one real authenticated CV scan with the configured DeepSeek key and verify structured profile, deterministic matches, raw file cleanup, and owner isolation.
3. Upload/update/delete a real Home section image through Cloudinary and verify the public Home refresh.
4. Run the security/code/test review gates, then update this file and `WORKLOG.md` with the final receipts. Do not fabricate proof if any external service is unavailable.

## Locked direction

- Brand is **Sugoi-oniichan** (`Sugoi` orange, `oniichan` blue); product is **Go get a job ferris**.
- Client filter is canonical Job Location only; remove radius from UI and active scan/matching contract.
- Home becomes promotion + upload + four API-backed sections; results move to `/client/scans/:scan_id`.
- “Saved CV” means owner-scoped structured profile/history only. Raw uploaded file and extracted raw text are temporary and must be deleted after parsing.
- Home sections are fixed-slot, plain-text/validated-URL CMS content with Cloudinary image metadata; no arbitrary HTML and no mock rows.
- Existing admin/client authentication boundaries remain separate.

## Remaining proof boundary

- Google OAuth is implemented and locally contract-tested, but the full browser callback/reload/logout flow still needs the developer's Google test account and running backend.
- DeepSeek parsing, Cloudinary Home upload/update/delete, and authenticated scan ownership still need live external-service proof. Do not infer success from the automated unit/integration gates.
- The in-process scan worker is bounded and shutdown-aware for this local MVP; a durable queue/recovery record for hard process termination remains production hardening.

## Resume protocol

1. Read the detailed plan and this section.
2. Check `baron continuity status` and `baron harness intent-status`.
3. Treat the implementation state and exact next action above as the resume point; do not restart completed tranches.
4. Preserve all pre-existing uncommitted Reasonix/Codex changes; no reset, cleanup, commit, or push unless requested.

---

(Previous continuity records are retained below.)

- Updated: 2026-08-19
- Task: Dev Client Google Login + User Profile. Add a full Google OAuth client login flow, client session, real Google profile display, and logout — fully separate from the admin auth.
- Status: **implemented; verified against live PostgreSQL; migration 000011 applied locally**
- Branch: `main` (uncommitted working tree)

## What shipped
- New client auth domain (`client_users`, `client_google_identities` [UNIQUE google_sub], `client_sessions`) via migration `000011_client_auth`, distinct `ferris_client_session` HttpOnly cookie and `RequireClientSession`/`RequireClientCSRF` middleware so a client session never authorizes admin routes and vice versa.
- Endpoints under `/api/v1/client/auth`: `GET /google` (state cookie + redirect), `GET /google/callback` (verify state, exchange code, verify OIDC ID token with google.golang.org/api/idtoken, upsert user by `sub`, create session), `GET /me` (returns user + csrf), `POST /logout` (CSRF-guarded, server-side revoke).
- Frontend: `/client/login`, `/client/profile`, rewritten `ClientHeader` (Google avatar + dropdown: Profile / CV của tôi [disabled] / Đăng xuất), `client-auth-store` + `client-auth-api` (credentials: include).
- Official Google Go libs added (`golang.org/x/oauth2`, `google.golang.org/api/idtoken`); logout mirrors the admin CSRF mutation pattern.

## Verification
- `go test ./... -count=1`, `go vet ./...` green; new client auth handler/service/integration tests pass.
- Live PostgreSQL `postgres_client_auth_integration_test` passed (unique identity, digest session, admin separation); zero leaked client rows.
- `database/tests/validate-schema.ps1` — "Schema contract passed" (incl. 000011).
- `bun run check` (0 errors), `bun run test` (79 pass incl. client-auth specs), `bun run build` succeeds.

## Security notes
- No `GOOGLE_CLIENT_SECRET` logged/committed/sent to frontend; `backend/.env` not committed. No raw OAuth/access/refresh token, subject, or secret persisted or exposed. Client sessions never reach admin endpoints.
- Independent security review completed: identity anchored on Google `sub` (not email) to prevent account convergence across shared emails; no client-supplied `redirect` on the OAuth callback (no open-redirect surface); `GOOGLE_REDIRECT_URL` requires HTTPS in production.

## Not in scope (per task)
- CV upload/save/parse/scan, DeepSeek, profile editing; "CV của tôi" is a disabled menu item only.

## Next action
- Live manual smoke (login → Google redirect → callback → /me → avatar menu → profile → reload → logout → 401) needs a real Google Test user + OAuth client (no browser tool in-session). Commit/push only on user request.

---

(Previous task retained below: admin-assigned job location lost after crawler restart/re-crawl.)

# Reasonix — Continuity Current (previous task: admin-assigned job location lost)

- Updated: 2026-08-19
- Task: Fix job_cache.location_id lost after crawler restart/re-crawl (URGENT BUG FIX). Admin-assigned canonical locations reverted to NULL because the crawler upsert blind-overwrote `location_id = EXCLUDED.location_id`.
- Status: **implemented; verified against live PostgreSQL; migration 000010 applied locally**
- Branch: `main` (uncommitted working tree)

## Root cause (confirmed by code + live DB)
1. `crawler/src/store.rs` `UPSERT_JOB_CACHE_QUERY` `ON CONFLICT ... DO UPDATE` set `location_id = EXCLUDED.location_id`.
2. The crawler's `resolve_location_id()` returns `None` whenever source `location_text` does not match a canonical location/alias (parser anomaly, changed text, or unresolvable text).
3. On the next crawl of the same stable `source_id + source_job_key`, that NULL overwrote the admin-assigned `location_id`. No ownership column existed to let the crawler distinguish admin assignment from auto-resolution. `reconcile_missing_jobs` does not touch location_id; `source_job_key` is stable (no row duplication).

## Design — location ownership marker
- New migration `000010_location_assignment_source`: `job_cache.location_assignment_source text NOT NULL DEFAULT 'AUTO'`, `CHECK IN ('AUTO','ADMIN')`.
- Crawler inserts rows as `AUTO`; on update, preserves ADMIN rows' `location_id` and never lets an unresolved NULL clobber an existing value.
- Admin `PATCH /api/v1/admin/jobs/:id/location`: set location → `'ADMIN'`; clear → `'AUTO'` (reverts to auto resolution).

## What changed
- Migration + README + validate-schema.ps1 checks; crawler upsert + integration contract test; backend repository/service/handler write + expose `location_assignment_source`.

## Verification
- `cargo test` (31 lib incl. 3 location guard tests; integration contracts passed live), `cargo check`, `cargo fmt --check`
- `go test ./... -count=1`, `go vet ./...`
- `database/tests/validate-schema.ps1` — "Schema contract passed"
- Migration `000010` applied to local DB; 8 existing rows defaulted AUTO.
- Crawler `location_ownership_contract` + `postgres_contract` passed against the migrated DB.

## Data recovery (PHASE 5)
- Live DB: **8 jobs, all `location_id = NULL`**; assignments already lost pre-fix.
- No audit history (`admin_audit_events` has only login/logout) → **no reconstruction possible**; recovery is preventing future loss only.

---

(Previous task retained below: Viettel employment-type badge bug.)

# Reasonix — Continuity Current (previous task)

- Updated: 2026-08-19
- Task: Fix real bug in Job Cache Employment Type — Viettel job rendered stray "." badges (e.g. [Toàn thời gian] [.] [Bán thời gian] [.] ...). Root cause traced to both crawler normalization and frontend parser. Frontend + crawler (no schema/mock).
- Status: **implemented; frontend gates green; crawler cargo check green; crawler unit test blocked by a running ferris-crawler.exe lock**
- Branch: `main` (uncommitted working tree)

## Root cause (trace)
1. **Crawler** (`crawler/src/crawl.rs` `normalize_employment_type`) split only on `['-',' ','/']`. A source value such as `FULL_TIME.PART_TIME.CONTRACTOR...` (dot-separated) or an array joined with `", "` would keep those punctuation separators verbatim into the stored `employment_type`.
2. **Frontend** (`frontend/src/lib/admin/job-labels.ts` `employmentLabels`) split only on `_`, so leftover punctuation (`.`, `,`, etc.) in the stored value collapsed into separate badges.
3. **Template/CSS** are clean — employment is rendered purely via `employmentLabels(...)` in `{#each}`; `.job-type-badge` has no `::before`/`::after`. So the stray "." badge came only from the parser output. (Confirmed: no CSS `::before/::after` on badges.)

## What changed
- **Frontend `job-labels.ts`** — `employmentLabels` rewritten to regex-match the full enum list atomically (accepting `_`, `-`, or space as intra-enum separators: `FULL_TIME`==`FULL TIME`==`FULL-TIME`) and to **drop all punctuation separators** (`.`, `,`, `;`, `|`, `/`) from the gaps. No badge can ever be a `.`/`,`/empty token. Unknown alnum tokens are still kept (uppercased), so no data is lost.
- **Frontend `job-labels.spec.ts`** — added tests for dot/comma/slash/space forms and an invariant that no punctuation-only badge is ever emitted.
- **Frontend `styles/admin.css`** — `.job-type-badge` `display:inline-flex; width:max-content; white-space:nowrap` (already present this session).
- **Crawler `crawl.rs`** — `normalize_employment_type` now splits on `[.,;|/- ' '\t]` so future crawls store a canonical underscore-joined enum string (dot/comma forms collapse). Added Rust test `normalize_employment_type_handles_mixed_separators`. Existing DB rows need no migration — the frontend parser handles any legacy separator.

Earlier context in this session (still in the working tree):
- **`frontend/src/lib/admin/job-labels.ts`** (+ `job-labels.spec.ts`): pure presentation mappers —
  - `workModeLabel(workMode)`: `ONSITE`→`Tại văn phòng`, `HYBRID`→`Hybrid`, `REMOTE`→`Remote`, unknown → uppercase words.
  - `employmentLabels(employmentType)`: maps each full enum to Vietnamese labels (see note above for the hardened tokenizer); no raw underscore enum shown.
  - `sourceDisplayLabel(sourceName)`: strips scheme/www/trailing-slash → short domain/display name.
- **`JobCacheTable.svelte`**:
  - Desktop table: Source column now shows only `sourceDisplayLabel(sourceName)` (source UUID/key removed); Mode col uses a colored `job-mode-badge`; a new "Type" column renders employment chips; the old Hash column is replaced by an "Original" link only (`contentHash` no longer rendered; aria-label + title added).
  - Mobile card: labeled vertical blocks — Lifecycle badge / Chế độ làm việc (mode badge) / Loại hình (employment chips) / Nguồn (domain) / Cập nhật lần cuối; Source UUID and hash removed; Original ↗ link wrap-safe with accessibility.
- **`admin.css`**: added `.job-mode-badge` (`is-remote`/`is-hybrid`/`is-onsite` color variants, non-monochrome), `.job-type-badges`+`.job-type-badge` wrap-safe chips, `.job-mobile-wide`; removed the now-unused `.job-hash` and `.job-source-cell small` rules.

Load-bearing invariants:
- Source/API fields (`source_key`, `content_hash`, work/employment enums) are **not** removed from backend/database/API — only the presentation layer hides them.
- No mock data, no hard-coded jobs, no `transform: scale`, no font shrink, no `overflow-x: hidden` cheat.
- Pagination / search / location filter / `PATCH location_id` unchanged.

## Verification
- `bun run check` — 0 errors, 0 warnings
- `bun run test` — 70/70 pass (incl. mixed-separator employment tests)
- `bun run build` — vite production build succeeds
- `cargo check` (crawler) — passes (compiles the normalize + test changes)
- `cargo test` (crawler) — BLOCKED by running `ferris-crawler.exe` (PID 18084) holding the `target\debug` exe → "Access denied" at rebuild. Not a code failure; rerun after that process stops.

## Limitations
- No browser tool in this session (no `playwright-cli`, no running dev server with auth), so I could not capture before/after screenshots at 320/375/390/768px/desktop or click-test dropdowns on a real viewport. All logic changes are pure functions covered by unit tests and the markup is static, but a real browser visual check (long titles, multi employment chips, long source domain, last-card dropdown) is still recommended before shipping.

## Next action
- Stop the running `ferris-crawler.exe` (PID 18084) or restart it, then `cargo test normalize_employment_type` in `crawler/` so the added Rust test actually executes.
- Run locally and open `/admin/jobs`; confirm the Viettel job now shows clean badges `[Toàn thời gian] [Bán thời gian] [Hợp đồng] ...` with **no stray "."/"," badges**, at desktop and mobile; existing DB rows (dot/comma/legacy forms) also render correctly without a migration.
- Then commit if the user asks. Do not commit/push otherwise.
