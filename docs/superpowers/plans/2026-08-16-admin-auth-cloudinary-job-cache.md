# Admin authentication, Cloudinary promotions, and job-cache console plan

> Execute this plan in the existing workspace. Preserve unrelated user changes and keep all runtime data sourced from the API/database.

## 1. Establish contracts and tests first

Files:

- `backend/internal/model/admin.go`
- `backend/internal/model/job.go`
- `backend/internal/service/admin_auth.go`
- `backend/internal/service/admin_auth_test.go`
- `backend/internal/httpapi/admin_auth_handler_test.go`
- `frontend/src/lib/admin/api/admin-api.spec.ts`
- `frontend/src/lib/admin/stores/admin-auth-store.spec.ts`

Work:

- Define normalized admin/session/job response types.
- Define explicit error codes for invalid credentials, missing session, CSRF failure, expired session, and invalid page parameters.
- Write unit tests for password verification, token hashing, expiry/revocation, login response validation, and no-mock API failure behavior.
- Keep fake Cloudinary behavior behind a narrow interface so tests never call the network.

Verification:

```powershell
go test ./backend/...
bun --cwd frontend test
```

## 2. Add reversible database migration and explicit fixture

Files:

- `database/migrations/000003_admin_auth_cloudinary.up.sql`
- `database/migrations/000003_admin_auth_cloudinary.down.sql`
- `database/fixtures/development-job.sql`
- `database/README.md`
- `database/tests/validate-schema.ps1`

Work:

- Add `admin_users`, `admin_sessions`, and `admin_audit_events`.
- Add Cloudinary metadata/provider columns and constraints to `promotion_slides` while retaining legacy bytea compatibility for old rows.
- Add indexes for token lookup, expiry cleanup, job listing order, and source join.
- Make the fixture idempotent, clearly development-only, `REVIEW`/`DISABLED`, and excluded from public active-job results.
- Document migration order and explicit fixture execution.

Verification:

```powershell
powershell -File database/tests/validate-schema.ps1
```

Then apply the migration to local PostgreSQL and query table/constraint counts without printing credentials.

## 3. Implement admin authentication and bootstrap CLI

Files:

- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/internal/repository/admin.go`
- `backend/internal/repository/postgres_admin.go`
- `backend/internal/service/admin_auth.go`
- `backend/internal/service/admin_auth_test.go`
- `backend/internal/httpapi/admin_middleware.go`
- `backend/internal/httpapi/admin_auth_handler.go`
- `backend/internal/httpapi/admin_auth_handler_test.go`
- `backend/cmd/admin/main.go`
- `backend/cmd/api/main.go`
- `backend/internal/httpapi/router.go`
- `backend/internal/httpapi/middleware.go`

Work:

- Add environment-backed session/cookie/rate-limit configuration with safe local defaults and production validation.
- Implement CLI `create-user` with hidden password input, bcrypt hashing, normalized email, and no overwrite.
- Implement session token generation/hash, persistence, expiry/revocation, and active-user lookup.
- Implement login/me/logout endpoints and admin session/CSRF middleware.
- Enable credentialed CORS for explicit frontend origins only.
- Replace the temporary promotion bearer-token guard with session authorization; keep state-changing operations CSRF-protected.
- Add safe audit events and structured logs without secrets.
- Add practical comments where security invariants or compatibility behavior are non-obvious.

Verification:

```powershell
gofmt -w backend
go test ./backend/...
go vet ./backend/...
```

## 4. Integrate Cloudinary and add admin jobs API

Files:

- `backend/go.mod`
- `backend/go.sum`
- `backend/internal/cloudinary/uploader.go`
- `backend/internal/cloudinary/uploader_test.go`
- `backend/internal/repository/promotions.go`
- `backend/internal/repository/postgres_promotions.go`
- `backend/internal/service/promotion_service.go`
- `backend/internal/service/promotion_upload.go`
- `backend/internal/httpapi/promotion_handler.go`
- `backend/internal/repository/jobs.go`
- `backend/internal/repository/postgres_jobs.go`
- `backend/internal/httpapi/admin_job_handler.go`
- `backend/internal/model/job.go`

Work:

- Add the official Cloudinary Go SDK and configure it from `CLOUDINARY_URL` on the server only.
- Upload new assets with bounded reader input, unique slot/hash public IDs, and image resource type.
- Persist only validated Cloudinary metadata and content hash.
- Preserve public promotion list and same-origin image route; redirect/serve Cloudinary URL safely with cache semantics.
- Destroy replaced assets after successful DB commit and record cleanup warnings.
- Add admin promotion list response for all three slots.
- Add paginated job-cache repository query joined with source metadata; return structured fields only.
- Ensure development fixture is not visible through public active-job paths.

Verification:

```powershell
go test ./backend/...
go test -race ./backend/...
```

If race testing is unavailable on Windows, record the exact limitation and run the remaining non-race suite plus `go vet`.

## 5. Build separate admin frontend

Files:

- `frontend/src/lib/admin/api/admin-api.ts`
- `frontend/src/lib/admin/api/admin-session.ts`
- `frontend/src/lib/admin/stores/admin-auth-store.ts`
- `frontend/src/lib/admin/components/*.svelte`
- `frontend/src/routes/admin/+layout.svelte`
- `frontend/src/routes/admin/+page.svelte`
- `frontend/src/routes/admin/login/+page.svelte`
- `frontend/src/routes/admin/promotions/+page.svelte`
- `frontend/src/routes/admin/jobs/+page.svelte`
- `frontend/src/lib/shared/styles/global.css`
- frontend tests for API/store/components where appropriate

Work:

- Build login form, protected shell, sidebar/nav, logout, and responsive mobile navigation.
- Send admin requests with credentials and in-memory CSRF token; redirect to login on 401.
- Build three-slot promotion manager with actual API state, upload/delete feedback, and no placeholder images.
- Build paginated job cache table/cards with explicit empty/error/loading states and visible dev-fixture label.
- Reuse Ferris visual tokens while keeping client and admin component ownership separate.
- Add comments only for non-obvious state/security behavior, not line-by-line noise.

Verification:

```powershell
bun --cwd frontend test
bun --cwd frontend run check
bun --cwd frontend run build
```

## 6. Apply local data and run integration proof

Work:

- Apply `000003` to local PostgreSQL.
- Execute the explicit development fixture.
- Create the first admin through the CLI without printing the password.
- Start API and frontend using the real `.env`/API base URL.
- Verify login, `/me`, logout, unauthorized admin access, job listing, and promotion request validation.
- If Cloudinary connectivity is available, upload one real promotion image and verify metadata/public image flow; otherwise verify configuration and fake-adapter contract without claiming an upload succeeded.
- Capture admin desktop and mobile screenshots using the browser skill.

## 7. Final quality gates

- Run schema validator and live PostgreSQL checks.
- Run backend tests, vet, frontend tests/check/build.
- Run security/no-secret/no-mock scans.
- Run browser smoke at desktop and mobile dimensions.
- Record Baron proof receipts and quality gates.
- Run `baron autopilot review`, `baron trace score`, and a final continuity checkpoint.
- Report exact proof, limitations, and any remaining operational action (such as the admin CLI command or Cloudinary upload verification).
