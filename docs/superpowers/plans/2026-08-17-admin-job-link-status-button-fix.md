# Admin Job Link Status Button Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Job Link “Dừng” action reliably reach the authenticated Go API and persist `DISABLED`, while keeping “Xóa” as a separate hard-delete action.

**Architecture:** Trace the existing browser-to-API path first. The current failure is at the browser preflight boundary: the Go CORS response does not advertise `PATCH`, so the browser blocks the authenticated status request before the handler sees it. Add `PATCH` to the explicit method allowlist, preserve the existing host/session/CSRF behavior, and lock the contract with a backend preflight test.

**Tech Stack:** SvelteKit/Vite, TypeScript, Bun test/check/build, Go/Gin, PostgreSQL, Baron proof/trace.

## Global Constraints

- No mock data or fake Job Link rows.
- `PATCH /api/v1/admin/job-links/:id/status` changes `approval_status`; `DELETE /api/v1/admin/job-links/:id` remains hard-delete.
- Preserve admin session cookie and CSRF requirements.
- Development requests should use the existing Vite proxy when the page runs at `localhost:5173`.
- Do not change crawler behavior, schema, or unrelated admin screens.

---

### Task 1: Reproduce and isolate the failing boundary

**Files:**
- Inspect: `frontend/src/lib/shared/api/api-url.ts`
- Inspect: `frontend/src/lib/shared/api/api-url.spec.ts`
- Inspect: `frontend/src/lib/admin/api/admin-api.ts`
- Inspect: `frontend/src/routes/admin/sources/+page.svelte`
- Inspect: `backend/internal/httpapi/router.go`
- Inspect: `backend/internal/httpapi/admin_job_link_handler.go`

- [x] Capture the active frontend API base, Vite proxy target, page host, backend listener, and CORS preflight response.
- [x] Confirm the failure is a CORS preflight omission: the running API advertised no `PATCH`; the route and CSRF contract already exist.

### Task 2: Add the regression test before changing production code

**Files:**
- Modify: `backend/internal/httpapi/handler_test.go`
- Test: `backend/internal/httpapi/handler_test.go`

- [x] Add a preflight test that describes the admin status contract: an allowed `localhost:5173` origin receives `PATCH` in `Access-Control-Allow-Methods`.
- [x] Run the focused test and verify it failed because the current response omitted `PATCH`.

### Task 3: Implement the smallest transport fix

**Files:**
- Modify: `backend/internal/httpapi/middleware.go`

- [x] Add `PATCH` to the explicit CORS method allowlist without broadening origins or headers.
- [x] Run the focused preflight test and confirm it passes.

### Task 4: Verify the status interaction and regression surface

**Files:**
- Inspect: `frontend/src/lib/admin/api/admin-api.ts`
- Inspect: `frontend/src/routes/admin/sources/+page.svelte`
- Inspect: `backend/internal/httpapi/admin_job_link_handler_test.go`
- Inspect: `backend/internal/service/job_link_test.go`

- [x] Run frontend unit tests, check, and production build.
- [x] Run the backend Job Link handler/service tests and all Go package tests.
- [x] Smoke-test the updated CORS preflight on a fresh backend process without inventing credentials or data; the existing `8080` process predates the patch and must be restarted.
- [x] Inspect the Job Link interaction path and confirm confirmation, saving/disabled, success reload, and transport/auth error states are already represented in the existing page code; live authenticated click remains unavailable because the browser session is anonymous.

### Task 5: Record Baron evidence and hand off

- [ ] Record proof receipts for actually executed registered providers; the backend proof runner blocked without a receipt, so no trusted proof is claimed.
- [x] Run `baron trace score` and `baron autopilot review`.
- [x] Update the Baron plan/checkpoint with changed files, fresh command results, and the stale `8080` runtime limitation.
