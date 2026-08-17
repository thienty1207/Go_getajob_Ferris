# Promotion Carousel + Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a no-mock, max-three promotion carousel to the client and a PostgreSQL-backed, separately protected client-read/admin-write promotion API.

**Architecture:** PostgreSQL stores up to three validated promotion image slots and bounded presentation metadata. Go owns validation, slot upsert/delete, public metadata/image responses, transitional admin token authorization, and safe admin lifecycle logs. SvelteKit owns the optional promotion fetch, fallback hero copy, accessible carousel interaction, and existing CV scan flow.

**Tech Stack:** Go 1.25+, Gin, pgx/v5, PostgreSQL, Svelte 5 runes, SvelteKit, TypeScript, Vitest, Bun.

## Global Constraints

- Do not add seeded, sample, placeholder, stock, or fabricated promotion images.
- The public client reads only active promotion records and renders at most three slots ordered by `slot ASC`.
- Promotion slots are exactly `1`, `2`, and `3`; the database unique constraint is the final cap.
- Store only validated PNG, JPEG, or WebP bytes plus bounded metadata; reject SVG and unknown formats.
- Limit image request/body size with `MAX_PROMOTION_IMAGE_BYTES`, default `5 MiB`.
- Use PostgreSQL placeholders for every value; never interpolate slot or request text into SQL.
- Admin writes require `Authorization: Bearer <PROMOTION_ADMIN_TOKEN>`; when the token is unset, admin writes return a generic disabled response.
- Keep the API loopback-only until the real admin authentication/session contract exists.
- Existing scan endpoints, frontend scan types, crawler scope, job cache, DeepSeek, and matcher behavior remain unchanged.
- The current hero copy is the only no-data fallback; it is existing product copy, not promotion fixture data.
- This repository has no Git metadata, so use Baron checkpoints and trusted execution receipts rather than invented commit evidence.

---

### Task 1: Promotion database migration and schema contract

**Files:**
- Create: `database/migrations/000002_promotion_slides.up.sql`
- Create: `database/migrations/000002_promotion_slides.down.sql`
- Modify: `database/README.md`
- Test/modify: `database/tests/validate-schema.ps1`

**Interfaces:**
- Produces `public.promotion_slides` with unique integer slots `1..3`, validated image bytes, MIME, SHA-256 hash, accessible metadata, optional safe target URL, active state, and timestamps.
- The migration is additive and reversible; it does not alter scan/job tables.

- [x] **Step 1: Write the failing schema checks**

Extend `database/tests/validate-schema.ps1` with assertions for the promotion table contract: slot range and uniqueness, image MIME allowlist, lowercase 64-character hash, non-empty bounded alt text, safe target URL, active state, and reversible down migration file. The checks must fail clearly because the table and migration do not yet exist.

- [x] **Step 2: Run the schema contract and verify the expected failure**

Run:

```powershell
pwsh -NoProfile -File database/tests/validate-schema.ps1
```

Expected result: FAIL on the missing `000002` promotion migration/table contract, not a PowerShell syntax error.

- [x] **Step 3: Add the additive up/down migrations**

Create `promotion_slides` with a unique `slot SMALLINT CHECK (slot BETWEEN 1 AND 3)`, `image_bytes BYTEA NOT NULL`, `mime_type`, `content_hash`, `alt_text`, optional bounded copy fields, optional HTTP(S) target URL, `is_active`, and timestamps. Add indexes for `(is_active, slot)`. The down migration drops only this table.

- [x] **Step 4: Document migration and no-seed behavior**

Update `database/README.md` with the new migration order, the fact that the table starts empty, the client/admin endpoint split, and the transitional admin-token limitation. Do not add an example image or insert statement.

- [x] **Step 5: Re-run the schema contract**

Run the same PowerShell validator and confirm the migration checks pass while all existing privacy, score, lifecycle, and rollback checks remain green.

### Task 2: Go promotion model, config, upload validation, and repository seams

**Files:**
- Create: `backend/internal/model/promotion.go`
- Create: `backend/internal/service/promotion_upload.go`
- Create: `backend/internal/repository/promotions.go`
- Create: `backend/internal/repository/postgres_promotions.go`
- Create: `backend/internal/service/promotion_service.go`
- Modify: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`
- Test: `backend/internal/service/promotion_upload_test.go`
- Test: `backend/internal/repository/postgres_promotions_test.go`

**Interfaces:**
- `model.Promotion` contains metadata and a relative `ImageURL`; it never exposes `ImageBytes` in a public response model.
- `repository.PromotionRepository` exposes `ListActive`, `GetActiveImage`, `Upsert`, and `Delete` using context and slot-based parameters.
- `service.PromotionService` owns bounded upload parsing, image signature validation, SHA-256 hashing, safe metadata validation, and delegates persistence.
- Config adds `MaxPromotionImageBytes`, `PromotionAdminToken`, and `PromotionRateLimitPerMinute` with a 5 MiB/disabled/10-per-minute default.

- [x] **Step 1: Write failing Go tests**

Add tests for: default and configured promotion settings; slot rejection outside `1..3`; PNG/JPEG/WebP signature acceptance; SVG/unknown signature rejection; copied-byte overflow rejection; stable lowercase SHA-256 hashing; bounded alt/title/body fields; target URL rejection for credentials, JavaScript, and malformed values; repository interface behavior; SQL placeholders; and no public model field named `image_bytes` or equivalent.

- [x] **Step 2: Run focused tests and verify they fail for missing seams**

Run:

```powershell
Set-Location backend
go test ./internal/config ./internal/service ./internal/repository -run Promotion -count=1
```

Expected result: compile/test failure because the promotion types and functions do not exist yet.

- [x] **Step 3: Implement config and pure upload validation**

Add environment parsing with positive bounded byte limits and rate limits. Implement helpers that read at most `MaxPromotionImageBytes+1`, detect only PNG/JPEG/WebP signatures, calculate SHA-256 from the exact stored bytes, validate text lengths, and validate target URLs with `http`/`https` and no userinfo. Do not write uploaded bytes to a permanent filesystem path.

- [x] **Step 4: Implement model and parameterized PostgreSQL repository**

Implement SQL for active metadata list ordered by slot with `LIMIT 3`, active image lookup by slot, transactional slot upsert, and idempotent delete. Return `image_url` using the slot and content hash. Use `ETag` data from the stored hash; never scan image bytes in metadata queries. Keep raw errors internal.

- [x] **Step 5: Run focused tests and static audits**

Run:

```powershell
Set-Location backend
gofmt -w internal/config internal/model internal/service internal/repository
go test ./internal/config ./internal/service ./internal/repository -run Promotion -count=1
rg -n "fmt\.Sprintf|\+.*SELECT|image_bytes.*json|os\.WriteFile|os\.Create\(" internal/model internal/service internal/repository
```

Expected result: focused tests pass; SQL uses placeholders; production code does not persist uploads to arbitrary filesystem paths or expose image bytes in JSON.

### Task 3: Go service, public client API, protected admin API, and runtime wiring

**Files:**
- Create: `backend/internal/httpapi/promotion_handler.go`
- Modify: `backend/internal/httpapi/errors.go`
- Modify: `backend/internal/httpapi/middleware.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/README.md`
- Test: `backend/internal/httpapi/promotion_handler_test.go`
- Test: `backend/internal/httpapi/promotion_auth_test.go`
- Test: `backend/internal/service/promotion_service_test.go`

**Interfaces:**
- `GET /api/v1/client/promotions` returns `{ "promotions": [] }` with at most three active metadata records.
- `GET /api/v1/client/promotions/:slot/image` returns safe image bytes, `Content-Type`, `ETag`, `Cache-Control`, `304`, or generic `404`.
- `PUT /api/v1/admin/promotions/:slot` accepts bounded multipart `image`, required `alt`, optional bounded copy and target URL, and returns one metadata record.
- `DELETE /api/v1/admin/promotions/:slot` is idempotent and returns `204`.
- Admin routes reject missing/wrong token and return `503 promotion_admin_disabled` when no server token is configured. Token comparisons use constant-time comparison and tokens never enter logs/responses.

- [x] **Step 1: Write failing handler/auth/service tests**

Cover empty/one/three active promotion responses; four active records being capped by the repository/handler boundary; invalid slot; missing or inactive image; ETag/304; invalid content type; over-limit body; malformed metadata; disabled admin; missing admin header; wrong token; successful idempotent upsert; idempotent delete; generic database failure without raw error text; and safe structured admin action logging fields.

- [x] **Step 2: Run focused HTTP tests and verify the expected failure**

Run:

```powershell
Set-Location backend
go test ./internal/httpapi ./internal/service -run Promotion -count=1
```

Expected result: failure because promotion routes, handlers, and auth middleware are not wired.

- [x] **Step 3: Implement promotion service and public responses**

Map service/repository errors to stable public codes, serialize metadata without bytes, validate slot path parameters before repository calls, stream image bytes only after active lookup, set `ETag` from content hash, and return `304` without a body for a matching entity tag.

- [x] **Step 4: Implement admin authorization and write handlers**

Add a server-side token gate, multipart body cap before parsing, admin rate limit, upsert/delete handlers, and safe `slog` events containing only action, slot, result, and bounded error code. Use `PUT` for idempotent slot replacement and `DELETE` for idempotent removal.

- [x] **Step 5: Wire routes and runtime**

Extend router construction and `cmd/api/main.go` to create the promotion repository/service/handler. Keep `/api/v1/client/...` and `/api/v1/admin/...` route groups separate. Document `PROMOTION_ADMIN_TOKEN`, `MAX_PROMOTION_IMAGE_BYTES`, and `PROMOTION_RATE_LIMIT_PER_MINUTE` without committing secrets.

- [x] **Step 6: Run focused tests and security audit**

Run:

```powershell
Set-Location backend
gofmt -w .
go test ./internal/httpapi ./internal/service ./internal/repository -run Promotion -count=1
rg -n "err\.Error\(\)|Access-Control-Allow-Origin.*\*|fmt\.Sprintf|os\.WriteFile|image_bytes" internal/httpapi internal/service internal/repository internal/model
```

Expected result: tests pass and the audit finds no raw error response, wildcard CORS, SQL string building, arbitrary upload path, or public image-byte JSON field.

### Task 4: Frontend promotion API contract and accessible carousel

**Files:**
- Create: `frontend/src/lib/client/components/PromotionCarousel.svelte`
- Create: `frontend/src/lib/client/components/promotion-carousel.ts`
- Create: `frontend/src/lib/client/components/promotion-carousel.spec.ts`
- Modify: `frontend/src/lib/shared/types/client.ts`
- Modify: `frontend/src/lib/client/api/client-api.ts`
- Modify: `frontend/src/routes/client/+page.svelte`

**Interfaces:**
- `getPromotions(fetcher?)` in `frontend/src/lib/client/api/client-api.ts` parses the backend response into at most three `PromotionSlide` values and rejects unsafe/malformed values.
- `PromotionCarousel` receives `slides: PromotionSlide[]` and an existing-copy fallback; it owns navigation, autoplay, focus/hover pause, keyboard controls, dots, and reduced-motion behavior.
- No promotion records or image URLs are embedded in production route code.

- [x] **Step 1: Write failing frontend tests**

Test API parsing for zero/one/three records, caps a four-record response, rejects invalid URL/hash/alt/slot, and retains `imageUrl` from the real response. Test pure carousel state for previous/next wrapping, max-three normalization, autoplay pause when focused/hovered, and reduced-motion disabling autoplay.

- [x] **Step 2: Run frontend tests and verify they fail for missing modules**

Run:

```powershell
Set-Location frontend
bun run test:unit -- --run src/lib/client/api/promotions-api.spec.ts src/lib/client/components/promotion-carousel.spec.ts
```

Expected result: failure because the promotion API/types/carousel modules are not present.

- [x] **Step 3: Implement the typed promotion API parser**

Add the additive `/api/v1/client/promotions` request, strict bounded field parsing, safe HTTP(S) URL checks for `image_url` and `target_url`, lowercase hash validation, and a three-item cap. A request failure is surfaced to the route as an optional enhancement failure, not as a fabricated slide.

- [x] **Step 4: Implement the carousel component and pure state helpers**

Render an image only from `imageUrl`, overlay only escaped text, add semantic carousel labels, previous/next/dot buttons, visible focus, keyboard arrows, five-second autoplay, hover/focus pause, document-hidden pause, and reduced-motion behavior. Keep a no-data fallback using the existing hero copy.

- [x] **Step 5: Replace the static left panel without changing the scan form**

Load promotions in `onMount` in `client/+page.svelte`, preserve fallback copy on request failure, and replace only the static `hero-copy` markup. Move carousel-specific responsive styles into the component or adjacent global rules; keep the existing form, scan states, trust strip, and footer unchanged.

- [x] **Step 6: Run frontend tests and type checks**

Run:

```powershell
Set-Location frontend
bun run test:unit -- --run
bun run check
bun run build
```

Expected result: all existing and new tests pass, Svelte diagnostics are clean, and the production build exits successfully.

### Task 5: Integrated verification, review gates, and browser evidence

**Files:**
- Modify: `backend/README.md`, `database/README.md`, and affected tests/docs as needed.
- Review: all changed migration, Go, Svelte, and TypeScript files.

- [ ] **Step 1: Run full repository verification**

Run `gofmt -l .`, `go test -mod=readonly ./... -count=1`, `go vet -mod=readonly ./...`, `go mod verify`, `go mod tidy -diff`, `pwsh -NoProfile -File database/tests/validate-schema.ps1`, all frontend unit tests, `bun run check`, and `bun run build`.

- [ ] **Step 2: Run focused no-mock/privacy/security audits**

Confirm no production promotion image/data records are seeded, no image bytes are serialized in metadata JSON, no raw admin token or image bytes are logged, no SVG is accepted, no wildcard CORS or raw errors were added, and admin/client route boundaries remain distinct.

- [ ] **Step 3: Run browser smoke and screenshot proof**

Start the frontend and verify desktop and mobile fallback states, keyboard focus, one-slide and three-slide carousel fixtures in a test-only harness, form reachability, no horizontal overflow, reduced-motion behavior, and the unchanged CV upload flow. Record which states are observed and which remain unverified if PostgreSQL/API data cannot run live.

- [ ] **Step 4: Dispatch mandatory review gates**

Request independent `code-reviewer`, `security-auditor`, and `test-engineer` review over the exact changed surface. Keep admin auth/token, upload, storage, XSS, access-control, and cache behavior findings open until fixes and verification exist.

- [ ] **Step 5: Record proof, trace, and completion state**

Record trusted execution receipts, quality-gate receipts, a final continuity checkpoint, an autopilot review, and `baron trace score`. Complete the Baron plan only if trusted proof and all mandatory gates pass; report live PostgreSQL and final admin-auth unknowns explicitly.
