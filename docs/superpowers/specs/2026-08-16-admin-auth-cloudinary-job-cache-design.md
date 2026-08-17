# Admin authentication, Cloudinary promotions, and job-cache console

## Status

Approved implementation design for the first admin slice. The user selected one-time CLI provisioning for the initial admin account and explicitly authorized implementation.

## Context

The client surface already calls the real Go API for scans and promotion metadata. The current promotion write path uses a transitional bearer token and stores image bytes in PostgreSQL. The product now needs a separate admin surface with real authentication, Cloudinary-backed promotion management, and a read-only view of crawled job-cache data.

The crawler remains source-controlled and approved-source-only. This change does not add web discovery, crawler behavior, CV parsing, DeepSeek integration, matching logic, or source-management CRUD.

## Goals

1. Provide a maintainable admin login/session boundary.
2. Provision the first admin through `go run ./cmd/admin create-user --email ...` with hidden password input.
3. Move new promotion uploads to Cloudinary while keeping the public same-origin image contract.
4. Let an authenticated admin manage the three promotion slots.
5. Let an authenticated admin inspect structured job-cache metadata with pagination.
6. Keep client and admin frontend code in separate folders and routes.
7. Add one explicit development-only job fixture for local admin verification without exposing it publicly.
8. Preserve the product privacy rules: no raw CV persistence, no full JD persistence, and no credentials in logs or frontend bundles.

## Non-goals

- Crawler implementation or source discovery.
- DeepSeek CV/job parsing.
- Deterministic CV matching implementation.
- Public job-feed changes.
- Admin source-management CRUD.
- Uploading CVs or user data to Cloudinary.
- Replacing the existing client visual language with a generic dashboard template.

## Confirmed decisions

### Admin bootstrap

The initial account is created explicitly by a CLI command:

```powershell
go run ./cmd/admin create-user --email admin@example.com
```

The command reads the password without echoing it, normalizes the email, hashes the password with bcrypt, and inserts an active admin user. It must refuse to overwrite an existing email. Passwords and hashes must not be printed or logged.

### Session model

- Login creates a cryptographically random session token.
- Only a SHA-256 hash of that token is stored in `admin_sessions`.
- The raw token is sent only as an `HttpOnly` cookie.
- The cookie is `SameSite=Lax`; `Secure` follows environment configuration and is required for production HTTPS.
- Sessions expire after a configured lifetime and can be revoked on logout.
- Admin authorization is derived from the database session and active admin row, never from a frontend flag.
- A CSRF token is generated for the session, stored hashed, returned by the authenticated `me` endpoint, and sent by the frontend in `X-CSRF-Token` on state-changing requests.
- CORS remains explicit and credentialed; wildcard origins are not permitted with credentials.

### Promotion storage

New promotion writes upload to Cloudinary from the backend using `CLOUDINARY_URL`. The API secret never reaches the browser. PostgreSQL stores the Cloudinary delivery URL and identifiers, the content hash, and presentation metadata; it does not store binary data for new uploads.

The migration is additive. The existing `image_bytes` column remains as a legacy rollback surface so an existing old row is not destroyed by the rollout. New writes use `storage_provider = 'CLOUDINARY'`, and the service refuses to create another database-bytea promotion. A later cleanup migration can remove the legacy path after an explicit data audit.

The public route remains `/api/v1/client/promotions/:slot/image`. It reads the stored Cloudinary URL and returns a controlled same-origin redirect/response so the existing client URL validation and cache contract remain intact.

Replacement uploads use a new Cloudinary public ID based on slot and content hash. The database row is updated only after upload succeeds. The previous Cloudinary asset is destroyed after the new row is committed; cleanup failure is logged as an operational warning and does not roll back a successful promotion replacement.

### Development fixture

The initial job is not inserted by a production migration and is not presented as a real crawl. It lives in an explicit fixture file and uses:

- source key `development-fixture`;
- source approval `REVIEW`;
- job status `DISABLED`;
- an unmistakable non-production company/title label;
- a reserved `.invalid` original URL;
- comments explaining that it is local verification data only.

The public active-job view must exclude it. The admin list includes it so the database-to-UI path can be verified while the crawler has no approved source yet.

## Database design

### New tables

`admin_users`

- `id uuid primary key`;
- normalized unique `email`;
- `password_hash`;
- `is_active`;
- `last_login_at`;
- `created_at`, `updated_at`.

`admin_sessions`

- `id uuid primary key`;
- `admin_user_id` with restrictive foreign key;
- unique `token_hash`;
- `csrf_token_hash`;
- `expires_at`;
- `last_seen_at`;
- nullable `revoked_at`;
- `created_at`.

Indexes cover token lookup, active expiry cleanup, and user-session lookup.

`admin_audit_events`

- `id uuid primary key`;
- nullable `admin_user_id` for events that survive account cleanup;
- action and result fields;
- resource type/slot where applicable;
- request correlation identifier where available;
- created timestamp.

The table must not contain secrets, passwords, cookies, raw CV text, or full job descriptions.

### Promotion columns

Add nullable Cloudinary metadata columns and a provider discriminator to `promotion_slides`. Add checks that a Cloudinary row has a nonblank public ID and secure URL, while retaining the legacy database-bytea shape for pre-existing rows. Add indexes only where lookup or cleanup requires them.

### Fixture

`database/fixtures/development-job.sql` is idempotent and must be run explicitly after migrations. It creates or reuses its marked source and job row without changing production migration history.

## Backend boundaries

### Packages

Use focused packages consistent with the existing layout:

```text
backend/cmd/admin/
backend/internal/auth/
backend/internal/cloudinary/
backend/internal/model/admin.go
backend/internal/model/job.go
backend/internal/repository/postgres_admin.go
backend/internal/repository/postgres_jobs.go
backend/internal/service/admin_auth.go
backend/internal/service/cloudinary_promotion.go
backend/internal/httpapi/admin_auth_handler.go
backend/internal/httpapi/admin_job_handler.go
backend/internal/httpapi/admin_middleware.go
```

Exact file names may be consolidated when an existing package boundary is clearer, but authentication, Cloudinary integration, repository code, and HTTP translation must remain independently testable.

### Authentication endpoints

`POST /api/v1/admin/auth/login`

Accepts JSON email/password. It returns a generic unauthorized error for invalid credentials, creates a session on success, sets the HttpOnly cookie, and returns the authenticated admin summary plus a CSRF token. It is rate limited independently from normal client reads.

`GET /api/v1/admin/auth/me`

Requires the session cookie. It validates expiry, revocation, and active-admin status; updates last-seen safely; and returns the admin summary and CSRF token.

`POST /api/v1/admin/auth/logout`

Requires a valid session and CSRF token, revokes the session, clears the cookie, and is safe to retry.

### Promotion endpoints

Admin promotion list, upload, and delete endpoints require the session middleware and CSRF validation. The existing public list endpoint remains unauthenticated and metadata-only.

Upload validation remains defense-in-depth at both HTTP and service layers:

- slot range `1..3`;
- bounded multipart body;
- supported PNG/JPEG/WebP signature;
- content hash;
- bounded copy fields;
- safe HTTP(S) target URL with no userinfo;
- Cloudinary response URL/identifier validation before persistence.

### Job list endpoint

`GET /api/v1/admin/jobs?page=1&page_size=20`

Returns a stable, bounded page of `job_cache` rows joined to source metadata. It includes title, company, location, role, seniority, employment type, work mode, status, source, original URL, last-seen time, and content hash. It never returns raw JD content because the schema does not persist it.

The default order is `updated_at DESC, id DESC`. Page size is capped server-side. Invalid page parameters are rejected or normalized consistently and covered by tests.

## Frontend design

Keep the current client under `src/lib/client` and `src/routes/client`. Add an independent admin tree:

```text
frontend/src/lib/admin/
├── api/admin-api.ts
├── api/admin-session.ts
├── components/AdminShell.svelte
├── components/AdminSidebar.svelte
├── components/AdminStatusMessage.svelte
├── components/PromotionManager.svelte
├── components/PromotionSlotCard.svelte
├── components/JobCacheTable.svelte
├── stores/admin-auth-store.ts
└── validation/

frontend/src/routes/admin/
├── +layout.svelte
├── +page.svelte
├── login/+page.svelte
├── promotions/+page.svelte
└── jobs/+page.svelte
```

The admin login and shell reuse Ferris tokens and typography but have an operational information hierarchy: navigation, session identity, current section, feedback, and data state. Empty states are honest and do not contain mock thumbnails or fake job rows.

The frontend sends `credentials: 'include'` for admin API calls. The CSRF token is held in the in-memory auth store and rehydrated from `/me` after a page reload. A 401 clears the store and routes to `/admin/login`.

Promotion UI has exactly three slot cards. Each card shows current Cloudinary-backed image metadata, an empty state when no image exists, upload/remove controls, and upload status. The file input is validated client-side for size/type but the backend remains authoritative.

Job Cache UI renders the actual API response, including the marked development fixture as a visible `DEV FIXTURE`/disabled state. It does not silently replace an API error with local sample rows.

## Test strategy

### Backend

- password normalization and bcrypt verification;
- session token hashing, expiry, revoke, and active-user checks;
- CSRF validation and cookie behavior;
- login rate limiting and generic auth errors;
- admin middleware authorization;
- migration/repository queries for users, sessions, audit events, promotions, and jobs;
- Cloudinary adapter request/response contract using a fake uploader at unit-test level;
- promotion replacement cleanup ordering;
- admin job pagination and field redaction;
- handler contract tests for 401/403/413/422/500 cases.

### Frontend

- admin login form validation;
- session store behavior on login, reload, 401, and logout;
- API response validation and no-fallback-to-mock behavior;
- promotion slot validation;
- job table empty/loading/error/populated states;
- `svelte-check`, unit tests, and production build.

### Integration/proof

- apply migrations to local PostgreSQL;
- run schema validator and live queries;
- create one admin through the CLI;
- run the development fixture explicitly;
- start API/frontend and verify login, logout, promotion route behavior, and job list using real HTTP;
- verify Cloudinary configuration without printing secret values;
- browser smoke screenshots at desktop and mobile widths;
- security/no-secret/no-mock audit.

## Rollback and operational notes

- Database migration has an up/down pair; legacy promotion bytes are retained during the first Cloudinary rollout.
- A failed Cloudinary upload leaves the existing database row unchanged.
- A database failure after upload triggers best-effort destruction of the newly uploaded asset and returns an error.
- A failed old-asset cleanup after a successful replacement is observable and can be retried by an operator.
- Logout/revocation is database-backed and does not depend on frontend state.
- The development fixture can be removed with its explicit fixture cleanup command/SQL without touching crawler-created records.

## Acceptance criteria

The task is complete only when:

1. The first admin can be created with the CLI without exposing a password.
2. Invalid credentials cannot access `/api/v1/admin/*`.
3. An authenticated admin can log in, reload, upload/replace/delete promotion slots, and log out.
4. New promotion writes persist Cloudinary metadata, not image bytes.
5. The client carousel still reads the real public promotion API.
6. The admin job screen reads the real PostgreSQL job cache and shows the explicit fixture only in admin.
7. No mock runtime data, raw CV, full JD, credentials, or API secret is introduced.
8. Backend, frontend, database, security, and browser verification evidence is recorded.
