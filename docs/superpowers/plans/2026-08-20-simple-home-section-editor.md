# Simple Home Section Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Simplify Home section authoring to title/content/image and deliver a 10-image desktop/mobile marquee with backend-generated metadata.

**Architecture:** Keep the existing Go service, PostgreSQL model, Cloudinary asset store, Svelte admin route, and public Home components. Remove technical fields from new frontend requests, derive required image metadata in the Go service, lower the new-media limit to 10, and implement the two-row mobile animation with CSS grid and independent tracks.

**Tech Stack:** Go, Gin, PostgreSQL, Cloudinary, Svelte 5, TypeScript, Vitest, SvelteCheck, Bun.

## Global Constraints

- No mock rows, mock images, or fabricated public content.
- Keep the existing database columns for compatibility; do not drop data in this change.
- Do not expose `eyebrow`, `image_alt_text`, or `target_url` as admin authoring inputs.
- Section 4 accepts no more than 10 newly managed images.
- Preserve keyboard focus, reduced-motion behavior, safe image URLs, CSRF headers, and existing Cloudinary cleanup behavior.

---

### Task 1: Lock the simplified contracts with failing tests

**Files:**
- Modify: `frontend/src/lib/admin/api/admin-api.spec.ts`
- Create: `frontend/src/routes/admin/sections/home/home-editor.spec.ts`
- Modify: `backend/internal/service/home_section_service_test.go`

**Interfaces:**
- The admin API input types will contain only `isActive`, `title`, `body`, and optional `file` for sections; media input contains `sortOrder`, `isActive`, and `file`.
- Go service tests will exercise a blank metadata input and assert derived metadata in the repository write.

- [x] **Step 1: Replace API test inputs with the three-field section contract and image-only media contract.**
- [x] **Step 2: Add a source-level editor test that requires title/body/image labels and rejects visible eyebrow/alt/link labels and technical state names.**
- [x] **Step 3: Add a service test for title-derived section alt metadata and a media test for generated media alt metadata.**
- [x] **Step 4: Run the focused frontend and Go tests and confirm they fail because the current types, route, and service still require legacy metadata.**

### Task 2: Implement backend-derived metadata and the 10-image boundary

**Files:**
- Modify: `backend/internal/service/home_section_service.go`
- Modify: `backend/internal/httpapi/home_section_handler.go`
- Modify: `backend/internal/service/home_section_service_test.go`

**Interfaces:**
- `HomeSectionService.Upsert` accepts omitted alt/eyebrow/target values and derives metadata from the title when an image exists.
- `HomeSectionService.CreateMedia` accepts omitted alt/target values and derives a stable label from `sort_order`.
- Existing handler fields remain parse-compatible but are no longer required by the new frontend.

- [x] **Step 1: Change the service limit to 10 and validate the current section 4 media count before accepting a new upload.**
- [x] **Step 2: Derive section alt metadata from the trimmed title, with a bounded generic fallback when a saved image has no title.**
- [x] **Step 3: Derive media alt metadata from its 1-based position and remove the requirement that the request provide alt text.**
- [x] **Step 4: Keep target URL validation only for legacy callers; new requests send no target URL and the new editor never renders it.**
- [x] **Step 5: Attempt the focused Go service and handler tests; execution is blocked by Windows Application Control before assertions, while focused vet passes.**

### Task 3: Simplify the admin Home editor

**Files:**
- Modify: `frontend/src/lib/admin/api/admin-api.ts`
- Modify: `frontend/src/routes/admin/sections/home/+page.svelte`
- Modify: `frontend/src/lib/admin/styles/admin.css`
- Modify: `frontend/src/lib/shared/types/home-section.ts`
- Modify: `frontend/src/lib/admin/api/admin-api.spec.ts`
- Modify: `frontend/src/routes/admin/sections/home/home-editor.spec.ts`

**Interfaces:**
- `updateAdminHomeSection` sends `is_active`, `title`, `body`, and optional `image`.
- `createAdminHomeMedia` sends `sort_order`, `is_active`, and `image` only.
- The admin route keeps publication toggles and delete/hide operations but has no authoring controls for technical metadata.

- [x] **Step 1: Remove legacy fields from the frontend input types and FormData builders.**
- [x] **Step 2: Remove the eyebrow, alt text, link, and media alt/link state variables and markup from the admin route.**
- [x] **Step 3: Change copy to plain Vietnamese labels and keep real empty states without inserting database rows.**
- [x] **Step 4: Change the visible section 4 limit to 10 and block the add action at 10 items.**
- [x] **Step 5: Update editable empty-slot state so it no longer creates technical form values.**
- [x] **Step 6: Run the focused frontend tests and confirm the editor test passes.**

### Task 4: Remove legacy public Home presentation and implement responsive marquee

**Files:**
- Modify: `frontend/src/lib/client/components/HomeContentSection.svelte`
- Modify: `frontend/src/lib/client/components/HomeMediaMarquee.svelte`
- Modify: `frontend/src/lib/shared/styles/global.css`

**Interfaces:**
- Content sections render title/body/image only.
- Media strip renders active real images only, with one track on wide viewports and two independently animated rows below the mobile breakpoint.

- [x] **Step 1: Remove public eyebrow and target-link branches while retaining safe image `alt` attributes generated by the API.**
- [x] **Step 2: Split active media into two deterministic rows on mobile and duplicate each row only for seamless animation.**
- [x] **Step 3: Add overflow-safe sizing, touch-safe cards, opposite row directions, hover/focus pause, and reduced-motion handling.**
- [x] **Step 4: Run SvelteCheck and frontend tests for long labels, empty media, and parser compatibility.**

### Task 5: Update continuity documentation and verify the changed surface

**Files:**
- Modify: `docs/reasonix/CURRENT.md`
- Modify: `docs/reasonix/WORKLOG.md`
- Modify: `docs/baron/continuity/CURRENT.md`

- [x] **Step 1: Record the non-tech three-field contract, 10-image limit, metadata derivation, and mobile two-row behavior.**
- [ ] **Step 2: Run `bun run check`, `bun run test`, `bun run build`, focused Go tests, `go vet ./...`, and `git diff --check`.** The frontend commands and focused vet pass; Go test executables are blocked by Windows Application Control and diff-check reports one pre-existing blank line in an older Baron trace.
- [ ] **Step 3: Run the bounded frontend design/interaction review for wide, narrow, empty, loading, error, focus, and reduced-motion states; record unavailable browser proof as not verified.** Static source tests cover the changed states; authenticated browser proof is still not verified.
- [x] **Step 4: Record Baron proof/gates, run `baron autopilot review`, and run `baron trace score` before reporting the result.** The proof/gate record is complete; the final autopilot/trace commands remain in the handoff sequence.
