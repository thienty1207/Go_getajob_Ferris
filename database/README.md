# Database

This directory contains the versioned PostgreSQL schema for Go Get a Job, Ferris!

The database is self-hosted PostgreSQL. Schema migrations create tables, constraints, indexes, comments, and the singleton crawler-runtime row. A separate, explicitly named development fixture can be run manually to verify the admin Job Cache screen. It is marked `REVIEW`/`DISABLED` and is never public.

## Scope

The initial migration owns these durable domains:

- `job_sources`: reviewed source registry and crawl permission evidence.
- `source_crawl_runs`: source health and crawl-cycle accounting.
- `job_cache`: structured job metadata, matching fields, source currency, and content hash.
- `locations` and `location_aliases`: admin-approved canonical location keys and source-text aliases used for deterministic resolution.
- `structured_profiles`: validated CV profile fields only; no raw CV data.
- `scans`: client scan request, location/radius, lifecycle, and retention timestamps.
- `scan_matches`: deterministic score components, total CV Match %, and distance in km.
- `promotion_slides`: at most three validated admin-managed promotion images and bounded presentation metadata; new uploads store Cloudinary metadata, not binary image bytes.
- `admin_users`, `admin_sessions`, and `admin_audit_events`: bcrypt-backed admin identity, hashed/revocable sessions, CSRF digest, and safe operational audit events.
- `app_settings`: typed, database-backed product settings owned by the admin control room.
- `crawl_requests`: idempotent manual Crawl Now requests consumed by the Rust daemon.
- `crawler_runtime`: singleton heartbeat, cycle status, current source, and next-cycle evidence for the admin runtime view.
- `home_sections` and `home_section_media`: four fixed admin-managed Home slots and ordered Cloudinary media metadata for the fourth slot.

The migrations do not implement the Go API, crawler, DeepSeek integration, admin UI, job status transitions, or production deployment. Those consumers must use this schema without introducing raw CV/full-JD persistence. The promotion API keeps client reads under `/api/v1/client/promotions` and cookie-authenticated admin operations under `/api/v1/admin/promotions`.

Migration `000004_job_link_constraints` adds the database-level uniqueness
guard for normalized enabled Job Links. A disabled source row remains in the
history and can be re-approved later without duplicating an enabled crawl
boundary.

The first Go API lives in `backend/`. Apply this migration before starting it, then configure `DATABASE_URL` in the local shell or a secret manager. The API reads public matches through `active_job_cache`, stores scan lifecycle metadata in `scans`, and does not add raw CV/full JD columns.

## Prerequisites

- PostgreSQL 16 or a compatible self-hosted PostgreSQL version.
- `psql` available on the machine running migrations.
- A database backup before applying the migration to a non-empty environment.
- A staging/empty database migration rehearsal before production use.

The migration uses the PostgreSQL `pgcrypto` extension for `gen_random_uuid()`. It creates the extension if it is absent and deliberately does not remove it during rollback because the extension may be shared by other database objects.

## Migration order

Apply files in their numeric order:

```text
database/migrations/000001_initial_schema.up.sql
database/migrations/000002_promotion_slides.up.sql
database/migrations/000003_admin_auth_cloudinary.up.sql
database/migrations/000004_job_link_constraints.up.sql
database/migrations/000005_canonical_locations.up.sql
database/migrations/000006_location_resolution_and_scan_location.up.sql
database/migrations/000007_location_key_normalization.up.sql
database/migrations/000008_settings_and_crawl_requests.up.sql
database/migrations/000009_admin_management_runtime.up.sql
database/migrations/000010_location_assignment_source.up.sql
database/migrations/000011_client_auth.up.sql
database/migrations/000012_client_cv_history.up.sql
database/migrations/000013_home_sections.up.sql
```
The matching down migrations are only for controlled rollback:

```text
database/migrations/000013_home_sections.down.sql
database/migrations/000012_client_cv_history.down.sql
database/migrations/000011_client_auth.down.sql
database/migrations/000010_location_assignment_source.down.sql
database/migrations/000009_admin_management_runtime.down.sql
database/migrations/000008_settings_and_crawl_requests.down.sql
database/migrations/000007_location_key_normalization.down.sql
database/migrations/000006_location_resolution_and_scan_location.down.sql
database/migrations/000005_canonical_locations.down.sql
database/migrations/000004_job_link_constraints.down.sql
database/migrations/000003_admin_auth_cloudinary.down.sql
database/migrations/000002_promotion_slides.down.sql
database/migrations/000001_initial_schema.down.sql
```

Run the `000009` and `000008` down migrations before the location and initial down migrations when rolling back the complete schema. Do not run either down migration against a database containing data unless the operator has an approved backup and understands which tables will be removed.

## Apply with `psql`

Set connection variables in the current shell or through the deployment secret manager. Do not commit credentials to this repository.

```powershell
$env:PGHOST = '127.0.0.1'
$env:PGPORT = '5432'
$env:PGDATABASE = 'gogetsomefoodferris'
$env:PGUSER = 'gogetsomefoodferris_app'
# Set PGPASSWORD through a local secret manager or an ephemeral shell value.

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

# Only for local admin-screen verification; this is not crawler output.
psql --set ON_ERROR_STOP=1 --file database/fixtures/development-job.sql
```

For the local PostgreSQL installation used by this project, create the empty
application database once before applying migrations. The command is safe to
repeat only after checking that the database does not already exist:

```powershell
createdb --host 127.0.0.1 --port 5432 --username postgres gogetsomefoodferris
```

The Go backend's `LoadLocal` path uses `gogetsomefoodferris` as the default
database name when `backend/.env` contains only `port`, `username`, and
`password`. Set `database=` explicitly when working with another local
database.

For a remote database, use the deployment platform's secret store for `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, and `PGPASSWORD` rather than putting a connection string in source control.

## Roll back the initial schema

Only run these after a backup and an explicit rollback decision:

```powershell
psql --set ON_ERROR_STOP=1 --file database/migrations/000013_home_sections.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000012_client_cv_history.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000011_client_auth.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000010_location_assignment_source.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000009_admin_management_runtime.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000008_settings_and_crawl_requests.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000007_location_key_normalization.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000006_location_resolution_and_scan_location.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000005_canonical_locations.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000004_job_link_constraints.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000003_admin_auth_cloudinary.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000002_promotion_slides.down.sql
psql --set ON_ERROR_STOP=1 --file database/migrations/000001_initial_schema.down.sql
```

The initial rollback drops dependent tables first (`scan_matches`, `scans`, `structured_profiles`, `job_cache`, `source_crawl_runs`, `job_sources`) and leaves `pgcrypto` installed. The promotion rollback drops only `promotion_slides`.

The location-key normalization down migration is a controlled data rollback;
run it before the location-resolution down migration. The location-resolution
down migration must run before the canonical-location down migration because
it removes `scans.location_id`, `location_aliases`, and the lookup constraints
added by migration `000006`.

Migration `000009_admin_management_runtime` must be rolled back before
`000008_settings_and_crawl_requests`; both must be rolled back before the
location and initial schema migrations. The runtime migration removes only
crawler observability state. The settings migration removes the admin settings
registry and pending manual crawl requests; hard-deleting a Job Link cascades
its queued requests.

## Privacy invariants

- No raw CV column, raw CV path, raw HTML, full job description, or source response payload is created.
- The structured profile contains only `roles`, `skills`, `years_of_experience`, `seniority`, `domains`, `education`, and `certifications`.
- Education and certification arrays are checked by `is_structured_record_array`: at most 20 scalar records, only approved schema keys, and no string value longer than 512 characters.
- Name, email, phone, address, and photo are not part of the profile schema.
- Job salary values stay in the source currency. The schema has no converted-currency field.
- Distances are kilometers.
- The backend must delete the temporary CV file after parsing and must enforce access control for scan IDs.
- New promotion uploads store Cloudinary public ID, asset ID, secure URL, and content hash. The legacy `image_bytes` column remains only as a reversible rollout surface; new runtime writes must not use it.
- The crawler interval is stored in `app_settings` as a validated JSON number of seconds. `.env` remains for connection aliases and secrets.
- `crawl_requests` contains only request state and source-run linkage; it never stores fetched source content or raw job descriptions.
- `database/fixtures/development-job.sql` is the only local job fixture and is not a public source or crawler result.
- Promotion target URLs are restricted to HTTP(S) without embedded credentials, and SVG is not an accepted image format.

Public job reads must use the `active_job_cache` view. It joins `job_cache` with `job_sources` and returns rows only when both the job and its approved source are `ACTIVE`; future backend repositories must not treat the base `job_cache` table as the public-read contract.

## Local verification

Run the dependency-free contract check from the repository root:

```powershell
pwsh -NoProfile -File database/tests/validate-schema.ps1
```

If a PostgreSQL server and `psql` are available, apply the up migration to an empty staging database, inspect the resulting tables and constraints, then apply the down migration. The contract check remains required because it also verifies the no-seed/no-raw-data policy in source files.
