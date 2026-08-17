# Admin Management And Crawler Runtime Design

**Status:** Approved by the product owner on 2026-08-17.

## Goal

Make the admin console operationally truthful and easy to manage: all large
admin lists are server-paginated at ten rows per page, Job Cache owns inline
canonical-location assignment, Job Cache supports search, the sidebar groups
related screens, and Settings exposes real crawler liveness and next-cycle
timing from PostgreSQL-backed runtime state.

## Product rules

- Job Link, Job Location, and Job Cache are admin-only operational data.
- Every admin list uses a bounded server-side page size; the UI maximum is ten.
- `job_cache.location_id` is the canonical location assignment for a job.
- Job Location creates, edits, activates, and deactivates canonical locations;
  it does not own the job-assignment workflow.
- Job Cache shows only the job lifecycle badge. Source approval is not rendered
  as a second lifecycle badge.
- Search and filters query PostgreSQL; no local mock rows or browser-only
  filtering are allowed.
- The crawler countdown is derived from persisted `next_cycle_at` and a
  persisted heartbeat. A browser timer may refresh the display, but it cannot
  claim that the crawler is alive without a recent database heartbeat.
- No raw job description or raw CV is added to any response or table.
- This tranche does not implement a source-specific crawler adapter or broaden
  the crawler source boundary.

## Architecture

### Admin list contracts

The admin Job Link and Job Cache endpoints keep their existing route families
and add bounded query contracts:

```text
GET /api/v1/admin/job-links?page=1&page_size=10
GET /api/v1/admin/jobs?page=1&page_size=10&q=developer&location_id=<uuid>
GET /api/v1/admin/jobs?page=1&page_size=10&unresolved=true
GET /api/v1/admin/locations?page=1&page_size=10
GET /api/v1/admin/locations/options
```

The server accepts page sizes from 1 through 10 and rejects larger values. The
options endpoint returns the canonical location choices needed by Job Cache
rows without changing the paginated Job Location list contract.
Job Cache search is parameterized and matches the existing structured fields
needed by an operator: title, company, role, location text, source name/key,
and original URL. The public client location endpoint remains a separate
unpaged active-location contract.

### Location ownership

The existing `PATCH /api/v1/admin/jobs/:id/location` contract remains the
single write boundary for `job_cache.location_id`. The frontend moves its
caller from the Job Location screen into each Job Cache row. This preserves
the already-tested transaction that validates the selected canonical location
and the job before committing.

### Crawler runtime state

Migration `000009` adds a singleton `crawler_runtime` row with status,
heartbeat, cycle start/finish, next-cycle, current-source, and last-error
fields. The Rust daemon updates the row when it polls, starts a cycle, starts
each source, completes a cycle, and encounters a recoverable cycle error.

The admin Settings response adds a runtime object. Go derives `OFFLINE` when
the last heartbeat is older than the documented heartbeat freshness window;
the stored scheduler status remains available for `IDLE`, `RUNNING`, and the
last error. The UI polls the endpoint periodically and ticks the remaining
seconds locally only as presentation.

### Navigation and visual corrections

The sidebar contains two expandable groups:

```text
Manage Job
  Job Link
  Job Location
  Job Cache

Manage User
  Users
  CV
```

The current route opens its group automatically. Job Link and Job Location
render ten records per page. Job Cache renders ten records per page, a search
field, a location select with an explicit unassigned option, and an inline
location select per row. The Location page keeps only canonical-location
management and its paginated list. Existing dark tokens and responsive
patterns remain; the change fixes spacing and information ownership rather
than adding a new visual language.

## Failure behavior

- Invalid page, page size, UUID, or search input returns the existing structured
  admin error shape with a specific machine-readable code.
- A database or crawler-runtime read failure shows the existing admin error
  state; the UI does not infer an online crawler.
- If a selected location is inactive, the backend still validates the location
  record according to the existing assignment invariant; the UI does not
  fabricate a replacement.
- If the crawler process stops, heartbeat age makes the status `OFFLINE` after
  the freshness window and the countdown is replaced with an offline state.

## Verification

- Go handler/repository/service tests cover page bounds, search, location
  pagination, inline assignment, and runtime serialization.
- Rust tests cover runtime status transitions and scheduler timestamp updates;
  the existing crawl/reconcile tests remain green.
- Frontend tests cover API parsing, pagination query parameters, search/location
  filter state, and runtime countdown display states.
- Fresh `go test ./... -count=1`, `go vet ./...`, `cargo test`, `cargo check`,
  `bun run check`, `bun run test:unit`, and `bun run build` are required.
- The migrated PostgreSQL schema and authenticated admin endpoints receive a
  smoke check, and changed admin screens receive desktop and mobile browser
  evidence.
