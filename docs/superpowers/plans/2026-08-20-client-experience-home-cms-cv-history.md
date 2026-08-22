# Client Experience, Home CMS, and CV History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `executing-plans` to implement this plan one tranche at a time. Do not skip tranche gates or combine database, backend, and frontend changes into one unverified batch.

**Status:** Implementation in progress; user approved Option A and the six-tranche direction. Tranches 0–4 are implemented and their automated gates pass. Tranche 5 still has external live-proof items that require the developer's Google test account and Cloudinary/DeepSeek runtime.

**Implementation checkpoint (2026-08-20):** Google client auth, owner-scoped structured CV history, location-only scan flow, dedicated scan-result polling, real admin User/CV APIs, and the fixed four-slot Home CMS are present in the working tree. PostgreSQL migrations `000012` and `000013` are applied locally. A browser smoke at `http://127.0.0.1:5173` verified the real promotion API, real Job Location API, mobile location listbox, and visible Google login button. The temporary `5174` smoke server is not a supported origin because backend CORS intentionally allows the configured `5173` origin only.

**Latest hardening checkpoint (2026-08-20):** The location-only matching path now treats the omitted radius as valid; client scan creation returns `202` and a bounded, shutdown-aware in-process worker performs parsing/matching and temporary-file cleanup. Home no longer renders the retired static hero fallback when promotions are empty. Explicit Home media deletion destroys the Cloudinary asset before removing database metadata and retains metadata when the provider fails. Automated gates and local public HTTP smoke pass; external Google/DeepSeek/Cloudinary proof remains open.

**Goal:** Deliver a reliable Google-authenticated client experience with Sugoi-oniichan branding, location-only CV scanning, structured CV history, a dedicated scan/results page, four admin-managed Home sections, and real admin User/CV management data.

**Architecture:** Keep the current SvelteKit frontend, Go API, PostgreSQL, Cloudinary, DeepSeek parsing, deterministic Go matching, and Rust crawler boundaries. Google authentication owns client identity; every new scan belongs to one authenticated client user. The uploaded CV file is temporary and deleted after extraction; PostgreSQL stores only scan metadata, validated structured profile JSON, and deterministic match results. Home content is a fixed-slot CMS rather than arbitrary HTML.

**Tech stack:** SvelteKit/Svelte 5 + Bun, Go/Gin, PostgreSQL migrations, DeepSeek JSON schema output, Cloudinary image storage, Rust crawler unchanged except for integration verification.

---

## 1. Approved product truths

- **Brand:** `Sugoi-oniichan`. Render `Sugoi` in orange and `oniichan` in blue; use the existing brand logo.
- **Product:** `Go get a job ferris`. It is the product/site name, not the company brand.
- **No mock data:** absent APIs render honest empty/loading/error states only.
- **Location-only discovery:** the client chooses one canonical Job Location. Remove the radius field and radius-dependent matching behavior from the active client contract.
- **CV retention:** “save CV” means saving the structured profile, scan metadata, status, timestamps, and matches. Never retain the raw PDF/DOCX/TXT, extracted raw text, Google tokens, or unnecessary PII.
- **Matching:** Go remains deterministic with weights 35% required skills, 25% role relevance, 15% experience, 15% seniority, and 10% preferred skills/domain.
- **DeepSeek:** use the configured backend secret only; default `deepseek-v4-flash`, fallback `deepseek-v4-pro`, strict schema validation before database writes, and no unnecessary name/email/phone/address/photo in prompts.
- **Home:** promotion carousel + CV upload card + four managed sections. The current result placeholder is removed.
- **Results:** submitting a CV navigates to a dedicated, reload-safe scan route with loading, failure, and completed match states.
- **Client profile:** Google avatar, read-only name/email, CV history, logout. Profile editing is out of scope.
- **Admin/client separation:** client sessions never authorize admin endpoints and admin sessions never authorize client-owned data endpoints.

## 2. UX direction and anti-template constraints

### Brief fingerprint

- **Product type:** Vietnamese CV-to-job matching service.
- **Primary task:** upload one CV, select one canonical location, start a scan, and understand the matched jobs.
- **Visual tone:** editorial, trustworthy, energetic, and human; dark Ferris surface with orange/blue brand color and red action color.
- **Typography:** use a legally distributable, self-hosted client font with a friendly editorial feel. Recommended: `Manrope` for client headings/body with carefully tuned weights. Keep admin typography and density stable unless a shared token must change.
- **Reference use:** borrow hierarchy, alternating information sections, whitespace rhythm, and horizontal media-strip behavior from CV Genius; do not copy proprietary text, artwork, DOM, or CSS.

### Required visual decisions

- Header brand lockup reads `Sugoi-oniichan`; product naming appears separately where useful.
- Client location control uses the admin listbox interaction pattern: black surface, white text, red hover/focus, keyboard navigation, and a portal/fixed menu that cannot clip.
- Google button has explicit white background and dark text in every state; never rely on an undefined fallback token.
- Client Home sections alternate composition:
  1. content left, image right;
  2. image left, content right;
  3. content left, image right;
  4. horizontal auto-scrolling image strip with manual interaction and reduced-motion fallback.
- At mobile widths, every section becomes a one-column flow with intentional image/content order; no horizontal clipping, text shrinking, or hidden overflow workaround.

## 3. Recommended authentication decision

### Option A — require Google login before scanning (**recommended**)

- Every scan has an owner at creation time.
- CV history is deterministic and cannot require a later “claim anonymous scan” mechanism.
- Ownership checks are straightforward for scan polling, history, and admin inspection.
- If a guest presses the scan button, redirect to `/client/login?return_to=/client`; after successful login, return to Home with the selected non-file form state restored where safe. The user must reselect the local file because browsers must not restore file input contents.

### Option B — allow anonymous scan and attach it after login

- Requires a short-lived claim token, replay protection, expiry, ownership transfer rules, and additional security testing.
- Adds complexity without a current product requirement.

### Option C — anonymous scans with no history

- Contradicts the requested per-user CV history.

**Decision:** user approved Option A and authorized implementation. Google login is required before a CV scan is submitted; anonymous scans are not created or later claimed.

## 4. Target API and data contracts

### Client authentication

- Keep:
  - `GET /api/v1/client/auth/google`
  - `GET /api/v1/client/auth/google/callback`
  - `GET /api/v1/client/auth/me`
  - `POST /api/v1/client/auth/logout`
- Local development uses one canonical browser host: `http://127.0.0.1:5173`.
- Backend callback remains the exact configured Google redirect URI on `127.0.0.1:8080`.
- OAuth state cookie must survive the outbound/return flow; state failures return a stable safe error without leaking callback details.

### Client scans and history

- `POST /api/v1/client/scans`
  - Authentication: required client session.
  - Request: multipart fields `cv` and `location_id`; no `radius_km`.
  - Response: `202 { scan_id, status: "processing" }`.
- `GET /api/v1/client/scans/:scan_id`
  - Authentication: required; owner only.
  - Processing response is pollable and reload-safe.
  - Completed response includes structured profile summary and public-safe match cards.
- `GET /api/v1/client/cvs?page=1&page_size=10`
  - Authentication: required; current user only.
  - Returns paginated scan/history metadata and structured profile summary, never raw CV.
- `GET /api/v1/client/cvs/:scan_id`
  - Authentication: required; owner only.
  - Returns one structured profile, scan state, selected location, and saved deterministic matches.

### Admin client users and CVs

- `GET /api/v1/admin/client-users?page=1&page_size=10&query=`
  - Admin authentication required.
  - Returns real users, Google profile display fields, created/last-login timestamps, and CV count.
- `GET /api/v1/admin/client-cvs?page=1&page_size=10&user_id=&role=`
  - Admin authentication required.
  - Returns real structured CV history only.
- `GET /api/v1/admin/client-cvs/:scan_id`
  - Read-only structured profile detail.
- `DELETE /api/v1/admin/client-cvs/:scan_id`
  - Admin authentication + CSRF required.
  - Deletes scan history, structured profile, and match rows transactionally. There is no raw file to delete.

### Home section CMS

- `GET /api/v1/client/home-sections`
  - Public, cacheable response with active fixed sections in display order.
- `GET /api/v1/admin/home-sections`
  - Admin authentication required; includes inactive/empty slots and item metadata.
- `PUT /api/v1/admin/home-sections/:slot`
  - Fixed slots 1–3 only; validates title/body/image/alt text/action fields and layout type.
- `POST /api/v1/admin/home-sections/4/items`
  - Multipart image upload through backend to Cloudinary; database stores metadata/public ID only.
- `PATCH /api/v1/admin/home-sections/4/items/:id`
  - Update order, active state, alt text, or safe target URL.
- `DELETE /api/v1/admin/home-sections/4/items/:id`
  - Remove database record and Cloudinary asset with explicit failure handling.
- No arbitrary HTML. Text fields render escaped; target URLs use an allowlisted `http/https` scheme.

### Database evolution

- Migration `000012_client_cv_history`:
  - add nullable `client_user_id` to existing scans for backward compatibility;
  - index `(client_user_id, created_at DESC)`;
  - enforce ownership for all newly created scans in service/repository contracts;
  - deprecate radius persistence and stop all active reads/writes of `radius_km`;
  - keep legacy anonymous rows inaccessible from user history rather than guessing ownership.
- Migration `000013_home_sections`:
  - fixed Home section records for slots 1–3;
  - ordered section-4 media items;
  - Cloudinary metadata, content hash, active/order fields, timestamps, and update actor;
  - constraints for slot/layout/cardinality and unique ordering.

## 5. Implementation tranches

Each tranche ends with its own checkpoint, tests, and Reasonix/Baron update. Do not start the next tranche while a mandatory gate is red.

### Tranche 0 — repair and prove Google OAuth locally

**Files likely involved**

- `frontend/vite.config.ts`
- `frontend/src/lib/shared/api/api-url.ts`
- `frontend/src/routes/client/login/+page.svelte`
- `frontend/src/lib/shared/styles/global.css`
- `backend/internal/config/config.go`
- `backend/internal/httpapi/client_auth_handler.go`
- `backend/internal/httpapi/client_auth_handler_test.go`
- `backend/.env.example`

**Steps**

- [ ] Add a failing regression test for the actual host mismatch: frontend origin/cookie flow must consistently use `127.0.0.1`, not a mixture of `localhost`, `127.0.0.1`, and IPv6 `::1`.
- [ ] Add a failing callback test proving a missing OAuth state cookie maps to `state_error`, clears temporary state, and redirects to the configured client login origin.
- [ ] Bind Vite explicitly to `127.0.0.1` and make `CLIENT_REDIRECT_ORIGIN=http://127.0.0.1:5173` an explicit local configuration contract.
- [ ] Fix the Google button with explicit foreground/background/hover/focus/disabled colors and accessible text.
- [ ] Add safe diagnostics that identify the OAuth stage without logging code, state, tokens, client secret, or user PII.
- [ ] Run Go auth tests, frontend auth tests, typecheck, and build.
- [ ] Live smoke in a real browser: login → Google consent → callback → `/me` → reload → avatar → profile → logout → `/me` returns 401.

**Checkpoint:** OAuth live smoke passes before any CV ownership work begins.

### Tranche 1 — client design foundation and location-only Home flow

**Files likely involved**

- `frontend/src/lib/client/components/ClientHeader.svelte`
- `frontend/src/lib/client/components/CvUploadForm.svelte`
- `frontend/src/lib/client/components/LocationSelect.svelte` (new shared client component or a shared extraction)
- `frontend/src/lib/admin/components/LocationSelect.svelte`
- `frontend/src/lib/admin/location-select-position.ts`
- `frontend/src/routes/client/+page.svelte`
- `frontend/src/routes/client/login/+page.svelte`
- `frontend/src/routes/client/profile/+page.svelte`
- `frontend/src/lib/shared/styles/global.css`
- `frontend/static/brand/sugoi-oniichan-logo.png`
- client component/unit specs

**Steps**

- [ ] Define client-only typography tokens and self-host the chosen licensed font; do not destabilize admin tables.
- [ ] Add a reusable brand lockup component and tests for the exact brand/product distinction.
- [ ] Replace the radius validation/control with one canonical Job Location listbox.
- [ ] Reuse the proven portal positioning and keyboard behavior from the admin location listbox, with client visual tokens.
- [ ] Remove inline result/loading/empty-result markup from Home.
- [ ] Keep Home’s top composition constrained: header, promotion carousel, upload card, then CMS section mount points.
- [ ] Build responsive states at desktop, tablet, 390px, 375px, and 320px; prove the listbox cannot clip or remove page scrolling.
- [ ] Add reduced-motion treatment for carousel/animated elements.

**Checkpoint:** Home has no radius and no result placeholder; brand and Google button are visually correct; desktop/mobile interaction proof captured.

### Tranche 2 — authenticated scan ownership and structured CV history

**Files likely involved**

- `database/migrations/000012_client_cv_history.up.sql`
- `database/migrations/000012_client_cv_history.down.sql`
- `database/tests/validate-schema.ps1`
- `database/README.md`
- `backend/internal/model/scan.go`
- `backend/internal/repository/scans.go`
- `backend/internal/repository/postgres_scans.go`
- `backend/internal/repository/postgres_scans_test.go`
- `backend/internal/service/scan_service.go`
- `backend/internal/service/scan_service_test.go`
- `backend/internal/httpapi/handler.go`
- `backend/internal/httpapi/handler_test.go`
- `backend/internal/httpapi/router.go`
- new client CV history handler/repository tests

**Steps**

- [ ] Write migration contract tests for scan ownership and indexes before the migration.
- [ ] Add repository tests proving user A cannot read user B’s scan/history.
- [ ] Add handler tests proving unauthenticated create/get/list returns 401 and foreign ownership is not disclosed.
- [ ] Remove `radius_km` from the active HTTP/service/repository contract and test exact canonical `location_id` filtering.
- [ ] Do not include all remote jobs as an implicit bypass; a result must satisfy the selected canonical location contract.
- [ ] Bind `client_user_id` from authenticated server context, never from a client request field.
- [ ] Preserve the existing `defer`/cleanup guarantee that deletes temporary CV files on success, validation error, parser error, database error, and cancellation.
- [ ] Persist validated structured profile JSON and deterministic matches in one coherent transaction boundary.
- [ ] Add paginated, owner-scoped CV history/detail APIs.
- [ ] Verify DeepSeek schema validation and fallback behavior without storing provider raw responses.

**Checkpoint:** live PostgreSQL test shows one authenticated user creates a scan, raw file disappears, structured history persists, and another user cannot access it.

### Tranche 3 — dedicated scan/results route, profile history, and real admin data

**Files likely involved**

- `frontend/src/routes/client/scans/[scan_id]/+page.svelte` (new)
- `frontend/src/routes/client/profile/+page.svelte`
- `frontend/src/lib/client/components/ScanProgress.svelte`
- `frontend/src/lib/client/components/MatchResults.svelte`
- `frontend/src/lib/client/components/MatchJobCard.svelte`
- `frontend/src/lib/client/api/client-api.ts`
- `frontend/src/lib/client/stores/scan-store.ts`
- `frontend/src/routes/admin/users/+page.svelte`
- `frontend/src/routes/admin/cvs/+page.svelte`
- new admin user/CV API modules and components
- corresponding frontend specs

**Steps**

- [ ] Submit authenticated scan, receive `scan_id`, and navigate immediately to `/client/scans/:scan_id`.
- [ ] Poll with bounded backoff; stop on completed/failed, abort on navigation, and resume correctly after browser reload.
- [ ] Show explicit parsing/matching/loading state without inventing progress percentages.
- [ ] Render only public-safe match fields: CV Match %, title, company, canonical location, employment type, work mode, salary if provided, up to three skill tags, and original URL.
- [ ] Add profile CV history with honest empty/loading/error states and structured detail; no raw download link.
- [ ] Keep name/email read-only and source avatar from the verified Google profile URL with a safe fallback.
- [ ] Wire admin Users/CVs pages to real paginated APIs; retain existing filters only where backed by the contract.
- [ ] Add confirmed delete flow for admin CV history with CSRF, loading, success, and rollback/error states.

**Checkpoint:** one real Google user can scan, reload the result route, revisit the same structured CV in Profile, and the admin can inspect that real user/CV row.

### Tranche 4 — Home section CMS and four client sections

**Files likely involved**

- `database/migrations/000013_home_sections.up.sql`
- `database/migrations/000013_home_sections.down.sql`
- `database/tests/validate-schema.ps1`
- new backend Home section model/repository/service/handlers/tests
- `backend/internal/httpapi/router.go`
- `backend/cmd/api/main.go`
- `frontend/src/lib/admin/components/AdminShell.svelte`
- `frontend/src/routes/admin/sections/home/+page.svelte` (new)
- `frontend/src/lib/client/components/HomeContentSection.svelte` (new)
- `frontend/src/lib/client/components/HomeMediaMarquee.svelte` (new)
- `frontend/src/routes/client/+page.svelte`
- admin/client API modules and specs

**Steps**

- [ ] Add failing schema/repository tests for fixed slots, ordering, active state, and item caps.
- [ ] Implement public read and admin mutation contracts with strict plain-text/URL/file validation.
- [ ] Route uploads through the backend to Cloudinary; store metadata/public ID/hash only.
- [ ] Add `Quản lý section` to the admin sidebar with a `Trang Home` child route.
- [ ] Build editor panels for slots 1–3 and an ordered image-list manager for slot 4; never populate fake cards.
- [ ] Render sections 1/3 content-left, section 2 image-left, and section 4 as a seamless horizontal strip.
- [ ] Pause marquee on hover/focus, expose manual navigation where appropriate, and disable continuous motion for `prefers-reduced-motion`.
- [ ] Add responsive and empty-state tests. Empty/inactive slots do not create unexplained blank space on Home.

**Checkpoint:** changing real section content/image in admin changes the public Home after API refresh; no code edit or frontend redeploy is needed.

### Tranche 5 — integrated verification, security review, and handoff

**Required automated gates**

- [ ] Backend: `go test ./... -count=1`.
- [ ] Backend: `go vet ./...`.
- [ ] Database: `database/tests/validate-schema.ps1` against a clean migration path and the current local PostgreSQL instance.
- [ ] Frontend: `bun run check`.
- [ ] Frontend: `bun run test`.
- [ ] Frontend: `bun run build`.
- [ ] Crawler regression: `cargo test` and `cargo check` to prove location/job-cache contracts were not broken.
- [ ] `git diff --check`.

**Required live proof**

- [ ] Google OAuth using the configured test user and exact redirect URI.
- [ ] PostgreSQL ownership and CV history persistence across backend restart.
- [ ] DeepSeek real parse with PII redaction and validated structured output.
- [ ] Raw CV file absence after successful and failed scans.
- [ ] Cloudinary Home section upload, replacement, and deletion.
- [ ] Desktop screenshots for Home, login, profile/history, scan loading, scan results, admin Home editor, admin Users, and admin CVs.
- [ ] Mobile screenshots at 390px and 320px for the same critical client flows.
- [ ] Keyboard-only navigation and visible focus for header menu, Google button, location listbox, upload form, and section controls.

**Review gates**

- [ ] Code review for API ownership, migration safety, component boundaries, and error handling.
- [ ] Security review for OAuth state/session/CSRF, IDOR, upload limits/content validation, PII/secret logging, XSS, URL validation, and Cloudinary deletion semantics.
- [ ] Test review for meaningful negative paths and live proof coverage.

## 6. Failure, rollback, and interruption rules

- Never rewrite or discard the existing dirty working tree. Every tranche touches only its listed scope and records overlapping files before edits.
- Migrations are additive and reversible. Do not assign old anonymous scans to a guessed client user.
- If OAuth live smoke fails, stop before scan ownership work and preserve callback/error evidence without secrets.
- If DeepSeek is unavailable, keep the scan failed/pending state truthful; do not fabricate a structured profile or jobs.
- If Cloudinary upload succeeds but the database transaction fails, attempt compensating asset deletion and log only the public ID/error class.
- If database write succeeds but Cloudinary deletion fails, retain a retryable cleanup record rather than silently losing ownership metadata.
- After every tranche, update:
  - `docs/reasonix/CURRENT.md` with completed/blocked state and the next exact action;
  - `docs/reasonix/WORKLOG.md` with files, decisions, and proof;
  - Baron plan progress and continuity checkpoint.
- If power/network/terminal interruption occurs, the next agent must read Baron continuity and this plan before running or editing anything.

## 7. Definition of done

- Google login works in a real browser and survives reload; logout revokes the server session.
- Top-right authenticated control is a round avatar with accessible dropdown; Profile and CV history show real owner-scoped data.
- Client uses one canonical location and no radius UI/request/matching branch.
- Every new scan is authenticated and owner-scoped; raw CV files and extracted text are not retained.
- A dedicated route shows honest loading/failure/completed states and public-safe deterministic job matches.
- Admin Users and CV pages show real database data with protected detail/delete behavior.
- Admin can manage the four Home sections; the client renders real API content responsively.
- Brand/product names and client typography are consistent across Home, login, profile, and scan routes.
- All automated gates, live integration smoke tests, security review, and desktop/mobile visual proofs pass.
- Reasonix and Baron continuity records point to the final proof and no unresolved blocker is hidden.

## 8. Approval gate

Approval received on 2026-08-20. The implementation follows Option A, the six-tranche sequence, and the structured-profile-only retention rule. Remaining work is limited to external live proof/review and must not introduce mock data or weaken the ownership/privacy boundaries.
