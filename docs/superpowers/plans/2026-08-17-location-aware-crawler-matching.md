# Location-Aware Crawler And Matching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a user-supplied Job Link produce structured jobs that can be resolved to canonical Vietnamese locations, then let a client choose a canonical location and receive real CV-matched active jobs from that location.

**Architecture:** Keep the existing PostgreSQL schema as the single source of truth. Add a canonical location resolver shared by crawler persistence and client scan validation, retain the source location text for audit, and expose a small additive client/admin API surface. The Rust worker remains generic and allowlist-scoped; it does not gain site-specific adapters. The Go service owns scan input validation, candidate filtering, deterministic scoring, and public response projection.

**Tech Stack:** PostgreSQL migrations, Go/Gin/pgx, Rust/Tokio/Spider/tokio-postgres, SvelteKit/Svelte 5, Bun/Vitest, existing project CSS tokens.

## Global Constraints

- First market is Vietnam; distance is kilometers; source currency is preserved without conversion.
- Generic source-controlled crawler only; no `fpt.rs`, unrestricted web discovery, hidden endpoint widening, mock jobs, or fabricated matches.
- Only `ACTIVE` approved Job Links are crawled; source/parser failures never count as missing jobs.
- A missing job reaches `VERIFYING` after one healthy missing cycle and `CLOSED` after two healthy missing cycles.
- Raw CV files are temporary and deleted after processing; only validated structured profile data may be persisted.
- Full raw job descriptions are not persisted; Job Cache stores structured fields, metadata, and content hashes.
- CV Match % is deterministic: required skills 35%, role relevance 25%, experience 15%, seniority 15%, preferred skills/domain 10%.
- Remote jobs are location-independent; Onsite and Hybrid jobs use canonical location and kilometer distance where coordinates exist.
- Existing client scan response shape remains readable; new required location identity is added with a migration and explicit client update.
- Frontend contains no invented job/location data; empty, unresolved, loading, and unavailable states remain honest.
- Code comments are only added when they explain an invariant or non-obvious lifecycle boundary.

---

### Task 1: Lock the database and interface contract

**Files:**
- Create: `database/migrations/000006_location_resolution_and_scan_location.up.sql`
- Create: `database/migrations/000006_location_resolution_and_scan_location.down.sql`
- Modify: `database/tests/validate-schema.ps1`
- Modify: `database/README.md`
- Create: `backend/internal/model/location.go`
- Modify: `backend/internal/model/scan.go`
- Modify: `backend/internal/model/job.go`

**Interfaces:**
- Produces canonical location records with a stable normalized key, optional coordinates, and alias rows.
- Produces a nullable `scans.location_id` foreign key for existing scans and requires a valid active location for new client scans at the service boundary.
- Preserves `job_cache.location_text` and `job_cache.location_id`; unresolved jobs remain queryable through `location_id IS NULL`.

- [ ] **Step 1: Write failing schema assertions.**

Add assertions to `database/tests/validate-schema.ps1` for:

```powershell
Assert-Contains -Text $locationUp -Pattern '(?im)canonical_key\s+text\s+NOT NULL' -Message 'Locations need a stable canonical key.'
Assert-Contains -Text $locationUp -Pattern '(?im)latitude\s+numeric' -Message 'Canonical locations need latitude for kilometer filtering.'
Assert-Contains -Text $locationUp -Pattern '(?im)longitude\s+numeric' -Message 'Canonical locations need longitude for kilometer filtering.'
Assert-Contains -Text $locationUp -Pattern '(?im)CREATE TABLE public.location_aliases' -Message 'Location aliases must be persisted.'
Assert-Contains -Text $locationUp -Pattern '(?im)ALTER TABLE public.scans.*ADD COLUMN location_id uuid' -Message 'Scans need canonical location identity.'
Assert-Contains -Text $locationUp -Pattern '(?im)scans_location_fk' -Message 'Scans must reference canonical locations.'
```

- [ ] **Step 2: Run the schema test and confirm the new assertions fail.**

Run from the repository root:

```powershell
powershell -ExecutionPolicy Bypass -File database/tests/validate-schema.ps1
```

Expected: FAIL because migration `000006` does not exist yet.

- [ ] **Step 3: Add the migration.**

The migration must:

```sql
ALTER TABLE public.locations
    ADD COLUMN canonical_key text,
    ADD COLUMN latitude numeric(9, 6),
    ADD COLUMN longitude numeric(9, 6);

UPDATE public.locations
SET canonical_key = lower(regexp_replace(display_name, '[^a-zA-Z0-9]+', '-', 'g'))
WHERE canonical_key IS NULL;

ALTER TABLE public.locations
    ALTER COLUMN canonical_key SET NOT NULL;

CREATE UNIQUE INDEX locations_canonical_key_uidx
    ON public.locations (canonical_key);

CREATE TABLE public.location_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    location_id uuid NOT NULL REFERENCES public.locations(id) ON DELETE CASCADE,
    normalized_text text NOT NULL,
    alias_text text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT location_aliases_normalized_not_blank CHECK (btrim(normalized_text) <> ''),
    CONSTRAINT location_aliases_alias_not_blank CHECK (btrim(alias_text) <> '')
);

CREATE UNIQUE INDEX location_aliases_normalized_uidx
    ON public.location_aliases (normalized_text);

ALTER TABLE public.scans
    ADD COLUMN location_id uuid REFERENCES public.locations(id) ON DELETE RESTRICT;

CREATE INDEX scans_location_idx ON public.scans(location_id, created_at DESC);
```

The down migration must drop the scan index/foreign key, alias table, location indexes/columns, and restore no data by guessing.

- [ ] **Step 4: Add model fields without changing public JSON yet.**

Add `CanonicalKey string`, `Latitude *float64`, and `Longitude *float64` to the admin location model. Add `LocationID *uuid.UUID` to `model.Scan`. Keep `Location string` as the audit/display snapshot.

- [ ] **Step 5: Run the schema test and Go compile tests.**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File database/tests/validate-schema.ps1
Push-Location backend; go test ./internal/model ./internal/repository ./internal/service; Pop-Location
```

Expected: PASS.

---

### Task 2: Implement deterministic location normalization and resolution

**Files:**
- Create: `backend/internal/location/normalize.go`
- Create: `backend/internal/location/normalize_test.go`
- Create: `crawler/src/location.rs`
- Modify: `crawler/src/lib.rs`
- Modify: `crawler/Cargo.toml` only if a dependency is required by the existing lockfile-compatible implementation

**Interfaces:**
- Go: `func Normalize(string) string` returns a lowercase, accent-insensitive, punctuation-normalized key.
- Rust: `pub fn normalize_location_text(&str) -> String` returns the same key format for crawler-side tests and persistence lookup.
- The first implementation uses deterministic normalization and database aliases; it does not call an LLM or guess an unknown city.

- [ ] **Step 1: Write failing Go normalization tests.**

Test these exact behaviors:

```go
func TestNormalizeLocation(t *testing.T) {
    cases := map[string]string{
        "TP.HCM": "tp hcm",
        "Ho Chi Minh City": "ho chi minh city",
        "Q.1, TP.HCM": "q 1 tp hcm",
        "  Hà Nội  ": "ha noi",
    }
    for input, want := range cases {
        if got := Normalize(input); got != want { t.Fatalf("Normalize(%q) = %q, want %q", input, got, want) }
    }
}
```

- [ ] **Step 2: Run the Go normalization test and confirm it fails.**

Run:

```powershell
Push-Location backend; go test ./internal/location -run TestNormalizeLocation -count=1; Pop-Location
```

Expected: FAIL because the package/function does not exist.

- [ ] **Step 3: Implement the smallest shared normalization rules.**

Use Unicode decomposition/removal of combining marks, lowercase, replacement of punctuation with spaces, whitespace collapsing, and bounded output. Do not add a hard-coded city mapping to the normalizer itself; aliases belong to the database.

- [ ] **Step 4: Add Rust normalization tests before Rust resolver code.**

Test that `Hồ Chí Minh`, `Ho Chi Minh City`, and punctuation variants produce stable keys, and that empty/oversized input is rejected by the resolver boundary.

- [ ] **Step 5: Implement the Rust normalization helper and export it.**

Keep crawler normalization behavior equivalent to the Go contract. Add a test-only table that compares expected keys for Vietnamese variants.

- [ ] **Step 6: Run both focused tests.**

Run:

```powershell
Push-Location backend; go test ./internal/location -count=1; Pop-Location
Push-Location crawler; cargo test location --lib; Pop-Location
```

Expected: PASS.

---

### Task 3: Make crawler persistence resolve canonical locations

**Files:**
- Modify: `crawler/src/crawl.rs`
- Modify: `crawler/src/store.rs`
- Modify: `crawler/src/main.rs`
- Create: `crawler/tests/location_persistence.rs` if the existing integration test harness can run against PostgreSQL
- Modify: `crawler/tests/postgres_contract.rs`
- Modify: `crawler/README.md`

**Interfaces:**
- `StructuredJobObservation` keeps `location_text` and gains no raw-content field.
- Persistence resolves `location_text` by querying `location_aliases` and `locations.canonical_key` inside the same transaction as the Job Cache upsert.
- A missing alias leaves `job_cache.location_id = NULL` and does not make a healthy crawl authoritative failure.
- The transaction preserves current source snapshot locking, hash reconciliation, and healthy-cycle missing rules.

- [ ] **Step 1: Add failing crawler observation tests.**

Assert that a structured observation carries the source location text unchanged and that a location resolver result distinguishes:

```rust
Resolved { location_id: Uuid }
Unresolved
```

Assert that an unresolved location is not converted into a fabricated UUID and does not reject an otherwise valid job observation.

- [ ] **Step 2: Run the focused Rust tests and confirm failure.**

Run:

```powershell
Push-Location crawler; cargo test location --lib; Pop-Location
```

Expected: FAIL because the resolver/result types do not exist.

- [ ] **Step 3: Add the resolver query to the persistence transaction.**

Resolve in this order:

```sql
SELECT location_id
FROM public.location_aliases
WHERE normalized_text = $1
UNION ALL
SELECT id
FROM public.locations
WHERE canonical_key = $1
LIMIT 1
```

The query must be parameterized. The raw source location is never logged. The upsert must write `location_id` and update it on every authoritative observation so a corrected alias can move a job to the canonical location.

- [ ] **Step 4: Add a PostgreSQL contract test.**

Using real PostgreSQL and transaction rollback, create a location and alias, persist one observation with `location_text = 'TP.HCM'`, then assert:

```sql
SELECT location_id, location_text
FROM public.job_cache
WHERE source_id = $1 AND source_job_key = $2;
```

Expected: the UUID is the canonical location and `location_text` remains `TP.HCM`. Repeat with an unknown location and assert `location_id IS NULL`.

- [ ] **Step 5: Update crawler startup output and documentation.**

Report only safe bounded counts such as `resolved_locations` and `unresolved_locations`; never print credentials, full CV/JD content, or raw location payloads. Document that unresolved location is a normal reviewable outcome, not a missing-job cycle.

- [ ] **Step 6: Run crawler tests.**

Run:

```powershell
Push-Location crawler; cargo fmt --all -- --check; cargo test; cargo check; Pop-Location
```

Expected: PASS. Run the ignored PostgreSQL contract only when the local database connection is available.

---

### Task 4: Add backend location-aware scan and admin Job Cache filtering

**Files:**
- Modify: `backend/internal/repository/locations.go`
- Modify: `backend/internal/repository/postgres_locations.go`
- Modify: `backend/internal/repository/jobs.go`
- Modify: `backend/internal/repository/postgres_jobs.go`
- Modify: `backend/internal/service/location.go`
- Modify: `backend/internal/service/scan_service.go`
- Modify: `backend/internal/model/scan.go`
- Modify: `backend/internal/httpapi/handler.go`
- Modify: `backend/internal/httpapi/admin_job_handler.go`
- Modify: `backend/internal/httpapi/admin_location_handler.go`
- Modify: `backend/internal/httpapi/router.go`
- Create/modify: focused Go tests beside each changed package

**Interfaces:**
- Add public `GET /api/v1/client/locations` returning active canonical locations with stable IDs and optional coordinates.
- Extend client scan intake with `location_id`; accept old `location` only during the compatibility window, resolve it to an active canonical location, and persist both ID and text snapshot.
- Add `location_id` and `unresolved` filters to `GET /api/v1/admin/jobs` with bounded page/page_size and stable `updated_at DESC, id DESC` ordering.
- Keep admin `PATCH /api/v1/admin/jobs/:id/location` for exception correction.
- Public routes do not expose raw JD, source internals, or location aliases.

- [ ] **Step 1: Write failing repository/handler contract tests.**

Cover:

```text
GET /api/v1/client/locations returns only active canonical locations.
POST /api/v1/client/scans rejects an inactive/unknown location_id.
POST /api/v1/client/scans accepts the compatibility location text only when it resolves uniquely.
GET /api/v1/admin/jobs?location_id=... adds the SQL predicate and preserves pagination.
GET /api/v1/admin/jobs?unresolved=true returns only NULL location_id rows.
```

The tests must assert response JSON and HTTP status, not only SQL string presence.

- [ ] **Step 2: Run the focused Go tests and confirm failure.**

Run:

```powershell
Push-Location backend; go test ./internal/httpapi ./internal/service ./internal/repository -run 'Location|Job|Scan' -count=1; Pop-Location
```

Expected: FAIL because the new routes/fields/filter contract is not present.

- [ ] **Step 3: Implement canonical location repository reads.**

Return active locations ordered by `display_name`, with `job_count` computed from `job_cache.location_id`. Keep list bounds explicit even though the current location list is small.

- [ ] **Step 4: Implement scan location validation and compatibility resolution.**

The service owns the invariant that a new scan uses an active canonical location. It must reject a caller-supplied arbitrary UUID and must not trust a client-provided `location_id` without a database lookup. For the compatibility field, normalize and resolve server-side; do not silently accept an unresolved text location.

- [ ] **Step 5: Implement admin Job Cache filter query and response.**

Use optional parameterized predicates, retain source location and canonical display location in the response, and keep raw description absent. Add a stable total count query with the same filters.

- [ ] **Step 6: Mount the routes and document them.**

Mount the client locations route outside admin auth. Keep admin jobs/locations behind the existing session and CSRF rules. Update `backend/README.md` with request/response examples and explicit compatibility behavior.

- [ ] **Step 7: Run focused Go tests and the full backend test suite.**

Run:

```powershell
Push-Location backend; go test ./internal/httpapi ./internal/service ./internal/repository -count=1; go test ./... -count=1; go vet ./...; Pop-Location
```

Expected: PASS.

---

### Task 5: Implement the real CV structured-profile and deterministic matching seam

**Files:**
- Create: `backend/internal/processor/profile.go`
- Create: `backend/internal/processor/profile_test.go`
- Create: `backend/internal/processor/deepseek.go`
- Create: `backend/internal/processor/deepseek_test.go`
- Create: `backend/internal/matching/matcher.go`
- Create: `backend/internal/matching/matcher_test.go`
- Modify: `backend/internal/processor/processor.go`
- Modify: `backend/internal/service/scan_service.go`
- Modify: `backend/internal/repository/postgres_scans.go`
- Modify: `backend/internal/repository/scans.go`
- Modify: `backend/internal/cmd/api/main.go`
- Modify: `backend/internal/config/config.go`

**Interfaces:**
- DeepSeek defaults to `deepseek-v4-flash`, falls back to `deepseek-v4-pro`, and receives only necessary non-PII CV text.
- JSON is decoded and schema-validated before any profile is stored.
- The processor persists only `structured_profiles`, changes the scan to `MATCHING`, filters active jobs by location/radius/work mode, computes deterministic scores, writes `scan_matches`, and completes the scan transactionally.
- Provider errors produce a bounded `FAILED` scan with a machine-readable error code; they never create a fabricated profile or match.

- [ ] **Step 1: Write failing matcher tests.**

Use a real `StructuredProfile` and `JobCandidate` value, not a mock matcher, and assert exact component totals:

```go
score := Match(profile, job)
if score.RequiredSkillsPoints != 35 { t.Fatal(...) }
if score.RoleRelevancePoints != 25 { t.Fatal(...) }
if score.ExperiencePoints != 15 { t.Fatal(...) }
if score.SeniorityPoints != 15 { t.Fatal(...) }
if score.PreferredSkillsDomainPoints != 10 { t.Fatal(...) }
if score.MatchPercent != 100 { t.Fatal(...) }
```

Add a negative case with no required skills, incompatible role, insufficient experience, and a different seniority.

- [ ] **Step 2: Run matcher tests and confirm failure.**

Run:

```powershell
Push-Location backend; go test ./internal/matching -count=1; Pop-Location
```

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement the pure deterministic matcher.**

Normalize case/spacing for comparisons, keep the five weights as named constants, cap score components at their weights, and make the total equal the sum stored in `scan_matches`.

- [ ] **Step 4: Write failing JSON schema validation tests.**

Test acceptance of the exact fields `roles`, `skills`, `years_of_experience`, `seniority`, `domains`, `education`, and `certifications`; reject unknown fields, wrong types, invalid numeric ranges, missing required fields, and PII fields such as `name`, `email`, `phone`, `address`, and `photo`.

- [ ] **Step 5: Implement provider client and fallback.**

Add bounded HTTP timeouts, response-size limits, JSON-only parsing, model fallback only for provider availability failures, and redacted structured logs containing model and error category but not prompt/CV content. Make the endpoint/base URL configurable for tests and deployment.

- [ ] **Step 6: Write failing scan integration tests.**

Assert that a temporary CV file is removed after success and failure, a completed scan contains only active jobs from the selected location, remote jobs have no distance, and an unavailable provider leaves the scan failed with no match rows.

- [ ] **Step 7: Implement the processor and wire it into `cmd/api`.**

Keep the existing `UnavailableProcessor` available only when the real provider configuration is absent; do not silently inject it when a configured provider can be used. Keep the processor interface testable with an HTTP test server and database repository interfaces.

- [ ] **Step 8: Run processor/matcher/backend tests.**

Run:

```powershell
Push-Location backend; go test ./internal/matching ./internal/processor ./internal/service ./internal/repository ./internal/httpapi -count=1; go test ./... -count=1; go vet ./...; Pop-Location
```

Expected: PASS. Live DeepSeek proof remains dependent on a configured API key and must be reported separately from local contract proof.

---

### Task 6: Connect the admin Location and Job Cache interfaces

**Files:**
- Modify: `frontend/src/lib/admin/api/admin-api.ts`
- Modify: `frontend/src/lib/admin/api/admin-api.spec.ts`
- Modify: `frontend/src/routes/admin/locations/+page.svelte`
- Modify: `frontend/src/routes/admin/jobs/+page.svelte`
- Modify: `frontend/src/lib/admin/components/JobCacheTable.svelte`
- Modify: `frontend/src/lib/shared/styles.css`

**Interfaces:**
- `getAdminJobs(page, pageSize, filters)` accepts optional `locationId` and `unresolved` query parameters.
- Location rows navigate to `/admin/jobs?location_id=<id>`.
- Job Cache reads the query parameter on mount and keeps it when paging.
- A visible `Chưa phân loại` filter handles jobs with `location_id = NULL`.

- [ ] **Step 1: Write failing frontend API tests.**

Assert that `getAdminJobs(1, 20, { locationId: 'loc-1' })` calls `/admin/jobs?page=1&page_size=20&location_id=loc-1`, and unresolved calls `unresolved=true`. Assert that old calls without filters keep their existing URL.

- [ ] **Step 2: Run the focused test and confirm failure.**

Run:

```powershell
Push-Location frontend; bun run test -- admin-api.spec.ts; Pop-Location
```

Expected: FAIL because the function has no filter argument.

- [ ] **Step 3: Implement API filters and typed location fields.**

Keep response parsing strict and preserve empty lists as real empty lists. Do not add local fallback rows.

- [ ] **Step 4: Add the Location-to-Job Cache navigation.**

Use a normal anchor/button with an accessible label such as `Xem job tại Hồ Chí Minh`; do not add explanatory copy above the title. Preserve current admin visual tokens and compact operational density.

- [ ] **Step 5: Add Job Cache filter state.**

Read `location_id` and `unresolved` from `URLSearchParams`, load real locations, show the selected filter, and reload the API page when the filter changes. Keep pagination stable.

- [ ] **Step 6: Run frontend tests/check/build.**

Run:

```powershell
Push-Location frontend; bun run test; bun run check; bun run build; Pop-Location
```

Expected: PASS with no Svelte errors or warnings.

---

### Task 7: Connect the client location selector and scan payload

**Files:**
- Modify: `frontend/src/lib/shared/types/client.ts`
- Modify: `frontend/src/lib/shared/types/job.ts` only if the response contract gains a canonical location ID
- Modify: `frontend/src/lib/client/api/client-api.ts`
- Modify: `frontend/src/lib/client/api/client-api.spec.ts`
- Modify: `frontend/src/lib/client/stores/scan-store.ts`
- Modify: `frontend/src/lib/client/validation/scan-form.ts`
- Modify: `frontend/src/lib/client/components/CvUploadForm.svelte`
- Modify: `frontend/src/routes/client/+page.svelte`

**Interfaces:**
- Client loads `GET /api/v1/client/locations` into a real select.
- `ClientScanInput` includes `locationId` and keeps `location` only as a display/snapshot compatibility field until the backend migration is complete.
- Empty API location data produces a disabled selector and a recoverable error, never a fake city list.
- Submit, polling, error, retry, empty result, and disabled states remain functional.

- [ ] **Step 1: Write failing client API and validation tests.**

Assert location list parsing rejects malformed IDs/names, scan payload includes `location_id`, and a missing selected location is invalid even when the display string is non-empty.

- [ ] **Step 2: Run focused tests and confirm failure.**

Run:

```powershell
Push-Location frontend; bun run test -- client-api.spec.ts scan-form.spec.ts; Pop-Location
```

Expected: FAIL because the client contract still posts only free-text location.

- [ ] **Step 3: Implement real location loading and selection.**

Use the API response as the only option source. Preserve the current radius control and CV validation. Keep labels and controls concise; do not reintroduce the removed promotional copy fields.

- [ ] **Step 4: Update scan store and submit flow.**

Reset location-related errors when the selected ID changes, send the UUID, and preserve the existing polling behavior. Do not claim completion when the backend returns `failed`.

- [ ] **Step 5: Verify the client at narrow and wide widths.**

Check the location select, CV dropzone, radius control, submit button, loading/empty/error states, long Vietnamese location names, keyboard focus, and reduced-motion behavior at mobile and desktop widths.

- [ ] **Step 6: Run all frontend quality checks.**

Run:

```powershell
Push-Location frontend; bun run test; bun run check; bun run build; Pop-Location
```

Expected: PASS.

---

### Task 8: End-to-end PostgreSQL and runtime verification

**Files:**
- Modify: `backend/README.md`
- Modify: `crawler/README.md`
- Modify: `database/README.md`
- Create: `docs/superpowers/specs/2026-08-17-location-aware-crawler-matching-design.md` if the implementation reveals a contract decision not covered by the existing design

- [ ] **Step 1: Apply migrations to the local PostgreSQL database.**

Run each migration in order with `ON_ERROR_STOP=1` and verify the new columns/indexes/tables with read-only SQL. Do not insert a fake job. A real canonical location created through the admin API may be used for verification.

- [ ] **Step 2: Run a real crawler smoke cycle.**

Start the Rust crawler from `crawler/` with its existing `.env`. Add/activate only a user-approved Job Link. Verify from PostgreSQL that `source_crawl_runs` is recorded, `job_cache` contains only structured fields, and `location_id` is resolved or explicitly NULL.

- [ ] **Step 3: Run the API smoke contract.**

Verify:

```text
GET /healthz
GET /api/v1/client/locations
GET /api/v1/admin/jobs?location_id=<known-id> with an admin session
GET /api/v1/admin/jobs?unresolved=true with an admin session
POST /api/v1/client/scans with a real temporary CV and location_id
GET /api/v1/client/scans/:scan_id until terminal state
```

If DeepSeek credentials/provider are unavailable, record the explicit provider failure and verify that no fabricated profile/matches were written.

- [ ] **Step 4: Run the complete local quality suite.**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File database/tests/validate-schema.ps1
Push-Location backend; go test ./... -count=1; go vet ./...; Pop-Location
Push-Location crawler; cargo fmt --all -- --check; cargo test; cargo check; Pop-Location
Push-Location frontend; bun run test; bun run check; bun run build; Pop-Location
```

- [ ] **Step 5: Record Baron proof, gates, continuity, and trace.**

Record exact command receipts for the quality gates, update the focused plan checkpoint after each task, run the autopilot review, and run `baron trace score`. Do not claim complete if required proof is missing or trace quality fails.

## Self-review checklist

- [ ] Every user requirement maps to at least one task: generic crawler, automatic location resolution, client location filtering, admin Location/Job Cache link, no mock data, structured-only persistence, deterministic matching, and honest unavailable states.
- [ ] No task depends on a source-specific adapter or invented data.
- [ ] Old scan/location text behavior has an explicit compatibility path while the frontend migrates to canonical IDs.
- [ ] All list APIs have bounded pagination and stable ordering.
- [ ] New provider and scan errors use bounded machine-readable codes.
- [ ] Raw CV/JD data is absent from response and persistence contracts.
- [ ] Frontend loading, empty, error, disabled, keyboard, mobile, desktop, and reduced-motion states have verification steps.
- [ ] Unknown production provider behavior and multi-location job behavior remain explicitly unknown rather than silently guessed.
