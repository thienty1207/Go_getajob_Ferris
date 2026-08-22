# Go backend API

This directory contains the real API boundary for the client scan flow, promotion carousel, and first admin console. It uses Gin, self-hosted PostgreSQL, and server-side Cloudinary uploads. There are no runtime sample CVs, fallback profiles, fabricated matches, or bundled promotion images. The one local Job Cache fixture is explicit and development-only.

## Current endpoints

- `GET /healthz` checks that the PostgreSQL pool is reachable.
- `GET /api/v1/client/locations` returns active canonical locations for the client selector.
- `POST /api/v1/client/scans` accepts multipart fields `cv`, `location_id`, and `radius_km` and returns `202` with `{ "scan_id": "...", "status": "processing" }`.
- `GET /api/v1/client/scans/:scan_id` returns `processing`, `failed`, or `completed` using the SvelteKit client contract. Processing includes `phase` (`received`, `parsing`, or `matching`); completed includes the bounded `cv_summary` and only matches for `ACTIVE` jobs.
- `GET /api/v1/client/promotions` returns up to three active promotion metadata records; `GET /api/v1/client/promotions/:slot/image` keeps the same-origin image contract and redirects Cloudinary-backed rows to the CDN.
- `POST /api/v1/admin/auth/login`, `GET /api/v1/admin/auth/me`, and `POST /api/v1/admin/auth/logout` implement the cookie session boundary.
- `GET /api/v1/admin/promotions` lists current slots; `PUT /api/v1/admin/promotions/:slot` uploads one multipart `image` through Cloudinary; `DELETE /api/v1/admin/promotions/:slot` removes a slot. State-changing routes require an authenticated session and CSRF header.
- `GET /api/v1/admin/jobs?page=1&page_size=20` returns structured job-cache metadata for the admin console; raw JD content is not persisted or returned.
- `GET /api/v1/admin/job-links?page=1&page_size=20` lists the admin-controlled crawl boundaries; `POST /api/v1/admin/job-links` adds one URL, `PATCH /api/v1/admin/job-links/:id` edits it, `PATCH /api/v1/admin/job-links/:id/status` pauses/resumes it, and `DELETE /api/v1/admin/job-links/:id` permanently deletes the source-owned crawl runs, cached jobs, and match rows. Editing a URL/path closes the previous active/verifying cached jobs in the same transaction so stale jobs cannot remain public under the new boundary; a later healthy crawl can reactivate observed jobs. Writes require the existing session and CSRF header.
- `GET /api/v1/admin/settings` returns the database-backed crawler interval as hours/minutes; `PATCH /api/v1/admin/settings/crawler` saves `{ "interval_hours": 6, "interval_minutes": 0 }` with the configured bounds. `POST /api/v1/admin/job-links/:id/crawl` queues one immediate crawl for an ACTIVE Job Link and returns `202`; the Rust crawler consumes the request when it is running.
- `GET /api/v1/admin/locations` lists canonical locations; `POST /api/v1/admin/locations` creates one, `PATCH /api/v1/admin/locations/:id` edits one, and `PATCH /api/v1/admin/jobs/:id/location` assigns or clears a canonical location. Location rows expose a Job Cache link, and Job Cache supports `location_id` or `unresolved=true` filters.

Provision the first admin once from this directory. The password is entered without echo and is never stored in `.env`:

```powershell
go run ./cmd/admin create-user --email admin@example.com
```

The bootstrap password must be 12-200 characters and cannot contain line
breaks. The CLI validates this before asking for confirmation and prints the
policy directly when the input is rejected.

When `DEEPSEEK_API_KEY` is configured, a valid upload is copied to a temporary file, extracted, sent to the configured DeepSeek parser without unnecessary PII, validated against the structured CV schema, and matched against active structured jobs with the deterministic five-part score. The temporary file is removed after processing. If the key is absent or the provider fails, the scan becomes `failed` with a bounded error code; the API never fabricates a profile or a match.

## Local PostgreSQL setup

Apply the existing migration from pgAdmin 4 or `psql` before starting the API:

```powershell
psql --set ON_ERROR_STOP=1 --file database/migrations/000001_initial_schema.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000002_promotion_slides.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000003_admin_auth_cloudinary.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000004_job_link_constraints.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000005_canonical_locations.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000006_location_resolution_and_scan_location.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000007_location_key_normalization.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000008_settings_and_crawl_requests.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000009_admin_management_runtime.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000010_location_assignment_source.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000011_client_auth.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000012_client_cv_history.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000013_home_sections.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000014_home_asset_cleanup.up.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000015_cv_summary.up.sql
# Optional local admin Job Cache verification fixture; never crawler output.
psql --set ON_ERROR_STOP=1 --file database/fixtures/development-job.sql
```

For local development, the API now reads `backend/.env` when started from the
repository root or `backend/`. The current file can use the simple fields
`port`, `username`, and `password`; `database` defaults to
`gogetsomefoodferris` when omitted. `DATABASE_URL` still wins whenever it is
provided by the process environment. The loader does not mutate the process
environment or print any secret.

Copy [`.env.example`](.env.example) to `.env` for a new machine, then set the
connection values locally. Do not commit credentials:

```powershell
$env:DATABASE_URL = 'postgres://gogetsomefoodferris_app:change-me@127.0.0.1:5432/gogetsomefoodferris?sslmode=disable'
$env:API_ADDR = '127.0.0.1:8080'
$env:CORS_ALLOWED_ORIGINS = 'http://localhost:5173,http://127.0.0.1:5173'
# Set CLOUDINARY_URL in backend/.env or a secret manager. Never expose it to the frontend.
```

For the SvelteKit client, copy `frontend/.env.example` to a local `.env` so its existing API client sends requests to this Go process. The value is a public URL, not a secret.

Optional settings:

| Variable | Default | Meaning |
| --- | --- | --- |
| `APP_ENV` | `development` | Runtime label used by configuration. |
| `MAX_CV_BYTES` | `10485760` | Maximum actual CV bytes copied to a temporary file. |
| `MAX_RADIUS_KM` | `500` | Maximum client search radius in kilometers. |
| `RATE_LIMIT_PER_MINUTE` | `10` | Per-source-IP POST intake limit. |
| `READ_RATE_LIMIT_PER_MINUTE` | `60` | Per-source-IP scan status read limit. |
| `MAX_PROMOTION_IMAGE_BYTES` | `5242880` | Maximum PNG/JPEG/WebP bytes accepted by the promotion admin route. |
| `PROMOTION_RATE_LIMIT_PER_MINUTE` | `10` | Per-source-IP promotion admin write limit. |
| `CLOUDINARY_URL` | required by API runtime | Server-only Cloudinary connection URL used for promotion uploads. |
| `DEEPSEEK_API_KEY` | empty | Enables the real structured CV parser and matcher pipeline. An empty value produces an honest failed scan. |
| `DEEPSEEK_BASE_URL` | `https://api.deepseek.com` | OpenAI-compatible DeepSeek endpoint. |
| `DEEPSEEK_PRIMARY_MODEL` | `deepseek-v4-flash` | First parser model attempted. |
| `DEEPSEEK_FALLBACK_MODEL` | `deepseek-v4-pro` | Fallback parser model after a retryable primary failure. |
| `ADMIN_SESSION_TTL` | `12h` | Lifetime of a revocable admin session. |
| `ADMIN_COOKIE_SECURE` | `false` locally, `true` in production | HTTPS-only session cookie flag. |
| `ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE` | `5` | Per-source-IP login attempt limit. |

Run from this directory so Go resolves backend/go.mod and LoadLocal finds .env:

```powershell
go test ./... -count=1
go vet ./...
go run ./cmd/api
```

The default bind address is local-only. Admin password provisioning is CLI-only, sessions are database-backed, and Cloudinary/DeepSeek credentials stay server-side. The scan pipeline persists only the validated structured profile and deterministic match components; it never persists raw CV or full JD content.
