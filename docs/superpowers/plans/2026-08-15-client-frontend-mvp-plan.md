# Client Frontend MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the real-API-only SvelteKit client surface for CV upload and matched-job results, with a strict physical boundary from the future admin surface.

**Architecture:** Scaffold a small SvelteKit app under `frontend/`. Keep client routes/components/API under `client/`, shared HTTP/types/styles under `shared/`, and reserve `admin/` without importing it. The client submits a multipart CV scan request, polls a scan status endpoint, and renders only API data or honest loading/empty/error states.

**Tech Stack:** SvelteKit, TypeScript, Bun, native CSS tokens, browser `fetch`, no UI framework, no mock-data library.

## Global Constraints

- Absolutely no mock data, fake API responses, seeded job records, sample result cards, or fallback demo payloads.
- The first tranche is client-only; do not implement admin, backend, database, crawler, or DeepSeek.
- Client route/module ownership must be physically separated from future admin route/module ownership.
- CV input accepts PDF, DOCX, and TXT up to 10 MB.
- Location and radius are required; radius is in kilometres.
- Public results show match percent, title, company, location, distance, employment type, work mode, optional source salary, at most three skill tags, and original URL.
- CV raw file is sent to the real API boundary and is not persisted by the browser.
- The user-facing copy must not imply that `CV Match %` predicts hiring probability.

---

### Task 1: Scaffold the SvelteKit client and route boundaries

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/svelte.config.js`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tsconfig.json`
- Create: `frontend/src/app.html`
- Create: `frontend/src/routes/+page.svelte`
- Create: `frontend/src/routes/client/+page.svelte`
- Create: `frontend/src/routes/admin/.gitkeep`
- Create: `frontend/src/lib/client/.gitkeep`
- Create: `frontend/src/lib/admin/.gitkeep`
- Create: `frontend/src/lib/shared/.gitkeep`

**Interfaces:**
- Produces a runnable `/client` route and a root redirect/entrypoint.
- Leaves physical `admin` and `client` directories visible without putting admin logic in the client.

- [ ] **Step 1: Write the scaffold smoke assertion**

Create a test command that can later prove `/client` builds and renders. Before implementation, verify the repository has no existing frontend package or test command.

- [ ] **Step 2: Run the baseline command**

Run `bun --version` and confirm the current repository has no package to test. Expected: Bun is installed; no existing app baseline exists.

- [ ] **Step 3: Create the minimal SvelteKit files**

Use the current Bun/SvelteKit scaffold or an equivalent minimal configuration. The root page must direct users to `/client`; it must not contain job or admin data.

- [ ] **Step 4: Run type/build checks**

Run `bun run check` and `bun run build`. Expected: both commands exit 0.

### Task 2: Add shared types and the real API transport

**Files:**
- Create: `frontend/src/lib/shared/types/client.ts`
- Create: `frontend/src/lib/shared/types/job.ts`
- Create: `frontend/src/lib/shared/api/api-errors.ts`
- Create: `frontend/src/lib/shared/api/http-client.ts`
- Create: `frontend/src/lib/client/api/client-api.ts`

**Interfaces:**
- `ClientScanInput = { file: File; location: string; radiusKm: number }`.
- `ScanAccepted = { scanId: string; status: 'processing' }`.
- `ScanStatus = processing | completed | failed`.
- `JobMatch` contains only the public result fields and `skillTags: string[]`.
- `startScan(input): Promise<ScanAccepted>` sends `POST /api/v1/client/scans`.
- `getScanStatus(scanId): Promise<ScanStatusResponse>` sends `GET /api/v1/client/scans/:scanId`.

- [ ] **Step 1: Write type-level/API behavior tests**

Cover accepted status parsing, failed response parsing, non-2xx error conversion, and the three public job states without inserting any product sample records.

- [ ] **Step 2: Run the tests and verify they fail**

Run `bun test`. Expected: the new API modules/tests are absent or fail with missing exports.

- [ ] **Step 3: Implement the transport**

Read `PUBLIC_API_BASE_URL` from `import.meta.env`. Use `FormData` for the CV upload. Convert non-2xx responses into a typed `ApiError`. Do not catch a failed request and return fake success.

- [ ] **Step 4: Run the tests**

Run `bun test` and `bun run check`. Expected: API contract tests and TypeScript checks pass.

### Task 3: Implement file/location/radius validation

**Files:**
- Create: `frontend/src/lib/client/validation/cv-file.ts`
- Create: `frontend/src/lib/client/validation/scan-form.ts`
- Create: `frontend/src/lib/client/validation/*.test.ts`

**Interfaces:**
- `validateCvFile(file: File | null): ValidationResult`.
- `validateScanForm(input): FieldErrors`.

- [ ] **Step 1: Write failing tests**

Test PDF/DOCX/TXT acceptance, unsupported extensions, the 10 MB boundary, missing file, missing location, invalid radius, and a valid Vietnamese location plus km radius.

- [ ] **Step 2: Run the tests to verify red**

Run the focused validation test command. Expected: failures because validators do not exist.

- [ ] **Step 3: Implement minimal validators**

Normalize the extension case-insensitively, check bytes rather than a display string, and return field-level errors suitable for labels and `aria-describedby`.

- [ ] **Step 4: Run focused and full checks**

Run the focused validation tests followed by `bun test`. Expected: all pass.

### Task 4: Build the client upload flow without product data

**Files:**
- Create: `frontend/src/lib/client/components/ClientHeader.svelte`
- Create: `frontend/src/lib/client/components/CvUploadForm.svelte`
- Create: `frontend/src/lib/client/components/ScanProgress.svelte`
- Create: `frontend/src/lib/client/components/ClientEmptyState.svelte`
- Create: `frontend/src/lib/client/stores/scan-store.ts`
- Modify: `frontend/src/routes/client/+page.svelte`

**Interfaces:**
- The page owns no job records.
- The store owns `{ status, selectedFile, location, radiusKm, error, scanId }` only.
- Submit calls `startScan`, then polls `getScanStatus` until completed/failed/timeout.

- [ ] **Step 1: Write interaction tests**

Test idle, invalid submit, selected file, disabled controls during processing, retry after API error, and empty result response. Tests may assert state transitions and request shapes; they must not add fake job cards to runtime code.

- [ ] **Step 2: Run interaction tests to verify red**

Run the focused component test command. Expected: missing component/store failures.

- [ ] **Step 3: Implement the minimal flow**

Use native file input plus drag/drop enhancement. Use an `aria-live` status region. Keep the selected location/radius on retry. Render no results until the API returns a completed response.

- [ ] **Step 4: Run tests and type checks**

Run `bun test` and `bun run check`. Expected: all interaction and type checks pass.

### Task 5: Render API-backed job results

**Files:**
- Create: `frontend/src/lib/client/components/MatchResults.svelte`
- Create: `frontend/src/lib/client/components/MatchJobCard.svelte`
- Create: `frontend/src/lib/client/components/ClientErrorState.svelte`
- Modify: `frontend/src/routes/client/+page.svelte`

**Interfaces:**
- `MatchResults` receives `JobMatch[]` from the API only.
- `MatchJobCard` displays at most three tags, preserves source salary text, and uses an external original URL.

- [ ] **Step 1: Write rendering tests**

Cover no result rows, missing salary, remote job without distance, long title/company, and a returned response with more than three skill tags being visually capped at three.

- [ ] **Step 2: Run rendering tests to verify red**

Run the focused component test command. Expected: missing result component failures.

- [ ] **Step 3: Implement result rendering**

Render only values from the API response. Use an honest empty state with a next action. Never insert the UI Demo's example companies, scores, salaries, or locations.

- [ ] **Step 4: Run tests and type checks**

Run `bun test` and `bun run check`. Expected: all pass.

### Task 6: Apply responsive, accessible visual design

**Files:**
- Create: `frontend/src/lib/shared/styles/tokens.css`
- Create: `frontend/src/lib/shared/styles/global.css`
- Modify: `frontend/src/routes/client/+page.svelte`
- Modify: client components under `frontend/src/lib/client/components/`

**Interfaces:**
- No new data source; styling consumes only client state/API values.

- [ ] **Step 1: Write the visual/state checklist**

Record checks for narrow viewport, wide viewport, long text, loading, empty, error, disabled controls, keyboard focus, and reduced motion.

- [ ] **Step 2: Implement design tokens and composition**

Use the evidence-backed black/red Ferris visual language, a focused upload-first composition, native controls, explicit focus styles, and CSS media queries. Use CSS decoration only where it reinforces hierarchy.

- [ ] **Step 3: Run browser verification**

Start the app with `bun run dev -- --host 0.0.0.0`, inspect `/client` in the in-app browser, and capture a screenshot. Check at least desktop and narrow viewport states. With no backend, verify the initial form and honest API-unavailable/error state; do not fake a success response.

- [ ] **Step 4: Run build/check after visual changes**

Run `bun run check`, `bun test`, and `bun run build`. Expected: all commands exit 0.

### Task 7: Final verification and handoff

**Files:**
- Inspect: all changed frontend files.
- Inspect: `docs/superpowers/specs/2026-08-15-client-frontend-mvp-design.md`.
- Inspect: `docs/superpowers/plans/2026-08-15-client-frontend-mvp-plan.md`.

- [ ] **Step 1: Verify no mock data exists**

Run a repository search for `mock`, `fixture`, `sample job`, `Monzo`, `Revolut`, `London`, and hard-coded result arrays under `frontend/src`. Review matches and remove any runtime product data.

- [ ] **Step 2: Verify changed-surface quality gates**

Record each frontend gate as observed, corrected, not applicable with reason, or not verified. Do not claim responsive/API success beyond what the browser and real runtime prove.

- [ ] **Step 3: Run final commands**

Run `bun test`, `bun run check`, and `bun run build` fresh. Preserve exact exit codes/output for handoff.

- [ ] **Step 4: Report the screenshot path and remaining integration gap**

Return the client screenshot as a clickable local artifact and state that populated results require the real backend endpoint; do not claim end-to-end success until it exists.

---

## Plan self-review

- Spec coverage: client route boundary, no mock data, real API transport, upload validation, polling states, public job fields, responsive/accessibility behavior, and screenshot verification are covered by Tasks 1–7.
- Placeholder scan: no `TBD`, `TODO`, or unspecified implementation step is required for the client tranche.
- Type consistency: `ClientScanInput`, `ScanAccepted`, `ScanStatusResponse`, and `JobMatch` are introduced in Task 2 and consumed by Tasks 4–5.
- Known gap: backend endpoint/auth deployment is unknown, so browser proof can verify the client shell and honest failure path only until the API is available.
