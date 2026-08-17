# Baron Intent Brief

- ID: `intent-2af67af9df04ed32`
- Title: job-link-location-crawler-promotion-fixes
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-17T07:59:28+07:00

## Current Behavior

Job Link UI is present but create is unreachable from the dev frontend, delete only changes approval_status to DISABLED, location screen has no API or table, and Rust crawler still uses NoopAdapter; client promotion carousel has a bordered non-16:9 frame.

## Target Behavior

Authenticated admin can create, edit, pause, resume, and hard-delete Job Links; admins can create canonical Vietnam locations and assign them to job-cache rows; crawler loads the local environment, crawls only approved Job Links through a reviewed real adapter, persists structured authoritative observations and reconciliation state; client promotion artwork renders without an outer frame at a 16:9 1920x1080 presentation ratio.

## Scope

Backend API, additive PostgreSQL migrations, Rust crawler, admin Job Link and Location surfaces, client promotion carousel.

## Non-Goals

- No raw JD or CV persistence, no mock job rows, no generic unrestricted web spider, no DeepSeek integration in this tranche, no automatic source activation without Job Link approval.

## Constraints

- Preserve source-controlled allowlist-only crawling, robots and DNS safety, deterministic match semantics, source currency, canonical location_id authority, and admin cookie/CSRF auth.

## Decisions

- Hard delete is a distinct DELETE operation; pause/resume is an explicit status operation. Development uses Vite API proxy; production uses environment/service configuration. The first adapter must only run for a reviewed Job Link source and use the source's public structured endpoint or job detail schema.

## Required Proof

Go tests, Rust tests, migration/schema checks, frontend tests/build, API smoke with local PostgreSQL, crawler smoke against an explicitly approved source if permission and source contract are confirmed, and browser/viewport proof for promotion UI.

## Remaining Unknowns

- Whether the current local PostgreSQL has all migrations applied; whether FPTJobs has crawl/cache permission sufficient for a production adapter; exact source adapter contract after source review.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
