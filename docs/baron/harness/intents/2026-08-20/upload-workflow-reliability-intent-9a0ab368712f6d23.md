# Baron Intent Brief

- ID: `intent-9a0ab368712f6d23`
- Title: upload-workflow-reliability
- Risk: `high`
- Confirmation: `confirmed`
- Updated: 2026-08-20T20:44:55+07:00

## Current Behavior

PostgreSQL and health endpoints are back online, but admin Home section image saves previously failed with a generic unavailable error and the authenticated client CV workflow lacks fresh end-to-end proof after the outage.

## Target Behavior

Admin Home section images upload through authenticated multipart requests to Cloudinary and persist metadata in PostgreSQL, while authenticated CV uploads are validated, processed, cleaned up, stored only as structured profiles, and return deterministic location-scoped matches.

## Scope

Admin Home section upload boundary, Cloudinary idempotency and metadata persistence, client CV multipart upload, temporary-file cleanup, DeepSeek parse validation, structured history persistence, deterministic matching, frontend error handling, and live-safe runtime verification.

## Non-Goals

- No mock UI data, no raw CV retention, no crawler redesign, no arbitrary CMS fields, no database reset, and no unrelated visual redesign.

## Constraints

- Preserve existing user data and dirty working-tree changes; do not expose secrets or PII; keep admin and client sessions separate; enforce CSRF and bounded PNG/JPEG/WebP plus PDF/DOCX/TXT uploads.

## Decisions

- Diagnose each component boundary first; add regression tests before production changes; use a temporary local test identity only if required and remove it after verification; external provider tests must clean up created assets.

## Required Proof

Red-green regression tests, full Go tests and vet, frontend unit/check/build, schema contract, focused upload security review, live PostgreSQL/API smoke, Cloudinary create-and-delete proof, and authenticated CV workflow proof up to the limits of an available real CV and provider credentials.

## Remaining Unknowns

- Whether the current Home failure is stale database-outage state, a frontend timeout, Cloudinary duplicate public-ID behavior, or a storage response; whether a real CV fixture is locally available for a live DeepSeek happy path.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
