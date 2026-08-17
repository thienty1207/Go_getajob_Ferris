# Admin Settings and Crawl Control Design

## Goal

Give an authenticated admin one operational control surface for the crawler and
make a Job Link crawl request observable without inventing job data.

## Product behavior

The admin has two independent controls:

1. `Crawl ngay` on an ACTIVE Job Link creates a durable request in PostgreSQL.
   The request is idempotent while it is pending or running. If the Rust daemon
   is offline, the request remains pending and is consumed after the daemon is
   started.
2. `Settings` contains the crawler interval as an hours-and-minutes form. The
   default is six hours (`21600` seconds), and the saved value is used for the
   next periodic cycle without requiring a daemon restart.

The periodic scheduler crawls every ACTIVE Job Link. A manual request targets one
ACTIVE Job Link. Both paths use the same scoped, structured, fail-closed crawl
pipeline and the existing source-crawl-run lifecycle.

## Boundaries and storage

`.env` remains the owner of database connection aliases and server/provider
secrets. Product-operational settings belong in PostgreSQL so an admin change is
durable and visible to every crawler process.

`public.app_settings` is a small typed registry boundary. The first registered
key is `crawler.interval_seconds`; the database stores a JSON number so future
settings can use the same table without putting arbitrary unvalidated fields in
the API. Go owns the key registry and validation. Unknown keys are never
accepted from the browser.

`public.crawl_requests` stores the manual queue. Its status is one of
`PENDING`, `RUNNING`, `COMPLETED`, or `FAILED`. The linked source crawl run
retains the authoritative result (`HEALTHY`, `SOURCE_ERROR`, `PARSER_ERROR`, or
`ANOMALY`) and counts. A partial unique index prevents duplicate pending or
running requests for one source.

Deleting a Job Link cascades its queued request rows. Disabling a source does
not create a new crawl request and a queued request for a source that becomes
inactive is finalized as failed with a bounded `source_inactive` code.

## API contract

Authenticated admin routes, protected by the existing session and CSRF rules:

```text
GET  /api/v1/admin/settings
PATCH /api/v1/admin/settings/crawler
POST /api/v1/admin/job-links/:id/crawl
```

`GET /settings` returns the typed crawler form model:

```json
{
  "crawler": {
    "interval_hours": 6,
    "interval_minutes": 0,
    "interval_seconds": 21600,
    "minimum_interval_minutes": 15,
    "maximum_interval_minutes": 10080
  }
}
```

`PATCH /settings/crawler` accepts only:

```json
{"interval_hours": 6, "interval_minutes": 0}
```

The server converts the pair to seconds, requires a whole non-negative hour and
minute part, rejects a total below 15 minutes or above 7 days, and returns the
canonical saved model. The response never echoes arbitrary setting keys or
secrets.

`POST /job-links/:id/crawl` returns `202 Accepted` with:

```json
{
  "request_id": "uuid",
  "source_id": "uuid",
  "status": "PENDING",
  "requested_at": "timestamp"
}
```

If the same source already has a pending or running request, the endpoint
returns that existing request with `202` instead of creating a duplicate. A
disabled or unknown source returns a bounded `409`/`404` error respectively.

The existing Job Link list adds bounded observability fields: active request
status, latest run status/time, page/job counters, and latest error code. It does
not return fetched HTML, raw JD, or provider payloads.

## Runtime flow

The Rust daemon starts with an immediate full ACTIVE-source cycle. It then keeps
a short request-poll loop and a periodic schedule. The request poll interval is
an internal five-second operational constant; it is not an admin setting in this
tranche. Each poll claims one pending request using a transaction-safe
`FOR UPDATE SKIP LOCKED` update, then runs the same source pipeline used by the
periodic cycle. The request is completed only after the linked source run has
been finalized; anomaly and parser/source errors are represented explicitly and
never converted to healthy or missing observations.

The scheduler reads `crawler.interval_seconds` from PostgreSQL when it starts
and after each completed periodic cycle. If the setting changes while the
daemon is waiting, the next periodic deadline is recalculated from the new
interval. Manual requests are not delayed until that deadline.

`cargo run -- --once` processes the current pending queue and one full cycle,
then exits. `cargo run` stays alive as the production-style daemon.

## Admin UI

The sidebar gains `Settings`. The page has one compact Crawler section with two
numeric inputs (hours and minutes), a save action, and the canonical saved
interval. It also shows the next operational state without explanatory filler.

Each Job Link row gains `Crawl ngay`. While a request is pending or running, the
button is disabled and the row polls the bounded Job Link list for fresh state.
After completion, the row shows the run status and counts. Errors preserve the
link list and offer a retry through the same button. No UI state pretends that a
job was found when the crawler reports an anomaly.

The page follows the existing admin dark surface, touch-sized controls, visible
keyboard focus, and mobile single-column layout. The empty, loading, error,
disabled, and success states are explicit.

## Verification

- Go unit/HTTP tests cover settings validation, default conversion, admin auth
  and CSRF, request idempotency response, disabled-source rejection, and list
  serialization.
- Rust tests cover interval parsing, default fallback, pending request claim
  semantics at the pure boundary, and daemon one-shot scheduling behavior where
  it can be tested without a live network source.
- PostgreSQL integration smoke covers settings persistence, request dedupe, and
  request deletion with a Job Link.
- Frontend tests cover request shapes/parsing and the Settings/Crawl Now loading,
  success, error, and disabled states.
- Fresh `go test ./...`, `go vet ./...`, `cargo fmt --all -- --check`,
  `cargo test`, `cargo check`, schema contract, `bun run check`, `bun run test`,
  and `bun run build` are required.

## Non-goals

- No site-specific crawler adapter.
- No unrestricted web crawl or browser automation.
- No mock job rows or fabricated crawl success.
- No public client settings.
- No arbitrary secret storage in the settings table.
- No redesign of unrelated admin pages.
