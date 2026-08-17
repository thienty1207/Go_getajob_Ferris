# Baron Intent Brief

- ID: `intent-5581890c7ec6b4f4`
- Title: admin-auth-cloudinary-job-cache
- Risk: `high`
- Confirmation: `confirmed`
- Updated: 2026-08-16T11:23:43+07:00

## Current Behavior

The client promotion carousel is backed by a temporary bearer-token promotion write guard and PostgreSQL bytea image storage. There is no admin login/session boundary, no admin UI, no Cloudinary integration, and no admin job-cache listing endpoint. The database currently has no job rows.

## Target Behavior

A separate admin application surface authenticates admins with a one-time CLI-provisioned bcrypt account, server-side sessions, CSRF protection, and audited state changes. Promotion uploads go through the Go API to Cloudinary and only Cloudinary metadata is stored for new uploads. Admins can manage at most three promotion slots and inspect structured job-cache rows. One explicit development-only disabled fixture job is available for local verification and is excluded from public client results.

## Scope

backend authentication repository/service/handlers/middleware and CLI; promotion storage migration and Cloudinary adapter; admin jobs API; admin Svelte routes/components/API/session store; reversible database migration; explicit development fixture and local verification.

## Non-Goals

- Do not implement crawler behavior, DeepSeek parsing, CV parsing, matcher scoring, source management CRUD, or public client job feed changes.

## Constraints

- Use PostgreSQL on localhost; preserve client/admin folder separation; no raw CV or full JD persistence; do not print credentials; keep existing public promotion response contract; Cloudinary secret remains backend-only; development fixture must be clearly marked and never public.
- Use practical maintenance comments for non-obvious auth, storage, migration, and security invariants.

## Decisions

- Provision the first admin through go run ./cmd/admin create-user --email ... with hidden password input; use bcrypt password hashes, hashed random session tokens in HttpOnly cookies, in-memory CSRF token on the frontend, and explicit credentialed CORS origins.
- Use additive promotion storage columns with legacy bytea retained for reversible rollout; all new promotion writes use Cloudinary and public image reads keep the same-origin API path.

## Required Proof

Go unit/integration tests; frontend unit/typecheck/build; migration validator and live PostgreSQL migration/query checks; auth/promotion/job API smoke tests; Cloudinary configuration validation without exposing secrets; browser smoke screenshots for admin desktop/mobile; security scan; no-mock audit; trace and quality-gate receipts.

## Remaining Unknowns

- Cloudinary SDK exact upload reader API and current local Cloudinary connectivity must be verified during implementation; production secret manager and TLS deployment settings remain environment concerns.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
