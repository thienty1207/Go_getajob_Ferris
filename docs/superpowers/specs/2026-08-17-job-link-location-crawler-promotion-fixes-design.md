# Job Link, location, crawler, and promotion fixes

## Goal

Make the first crawl path usable end to end without weakening the product boundary:

- Job Link is an allowlist record, not an invitation to crawl the open web.
- Deleting a Job Link is a real destructive delete of the source-owned crawl data.
- Pausing and resuming a Job Link are separate actions.
- Locations are canonical records that can be assigned to cached jobs.
- The crawler starts automatically from `crawler/.env` and extracts structured `JobPosting` JSON-LD only from pages inside the registered URL scope.
- The client promotion carousel displays the existing promotion image as a borderless 16:9 surface.

## Decisions

### Job Link lifecycle

`ACTIVE` and `DISABLED` remain source approval states. The API exposes status changes explicitly and keeps hard deletion separate:

- `PATCH /api/v1/admin/job-links/:id/status` with `ACTIVE` or `DISABLED` pauses/resumes a source.
- `DELETE /api/v1/admin/job-links/:id` permanently removes the source and source-owned crawl runs, cached jobs, and match rows in one transaction.
- The admin UI must confirm deletion and name the data that will be removed.

This prevents the current ambiguous “Xóa” action from silently turning into “Dừng lại”. A deleted source cannot be restored; it can be added again as a new source record.

The local frontend dev server proxies `/api` to the Go API. Production may still use `PUBLIC_API_BASE_URL`; no API response or local fake row is added to hide a failed connection.

### Canonical locations

Add a `locations` table with a canonical display name, province/city, country, active flag, and timestamps. Add nullable `job_cache.location_id`:

- `location_text` remains the source-provided text for audit and crawler reconciliation.
- `location_id` is the admin-approved canonical location.
- The public job view prefers the canonical display name when assigned and falls back to the source text otherwise.
- Deleting a location sets `job_cache.location_id` to `NULL`; it does not delete a job.

The admin API supports list/create/update locations and assigning or clearing a job’s canonical location. The UI has one location form, a real list, and an assignment control for current job-cache rows.

### Crawler configuration and scope

The crawler loads `.env` at startup. An explicitly exported `DATABASE_URL` wins. If it is absent, the loader builds a PostgreSQL URL from the existing local aliases (`username`, `password`, `port`, plus optional host/database aliases) with safe URL encoding and local defaults. Secrets are never logged.

The first adapter is a generic server-rendered JSON-LD `JobPosting` adapter. It:

- reads only `script[type="application/ld+json"]` from pages already selected by the scoped Spider crawl;
- maps title, employer, location, employment type, work mode, skills, seniority, experience, domains, and salary when present;
- hashes normalized structured JSON-LD and never stores HTML or the raw description;
- treats an expired `validThrough` value as an explicit closed signal;
- returns parser/anomaly health when a page has no usable JobPosting object.

The adapter does not call a hidden listing endpoint and does not widen the registered path. A client-rendered listing page whose API endpoint is outside the registered path remains unsupported until that endpoint is explicitly approved and represented in the source contract. This keeps the crawler source-controlled and no-mock.

### Promotion surface

The client carousel has no outer decorative border and uses `aspect-ratio: 16 / 9` with `object-fit: cover`. The admin preview follows the same ratio so an uploaded 1920x1080 image is previewed without a square crop. Existing image validation and Cloudinary flow remain unchanged.

## Non-goals

- No unrestricted web search or arbitrary source discovery.
- No mock jobs, mock locations, local-only persistence, or hard-coded production rows.
- No raw CV or full raw JD storage.
- No client authentication implementation in this tranche.
- No DeepSeek parsing integration in the crawler tranche.
- No automatic approval of a newly added source.

## Risks and safeguards

- Hard deletion removes source-owned cache and match data. The confirmation message and transactional delete make the scope explicit; this is intentionally different from pause/resume.
- A JSON-LD page may omit fields required by the database. The adapter rejects that observation and records parser/anomaly health instead of inventing a row.
- A Job Link can be syntactically valid but not permissioned. It stays pending/disabled until the admin-approved source contract is satisfied; the crawler only consumes `ACTIVE` rows.
