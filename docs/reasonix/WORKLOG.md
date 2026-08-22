# Reasonix — Worklog

## 2026-08-22 — Final upload workflow verification before push

**Status:** implementation complete; direct automated, database, Cloudinary, DeepSeek, and live API gates pass.

**Verified:**
- Full backend Go tests and vet pass.
- Frontend unit suite: 24 files / 101 tests pass; Svelte check has 0 errors and 0 warnings; production build pass.
- Crawler PostgreSQL contract tests pass with `DATABASE_URL` loaded from the local backend environment; `cargo check` pass.
- Isolated database rehearsal applies all 14 up migrations, rolls back and re-applies `000014_home_asset_cleanup`, then removes the temporary database.
- Real Home upload gate proves admin auth, multipart upload, PostgreSQL metadata, public API read, Cloudinary HTTP 200 delivery, and durable cleanup.
- Real CV gate proves CSRF rejection, authenticated upload, DeepSeek parse, canonical-location matching, structured history, owner-scoped delete/profile cleanup, and zero raw CV files.
- Schema validation pass; post-gate database cleanup is zero temporary users and zero cleanup jobs.

**Operational note:** the dev API/frontend were restarted for live verification and are currently available on `127.0.0.1:8080` and `127.0.0.1:5173` while this session remains active. If the terminal is closed, restart them with the documented commands.

**Review note:** the three mandatory reviewer agents could not run because the environment usage limit was exhausted. No reviewer result or Baron receipt is fabricated; the direct evidence is the source of truth for this handoff.

## 2026-08-20 — Simplify Home CMS fields and add responsive 10-image marquee

**Status:** implementation complete; frontend automated gates and backend vet pass; Go test execution is blocked by Windows Application Control before assertions; authenticated browser viewport proof pending

**Approved product rule:** Home CMS is designed for non-technical admins. Sections 1–3 need only a title, body, image, and publication toggle. Section 4 is an image strip with a maximum of 10 real uploads. No admin-facing eyebrow, alt-text, target URL, link, or technical ordering field is required.

**Change:**
- `frontend/src/routes/admin/sections/home/+page.svelte`: reduced the editor to the approved fields; section 4 enforces a 10-image UI limit and hides technical metadata.
- `frontend/src/lib/admin/api/admin-api.ts`: sends only title/body/image for sections and sort order/visibility/image for media. Legacy response fields remain parsed for compatibility.
- `backend/internal/service/home_section_service.go`: derives image metadata automatically, rejects new media after 10 items, and preserves the no-raw/no-mock boundary.
- `backend/internal/repository/postgres_home_sections.go`: enforces the application/repository limit of 10 media items without destructive migration of legacy rows.
- `frontend/src/lib/client/components/HomeContentSection.svelte`: renders title/body/image only; legacy eyebrow and target links are not public output.
- `frontend/src/lib/client/components/HomeMediaMarquee.svelte` and `frontend/src/lib/shared/styles/global.css`: desktop uses one continuous horizontal marquee; mobile splits real items into two opposing rows. Hover pauses movement and reduced-motion users receive a paused presentation.

**No-mock boundary:** empty editor slots are UI controls only. No fake title, body, image, job, or logo data is introduced. Public Home still renders only active API content.

**Verification:**
- RED: focused frontend editor tests failed against the old eyebrow/alt-text/12-image contract before implementation.
- `bun run test` — 23 files / 96 tests pass.
- `bun run check` — 0 errors, 0 warnings.
- Focused Go Home-section service/repository/http API tests — blocked before assertions by Windows Application Control refusing the generated test executables in `%TEMP%`; no test failure was observed.
- Focused `go vet` for changed backend packages — pass.
- `bun run build` — pass; Baron trusted receipt `receipt-333648290cda` recorded.
- Authenticated desktop/mobile browser smoke — not verified in this session.

**Next action:** inspect the authenticated admin editor and public Home at desktop/mobile widths. If browser access or Windows test execution policy remains unavailable, preserve both as unverified proof items rather than guessing.

## 2026-08-20 — Fix blank admin Home section editor on an empty database

**Status:** implemented; focused tests, check, and production build pass; authenticated browser smoke pending

**Observed problem:** The sidebar exposed `Quản lý section → Trang Home`, but the page displayed only `Quản lý section.` and no editor controls.

**Root cause:** The backend correctly returned an empty `sections` array because no Home CMS rows had been saved. The route rendered only `sections.filter(slot < 4)` and rendered Section 4 only when a persisted slot 4 existed. This made an empty database look like a missing feature.

**Change:**
- `frontend/src/lib/shared/types/home-section.ts`: added `createEditableHomeSectionSlots`, which supplies empty editor state for fixed slots 1–4 with the approved layouts.
- `frontend/src/lib/admin/api/admin-api.ts`: applies the fixed-slot normalization only to the admin API response.
- `frontend/src/lib/shared/types/home-section.spec.ts`: added the empty-database regression contract.
- `frontend/src/lib/admin/api/admin-api.spec.ts`: added the real admin API empty-response contract.

**No-mock boundary:** These are UI form slots, not fake CMS records. No PostgreSQL row, image, title, body, or public Home content is fabricated. The client still renders only active content returned by the public API.

**Verification:**
- RED focused test reproduced the old missing-normalizer failure.
- Focused tests: 2 files / 9 tests pass.
- `bun run check` — 0 errors, 0 warnings.
- `bun run test` — 22 files / 93 tests pass.
- `bun run build` — production client/server build pass; adapter-auto informational warning only.
- Browser smoke was blocked by the available browser session being anonymous and redirecting to `/admin/login`; no credentials were guessed or bypassed.

**Next action:** Open `/admin/sections/home` with the real admin session and confirm the four empty editors render, then save one real section image/copy through Cloudinary to verify the end-to-end CMS path.

## 2026-08-20 — Fix Home CMS public/admin filtering boundary

**Status:** implemented; finding closed; targeted backend verification pass; full Go suite partially blocked by Windows Application Control

**Review finding:** The repository query used `WHERE $1::boolean OR (...)` while the handler passed `true` for public and `false` for admin. That was backwards: anonymous public reads could receive inactive CMS rows, and admin reads could omit inactive drafts.

**Change:**
- `backend/internal/repository/postgres_home_sections.go`: changed the predicate to `WHERE NOT $1::boolean OR (...)`. `publicOnly=true` now means active-only, while `publicOnly=false` means all admin slots/drafts.
- `backend/internal/repository/postgres_home_sections_test.go`: added a query contract covering the corrected boolean direction and active predicate.
- Closed Baron finding `finding-db94f98c6c71b15d` after the fix and targeted verification.

**Verification:**
- RED repository contract failed against the old predicate.
- `go test ./internal/repository -run TestListHomeSectionsQueryKeepsAdminDraftsAndFiltersPublicRows -count=1` — pass.
- `go test ./internal/httpapi -run TestPublicHomeSectionsReturnsCloudinaryMetadataWithoutStorageIdentifiers -count=1` — pass.
- `go vet ./...` — pass.
- Full `go test ./... -count=1` was blocked only when Windows Application Control refused temporary `cloudinary.test.exe` and `service.test.exe`; all relevant changed packages reported pass before those policy failures.

**No-mock/public boundary:** The fix changes only query filtering. It does not seed content or expose storage identifiers. Public response filtering is now enforced in PostgreSQL, not trusted solely to the client.

## 2026-08-20 — Correct Home CMS media draft visibility

**Status:** implemented; review finding closed; focused repository verification pass

**Review finding:** After correcting the section-level predicate, `ListHomeSections` still passed `publicOnly` directly into the media loader's `includeInactive` argument. That left inactive media exposed to public reads and hidden from the admin editor.

**Change:**
- `backend/internal/repository/postgres_home_sections.go`: added `includeInactiveForHomeRead(publicOnly)`, returning the inverse of `publicOnly`, and uses it when loading Section 4 media.
- `backend/internal/repository/postgres_home_sections_test.go`: added a truth-table regression: public reads exclude inactive media; admin reads retain inactive media drafts.
- Closed Baron finding `finding-9ea84b541a6d8741` after verification.

**Verification:**
- RED test reproduced the missing media-scope helper before the production change.
- `go test ./internal/repository -run 'TestListHomeSectionsQueryKeepsAdminDraftsAndFiltersPublicRows|TestHomeSectionMediaReadScopeKeepsDraftMediaAdminOnly' -count=1` — pass.
- `go vet ./...` — pass.
- The existing public Home handler contract passed as well.

**Boundary:** Admin receives all persisted draft state; public receives active sections and active media only. No mock content or storage identifiers were introduced.

## 2026-08-20 — Fix client Home vertical scroll and decorative overflow

**Status:** implemented; focused test and browser layout verification pass

**Observed problem:** The Home looked fixed to one viewport. Wheel scrolling exposed almost no content, and future CMS sections risked being clipped.

**Root cause:** `.client-page` used `overflow: hidden`. Its absolute ambient circle extended the internal scroll height to 790px while the page box was 754px and the browser viewport was 720px. The outer document therefore exposed only a small decorative tail rather than allowing normal content growth.

**Change:**
- `frontend/src/lib/shared/styles/global.css`: `.client-page` now clips only horizontal decoration (`overflow-x: clip`) and leaves vertical flow visible (`overflow-y: visible`).
- Ambient decoration is `position: fixed`, keeping visual rings out of document height while preserving the existing composition.
- `frontend/src/lib/client/components/client-entry-layout.spec.ts`: added regression guards for Home vertical scroll, horizontal clipping, and fixed ambient decoration.

**Verification:**
- RED test observed before the CSS changes; it failed because `.client-page` still used `overflow: hidden` and ambient decoration was `absolute`.
- Focused test: 5 tests pass.
- Browser measurement: document scroll height 754px vs viewport 720px, `.client-page` computed overflow `clip visible`, ambient computed position `fixed`.
- Existing Home API still returns four inactive/empty slots; no section fixture or mock content was added.

**Next action:** Publish real content/images from `/admin/sections/home` to verify the full multi-section page height and mobile scroll in production-like data. Do not seed fake CMS content.

## 2026-08-20 — Client login/CV CTA cleanup and Home section UI refinement

**Status:** implemented; frontend gates green; external CMS content and live provider proof remain pending

**User-facing fixes:**
- Removed the repeated Sugoi-oniichan lockup from the client login card. Login uses the shared header brand only and hides the redundant header login action.
- Removed the duplicate `Bắt đầu quét` CTA from the client CV history route. `Quét CV mới` is now the single red action and still navigates to the scan Home.
- Refined Home CMS content-section layout for desktop/mobile and added safe accessible labels for empty CMS titles. Existing real-data-only behavior remains unchanged.

**No-mock boundary:**
- The client Home currently has no visible CMS content because all four fixed PostgreSQL Home slots are inactive/empty. This is an honest empty state, not a rendering failure. An admin must publish real section copy/images at `/admin/sections/home` before those sections appear publicly.
- No fixture jobs, marketing copy, images, or fake section rows were seeded.

**Files changed:**
- `frontend/src/lib/client/components/ClientHeader.svelte`
- `frontend/src/routes/client/login/+page.svelte`
- `frontend/src/routes/client/cv/+page.svelte`
- `frontend/src/lib/client/components/HomeContentSection.svelte`
- `frontend/src/lib/client/components/HomeMediaMarquee.svelte`
- `frontend/src/lib/shared/styles/global.css`
- `frontend/src/lib/client/components/client-entry-layout.spec.ts`

**Verification:**
- Browser smoke: `/client/login` has one header brand and one Google CTA; `/client/cv` has one `Quét CV mới`; `/client` renders real promotion/upload and omits inactive CMS slots.
- `bun run check` — 0 errors, 0 warnings.
- `bun run test` — 21 files / 91 tests pass.
- `bun run build` — pass.
- `git diff --check` — pass; only CRLF normalization warnings.

**Next action:** publish real Home section content through the existing admin CMS, then run Cloudinary upload/update/delete and full Google/DeepSeek live proof. Do not add mock content to make the public Home look populated.

## 2026-08-20 — Client experience, Home CMS, and CV history planning

**Status:** plan complete; implementation intentionally not started; awaiting user confirmation

**Plan:** `docs/superpowers/plans/2026-08-20-client-experience-home-cms-cv-history.md`

**Planned scope:** repair/prove Google OAuth locally; apply Sugoi-oniichan brand and client typography; remove radius and use canonical location only; require owner-scoped scans; persist structured CV history without raw files; move scan progress/results to a dedicated route; wire admin Users/CVs to real data; add four admin-managed Home sections backed by PostgreSQL and Cloudinary.

**Execution shape:** six gated tranches. Tranche 0 (OAuth) must pass live browser smoke before later database/API/UI work begins. Every tranche must update Reasonix and Baron continuity so a power/network interruption is recoverable.

**Pending decision:** confirm recommended Option A — Google login is required before CV submission. No code implementation is allowed until confirmed.

**Security boundary:** secrets in `.env` are backend-only and must never be copied to documentation, tests, logs, frontend bundles, commits, or final reports. Uploaded CV bytes and extracted raw text remain temporary.

---

## 2026-08-19 — Fix Job Cache Job Location dropdown clipping

**Status:** implemented; gates green; browser E2E unavailable in-session

**Symptom (user-reported):** In the Job Cache table, last rows' job content was cut/overflowing due to table horizontal overflow, the Location dropdown clipped below the container, only "Chưa gán" / "Hà Nội" were visible, and the page vertical scrollbar disappeared while the dropdown was open.

**Root cause:** `LocationSelect` menu was `position:absolute` in a table cell under two clipping ancestors — `.job-table-wrap` (`overflow-x:auto`, which per the CSS scrollbox rule also clips vertically) and `.admin-root` (`overflow:hidden`). No ancestor had `transform`, so a `position:fixed` menu can escape all of them.

**Change scope:** frontend only (SvelteKit + Svelte 5). Backend/API/database/PDC unchanged.

**Files changed:**
- `frontend/src/lib/admin/location-select-position.ts` (new) — pure viewport positioning
- `frontend/src/lib/admin/location-select-position.spec.ts` (new) — 7 unit tests
- `frontend/src/lib/admin/components/LocationSelect.svelte` — fixed-position portal menu + scroll/resize handling
- `frontend/src/lib/admin/styles/admin.css` — portal/listbox styles

**Verification:**
- `bun run check` → 0 errors, 0 warnings
- `bun run test` → 54/54 pass (15 files)
- `bun run build` → success

**Left undone / next:** live browser visual check at ≥10 jobs + ≥5 locations, desktop and narrow viewport (no browser tool available this session). Commit only on user request.

## 2026-08-19 — Fix Admin Job Cache responsive on mobile

**Status:** implemented; gates green; browser E2E unavailable in-session

**Symptom (user-reported):** At ~320–390px the Job Cache page did not show job info fully: content spilled horizontally, long titles/company/address were clipped, employment type + source key stuck on one line and overflowed, the Location dropdown widened the card, and some rows were hidden because the desktop table did not switch to a mobile layout.

**Root cause:** mobile cards only kicked in at `max-width: 560px` (so 561–768px still rendered the `min-width: 1020px` table → horizontal overflow); cards lacked `min-width: 0` + wrap rules; `LocationSelect` kept `min-width: 150px`; several metadata fields were missing.

**Change scope:** frontend only (`JobCacheTable.svelte`, `admin.css`). Response shape and `PATCH location_id` unchanged.

**Files changed:**
- `frontend/src/lib/admin/components/JobCacheTable.svelte` — mobile card rebuilt into labeled grid (title / company·location / Location dropdown / Lifecycle / Mode / Employment / Source / Last seen / Hash / Original URL)
- `frontend/src/lib/admin/styles/admin.css` — new `max-width: 768px` block hiding the table + showing cards; wrap-safety (`min-width:0`, `overflow-wrap:anywhere`, `word-break:break-word`); LocationSelect `width:100%; max-width:100%; min-width:0`; metadata grid 2-col→1-col at ≤560px; pagination centered/wrap

Also: added a `@media (max-width: 768px)` breakpoint, and collapsed the metadata grid to one column on narrow phones.

**Verification:**
- `bun run check` → 0 errors, 0 warnings
- `bun run test` → 54/54 pass
- `bun run build` → success

**Left undone / next:** live browser/devices check at 320/375/390/430/768px (long title reads fully, no body horizontal overflow, last card dropdown opens, desktop table unchanged). No browser tool in-session. Commit only on user request.

## 2026-08-19 — Refine Job Cache UI readability (labels + hide internal ids)

**Status:** implemented; gates green; browser E2E unavailable in-session

**Symptom (user-reported):** Work mode showed `onsite` alone; employment type showed the raw underscore enum `FULL_TIME_PART_TIME_CONTRACTOR_INTERN_VOLUNTEER_PER_DIEM_OTHER`; Source showed the internal UUID `source-180638f6-...`; Original link showed a truncated content hash next to `Original ↗`. Hard to read, esp. on mobile.

**Change scope:** presentation only — API/database fields (`source_key`, `content_hash`, enums) retained; only the UI projection changed.

**Files changed:**
- `frontend/src/lib/admin/job-labels.ts` (new) + `job-labels.spec.ts` (new, 12 tests) — `workModeLabel`, `employmentLabels`, `sourceDisplayLabel`
- `frontend/src/lib/admin/components/JobCacheTable.svelte` — desktop table (Source domain only, Mode badge, Type chips, Original link w/o hash) and mobile card (labeled vertical blocks: Lifecycle / Chế độ làm việc / Loại hình / Nguồn / Cập nhật lần cuối / Original)
- `frontend/src/lib/admin/styles/admin.css` — `.job-mode-badge` (remote/hybrid/onsite colors), `.job-type-badge` chips, `.job-mobile-wide`; removed dead `.job-hash` / `.job-source-cell small`

**Verification:**
- `bun run check` → 0 errors, 0 warnings
- `bun run test` → 66/66 pass
- `bun run build` → success

**Left undone / next:** live browser visual check at 320/375/390/768px/desktop (no source UUID or hash, distinct work mode vs employment, multi employment chips, Original link opens correctly, drop-down works on last row). No browser tool in-session. Commit only on user request.

## 2026-08-19 — Fix Employment Type badge splitting in Job Cache

**Status:** implemented; gates green; browser E2E unavailable in-session

**Symptom (user-reported):** UI split employment enums on `_`, so `FULL_TIME`→ two badges `FULL`+`TIME`, `PART_TIME`→`PART`+`TIME`, `PER_DIEM`→`PER`+`DIEM`. Each enum must be one complete badge.

**Finding:** the previously-shipped `employmentLabels` already returned one badge per full enum (`FULL_TIME`→`['Toàn thời gian']`); the reported split was not reproducible in current code. Nevertheless I hardened the implementation to satisfy the full-enum-list rule and made the invariant structural.

**Files changed (frontend only):**
- `frontend/src/lib/admin/job-labels.ts` — rewrote `employmentLabels` as an explicit full-enum-list tokenizer (`FULL_TIME, PART_TIME, PER_DIEM, CONTRACTOR, INTERN, VOLUNTEER, OTHER`): each full enum matched atomically, never splitting `FULL`/`TIME`/`PART`/`PER`/`DIEM`; multi-enum strings yield one badge per enum.
- `frontend/src/lib/admin/job-labels.spec.ts` — added 2 regression tests (no bare `FULL`/`TIME`/`PART`/`PER`/`DIEM` badge; correct length/order e.g. `FULL_TIME_PART_TIME`→2, `FULL_TIME_PER_DIEM_CONTRACTOR`→Toàn thời gian/Theo ngày/Hợp đồng).
- `frontend/src/lib/admin/styles/admin.css` — `.job-type-badge` added `white-space: nowrap` (already `display:inline-flex; width:max-content`) inside the flex-wrapping `.job-type-badges` container.

**Verification:**
- `bun run check` → 0 errors, 0 warnings
- `bun run test` → 68/68 pass
- `bun run build` → success

**Left undone / next:** live browser visual check (each enum a single unbroken badge, no overflow). No browser tool in-session. Committed only on user request; no commit/push done.

## 2026-08-19 — Fix real Employment Type bug in Job Cache (stray "." badges)

**Status:** implemented; frontend gates green; crawler `cargo check` green; crawler `cargo test` blocked by running `ferris-crawler.exe`.

**Symptom (user-reported):** Viettel job Employment Type showed `[Toàn thời gian] [.] [Bán thời gian] [.] [Hợp đồng] ...` — stray "." rendered as its own badge. Not to be hidden with CSS.

**Root cause (traced raw→UI):**
1. **Crawler** `normalize_employment_type` (`crawler/src/crawl.rs`) split only on `['-',' ','/']`, so dot/comma-separated source enum strings passed through with their punctuation into the stored `employment_type`.
2. **Frontend** `employmentLabels` (`frontend/src/lib/admin/job-labels.ts`) split only on `_` and didn't drop punctuation-only tokens → "." became a badge.
3. **Template/CSS** were clean (no `::before/::after`, no injected separator), so the parser was the only source of the stray badge.

**Fixes (no schema/mock changes):**
- `frontend/src/lib/admin/job-labels.ts` — `employmentLabels` regex-matches the full enum list atomically (accepts `_`, `-`, space intra-enum: `FULL_TIME`==`FULL TIME`) and drops all punctuation separators (`.`, `,`, `;`, `|`, `/`) from gaps; punctuation-only/empty badges become impossible; unknown alnum tokens still kept (uppercased).
- `frontend/src/lib/admin/job-labels.spec.ts` — added separator-form tests + a no-punctuation-badge invariant test.
- `crawler/src/crawl.rs` — `normalize_employment_type` now splits on `[.,;|/' '\t-]` so future crawls store canonical underscore-joined enums; added Rust test `normalize_employment_type_handles_mixed_separators`. Existing DB rows need no migration (frontend parser handles legacy separators).

**Raw `employmentType` actually received:** not captured from a live DB (no local PostgreSQL connection this session); inferred from the crawler path as a dot-separated string such as `FULL_TIME.PART_TIME.CONTRACTOR...` (reproduced exactly via the parser — see `bun run .tmp-probe/emp.mjs`).

**Verification:**
- `bun run check` → 0 errors, 0 warnings
- `bun run test` → 70/70 pass
- `bun run build` → success
- `cargo check` (crawler) → passes
- `cargo test` (crawler) → blocked: running `ferris-crawler.exe` (PID 18084) holds the target exe ("Access denied"). Not a code failure.

**Left undone / next:** stop `ferris-crawler.exe` then `cargo test normalize_employment_type`; live browser check of the Viettel job at desktop + mobile (no "." badges, no overflow). No browser tool in-session. No commit/push done.





## 2026-08-19 — Fix admin-assigned job location lost after crawler restart/re-crawl (data bug)

**Status:** implemented; verified against live PostgreSQL; migration applied locally

**Symptom (user-reported):** Admin-assigned canonical locations in Job Cache reverted to "Chưa gán" (location_id NULL) after the crawler restarted or ran a new cycle.

**Root cause (confirmed by code + live DB):**
- `crawler/src/store.rs` `UPSERT_JOB_CACHE_QUERY` had a blind overwrite `location_id = EXCLUDED.location_id` in the `ON CONFLICT ... DO UPDATE` branch.
- The crawler computes `location_id` via `resolve_location_id()`, which returns `Option<Uuid> = None` whenever the source `location_text` does not match any `location_aliases`/`locations` canonical key (parser anomaly, changed source text, or simply unresolvable text).
- On the next crawl of the same `source_id + source_job_key`, the `DO UPDATE` wrote that NULL over the admin-assigned `location_id`, erasing it. There was no ownership marker distinguishing crawler-auto resolution from admin assignment, so the crawler could not know to preserve it.
- `source_job_key` is stable (it is the unique identity key, never regenerated per crawl), so rows were not duplicated — the overwrite hit the same row. `reconcile_missing_jobs` only touches `missing_healthy_cycles`/`status`, not `location_id`.

**Design — location ownership marker:**
- Added `job_cache.location_assignment_source` (`text NOT NULL DEFAULT 'AUTO'`, `CHECK IN ('AUTO','ADMIN')`) via new migration `000010_location_assignment_source`.
- `AUTO` = crawler-owned resolution; `ADMIN` = operator-assigned. The crawler creates rows as `AUTO` and therefore claims no location ever written by an admin.
- Upsert rule: ADMIN rows keep the admin `location_id`; AUTO rows accept fresh crawler resolution but an unresolved NULL never clobbers an existing non-NULL value; the assignment source is never changed by the crawler.
- Admin `PATCH /api/v1/admin/jobs/:id/location`: non-NULL location → sets `location_id` + `location_assignment_source='ADMIN'`; clearing to NULL → reverts to `'AUTO'` (crawler may re-resolve).

**Files changed:**
- `database/migrations/000010_location_assignment_source.up.sql` / `.down.sql` (new)
- `database/README.md` — migration order/applies
- `database/tests/validate-schema.ps1` — 000010 contract checks
- `crawler/src/store.rs` — upsert preserves ADMIN location and rejects NULL overwrite; made query `pub` internals + tests
- `crawler/tests/location_ownership_contract.rs` (new) — PostgreSQL integration regression contract (governed by real `persist_healthy_observations` path)
- `backend/internal/repository/postgres_locations.go` — admin assignment writes source
- `backend/internal/repository/postgres_jobs.go`, `backend/internal/model/job.go`, `backend/internal/httpapi/admin_job_handler.go` — expose `location_assignment_source` in admin job list

**Regression tests (PHASE 4 cases covered):**
- Rust unit guards on the upsert constant (no blind overwrite; ADMIN + NULL-guard present; reconcile touches no location).
- Rust integration contract (live DB): ADMIN survives unresolved-NULL re-crawl; ADMIN survives a different-location re-crawl; row id stable; AUTO accepts resolution and stays AUTO; unresolved AUTO keeps last resolved location.
- Backend `go test`/`go vet` green; existing locked handler test for the clear-null path still passes.

**Verification (live):**
- `cargo test` (31 lib units + integration contracts), `cargo check`, `cargo fmt --check`
- `go test ./... -count=1`, `go vet ./...`
- `database/tests/validate-schema.ps1` — "Schema contract passed" (incl. 000010)
- Migration `000010` applied to local DB; all 8 existing rows defaulted `AUTO`.
- Both crawler PostgreSQL integration contracts passed against the migrated DB (`--ignored`); no leaked test rows after rollback.

**Data recovery report (PHASE 5):**
- Live DB currently has **8 jobs, all with `location_id = NULL`** (0 have a location). The admin assignments were already destroyed by the pre-fix crawler overwrite.
- `admin_audit_events` contains only `admin_login`/`admin_logout` (no location-assignment records), so **there is no history to reconstruct the lost assignments**. No mock/guessed locations were written.
- Recovery is therefore limited to **preventing future loss** (this fix). Re-assigning locations now will be persisted across subsequent crawls/restarts. A data-audit history column is a potential future improvement but is out of scope.

**Next action:** commit/push only on the user's request. Optionally re-assign the 8 jobs' locations in the Job Cache UI after this fix is deployed and confirm via PostgreSQL they persist across a crawler cycle.

## 2026-08-19 — Dev Client Google Login + User Profile

**Status:** implemented; checked; verified against live PostgreSQL; migration 000011 applied locally

**Symptom/goal:** Client users had no login. Add a full Google OAuth flow (real login, client session, profile display, logout) for the public client, kept fully separate from the admin auth.

**Design decisions (approved):**
- Use the official Google Go libs (`golang.org/x/oauth2` + `google.golang.org/api/idtoken`) for the Authorization Code exchange and OIDC ID-token verification.
- Client logout mirrors the admin CSRF mutation pattern (`X-CSRF-Token` header validated against `client_sessions.csrf_token_hash`, token returned by `GET /me`).
- Identity anchored to the Google `sub` (`client_google_identities.google_sub` UNIQUE); email stored lowercase and never used as the identity key.
- The client domain is fully separate from admin: distinct tables, `ferris_client_session` cookie, `RequireClientSession`/`RequireClientCSRF` middleware, `/api/v1/client/auth` group. A client session never authorizes admin routes.

**Migration `000011_client_auth`** (applied to local DB): `client_users`, `client_google_identities` (UNIQUE google_sub), `client_sessions` (token_hash/csrf_token_hash bytea = 32). No passwords, no access/refresh tokens, no raw OAuth response persisted. Reversible down migration; validate-schema.ps1 checks added.

**Endpoints added (`/api/v1/client/auth`):**
- `GET /google` — one-time HMAC-free random `state` mirrored in a short-lived HttpOnly SameSite=Lax cookie; redirects to Google with `openid email profile`, exact redirect URI from GOOGLE_REDIRECT_URL.
- `GET /google/callback?code&state` — verifies state, exchanges code, verifies ID token (issuer/audience/exp email_verified=true), upserts client user by `sub`, creates session, sets HttpOnly cookie, redirects to client. On error → `/client/login?error=oauth_error`.
- `GET /me` — session-bound; returns `{user, csrf_token}` or 401 `client_unauthorized`. Refreshes CSRF on each call.
- `POST /logout` — CSRF-guarded; revokes server-side and clears cookie.

**Backend files:** `internal/config/config.go` (Google + client cookie/session settings), `internal/model/client_auth.go`, `internal/service/client_auth.go`, `internal/repository/client_auth.go` + `postgres_client_auth.go`, `internal/httpapi/client_auth_handler.go` + `client_auth_middleware.go`, `router.go`, `cmd/api/main.go`, `.env.example` (commented placeholders, no secrets).

**Frontend:** `client-auth-api.ts` (credentials: include, snake_case parsing), `client-auth-store.ts` (user/status/loadCurrentUser/logout), rewritten `ClientHeader.svelte` (Google avatar button + accessible dropdown: Profile / CV của tôi [disabled "Sắp có"] / Đăng xuất; "Đăng nhập Google" when logged out), `/client/login` route, `/client/profile` route, shared auth styles in global.css.

**Verification:**
- Backend: `go test ./... -count=1`, `go vet ./...` all green. New client auth handler/service/integration tests pass.
- Live PostgreSQL: `postgres_client_auth_integration_test` (unique Google identity, session digest storage, admin separation) passed; zero leaked client rows after runs.
- Schema: `database/tests/validate-schema.ps1` → "Schema contract passed".
- Frontend: `bun run check` (0 errors/warnings), `bun run test` (79 pass incl. new client-auth specs), `bun run build` succeeds.

**Security notes:** No `GOOGLE_CLIENT_SECRET` is logged, committed, or sent to the frontend; `backend/.env` is not committed. No raw Google token, subject, or access/refresh token is persisted or exposed. Client sessions never authorize admin endpoints (separate cookie + middleware).

**Not done (per scope):** CV upload/save/parse/scan, DeepSeek, profile editing, "CV của tôi" is present as a disabled menu item only, no fake CV list/user/avatar.

**Next action:** Live manual smoke (start login → Google redirect → callback → `/me` → avatar menu → profile → reload → logout → 401) requires a real Google Test user + the OAuth client; not automated here because no browser tool was available. Commit/push only on user request.

## 2026-08-19 — Client Google login: security review follow-up

An independent security review of the client Google auth found 3 issues; all fixed:

1. **HIGH — identity confusion via email-keyed user row.** `CreateClientUserAndGoogleIdentity` upserted `client_users` on `lower(email)` while anchoring identity on `google_sub`, so two Google accounts that ever shared one email would converge on one user row (one could authenticate into the other's sessions/data). Fixed: the client user is now resolved/created by the Google `sub` only (`findClientUserByGoogleSubQuery` → new user + identity on first login, profile refresh on re-login). Migration `000011` no longer builds the `client_users_email_lower_uidx` index and adds `client_google_identities_user_uidx UNIQUE (client_user_id)`; additionally, `updateClientGoogleIdentityQuery` now stores email lowercase for consistency. Added a regression assertion that two accounts with the same email produce two distinct client users (passed in the live-DB integration test).
2. **MEDIUM — open redirect via backslash in `?redirect=` on the OAuth callback.** The backend no longer trusts any client-supplied `redirect`; the callback always lands on the fixed `/client` path (no open-redirect surface). The frontend never sent `redirect`.
3. **LOW — `GOOGLE_REDIRECT_URL` accepted `http` in production.** `validateGoogleOAuth` now requires `https` when `APP_ENV=production` (plain `http` still allowed for local loopback dev).

Re-verified after fixes: `go build`, `go test ./... -count=1`, `go vet ./...`, `database/tests/validate-schema.ps1`, and the live-DB client-auth integration test all pass with zero leaked rows.

## 2026-08-20 — Client experience, CV history, and Home CMS implementation

**Status:** Tranches 0–4 implemented in the existing dirty working tree. No mock data, commit, push, or reset performed.

**Implemented:**
- Client Google OAuth/session/profile/avatar/logout remains separate from admin authentication.
- Client Home is location-only and sends no active radius field. CV submission requires client authentication, creates an owner-bound scan, and redirects to the dedicated polling route `/client/scans/:scan_id`.
- CV history is real, paginated, owner-scoped, read/delete only, and exposes structured profile fields without raw CV bytes or extracted raw text.
- Admin Users/CV pages read real PostgreSQL APIs; admin delete is CSRF-protected.
- Fixed Home CMS slots 1–4 added in migration `000013_home_sections`: slots 1/3 content-left, slot 2 image-left, slot 4 ordered media strip with a maximum of 12 items. Backend validates plain text, URLs, image metadata, active state, and Cloudinary compensation on DB failure.
- Admin navigation includes `Quản lý section → Trang Home`; client Home reads public active section content. Empty/inactive slots remain absent rather than being filled with fixtures.
- Fixed SSR crash in `ClientLocationSelect.svelte` by guarding browser-only cleanup in `onDestroy`.

**Database/runtime:**
- Migrations `000012_client_cv_history` and `000013_home_sections` applied to local PostgreSQL; schema contract passed.
- Cloudinary Home uploads use folder `ferris/home-sections`; database stores metadata/public ID/hash only.

**Verification:**
- `go test ./... -count=1` — pass.
- `go vet ./...` — pass.
- `bun run check` — 0 errors, 0 warnings.
- `bun run test` — 20 files / 86 tests pass.
- `bun run build` — client and server production builds pass; adapter-auto emitted only its informational environment warning.
- `cargo test` — 31 unit tests + 5 non-DB integration tests pass; 2 PostgreSQL contract tests are intentionally ignored unless run explicitly.
- `cargo test --test location_ownership_contract -- --ignored` and `cargo test --test postgres_contract -- --ignored` — both live PostgreSQL contracts pass after loading `DATABASE_URL` from the local backend environment for the process only.
- `go test -tags integration ./internal/repository -run "TestPostgres" -count=1` — live PostgreSQL client-auth, location-ownership, and Job Link integration contracts pass.
- `cargo check` — pass.
- `database/tests/validate-schema.ps1` — Schema contract passed.
- `git diff --check` — no diff errors; Git only reported CRLF normalization warnings.
- Browser smoke at `http://127.0.0.1:5173`: real promotion/location API data rendered, mobile location listbox opened, and Google login button was visible. A temporary `5174` server was not treated as supported because CORS is allowlisted to the configured `5173` origin.

**Remaining external proof:**
- Use the developer's Google test account for full OAuth callback/reload/logout smoke.
- Run one real authenticated CV scan against configured DeepSeek and verify raw-file cleanup plus owner isolation.
- Upload/update/delete real Home media through Cloudinary and refresh the public Home.
- Run final security/code/test review receipts after those external checks.

## 2026-08-20 — Scan lifecycle and Home empty-state hardening

**Status:** implemented; automated gates rerun green; external live proof remains open

**Changes:**
- Removed the stale radius-positive guard from the Go matching processor. Location-only scans now pass the intentional zero value and scope through canonical `location_id`.
- Changed client scan creation to return `202` immediately with a real processing state. A bounded background worker performs DeepSeek parsing and deterministic matching, marks failures truthfully, removes the temporary CV file, and is cancelled/joined during API shutdown.
- Removed the old static Home hero fallback. When no real promotion or CMS section exists, Home does not invent content or show the retired result placeholder.
- Changed Home media deletion to destroy the Cloudinary asset before deleting its database metadata; provider failure leaves the metadata available for retry.

**Verification:**
- Focused processor/service tests pass, including zero-radius matching and background-start behavior.
- `go test ./... -count=1`, `go vet ./...`, `bun run check`, `bun run test` (20 files / 86 tests), `bun run build`, `cargo test`, `cargo check`, schema validation, and `git diff --check` pass.
- Local `GET /client` returned `200` with the real brand and without the removed static hero copy. Public Home Sections returned four real inactive slots from PostgreSQL.

**Remaining boundary:**
- The worker is an in-process MVP; a hard process kill can still leave a `PARSING` row, so a durable queue/recovery record is production hardening.
- Full Google OAuth, DeepSeek, and Cloudinary live proofs still require the developer's configured external services. No mock result or fabricated proof was added.

## 2026-08-20 — PostgreSQL recovery incident and admin 500 hardening

**Status:** code fix verified; live database recovery and migration verification blocked by Windows service permissions

**Symptom:** crawler logged `interval reload failed; keeping 7200 seconds` because PostgreSQL returned `FATAL: the database system is in recovery mode`; a browser request to `/api/v1/admin/auth/me` returned `500` and the admin screen showed that the workspace was unavailable.

**Root cause evidence:**
- `pg_isready` reports `127.0.0.1:5432 - rejecting connections`.
- PostgreSQL log records an earlier improper shutdown and automatic recovery, then repeated recovery-mode connection failures.
- The API's session repository error was wrapped as `service.ErrAdminAuthStorage`, while the HTTP middleware mapped every non-session error to generic `500`.
- The same database log also recorded `scans.client_user_id does not exist`, so the running database schema must be checked against migrations before the client CV history runtime is considered ready.

**Code fix:**
- `writeAdminAuthError` maps `service.ErrAdminAuthStorage` to HTTP `503`, code `database_unavailable`, without exposing raw database text.
- Admin login and logout use the same mapping as `/me` and CSRF refresh.
- Added regression tests for session lookup, CSRF refresh, login, and logout storage failures; the red test reproduced the old `500`, and the green focused suite passed after the mapping was implemented.

**Operator recovery:**
- The current session cannot stop/restart `postgresql-x64-18` because Windows returned access denied. Do not kill `postgres.exe` or run any down migration.
- Run `Restart-Service postgresql-x64-18` in elevated PowerShell, verify `pg_isready` accepts connections, then inspect `information_schema.columns`/table presence and apply only missing up migrations in order.
- Required post-recovery proof: PostgreSQL readiness, schema query, `/healthz` 200, stale-cookie `/admin/auth/me` 401 (or live valid-cookie 200), crawler interval reload without recovery errors, and focused/full Go verification.
