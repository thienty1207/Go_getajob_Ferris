# Baron Intent Brief

- ID: `intent-2d9cbf9aabb8b879`
- Title: database-recovery-and-api-readiness
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-20T19:38:00+07:00

## Current Behavior

PostgreSQL rejects connections with recovery-mode errors; API auth/me can surface a generic 500 when session lookup cannot reach storage; database schema may be behind current code

## Target Behavior

Backend and crawler fail or degrade clearly when PostgreSQL is unavailable, protected admin routes return a stable 503 database_unavailable response, and the live database schema is verified against the current migrations before runtime smoke testing

## Scope

PostgreSQL readiness, Go database error classification, admin auth error mapping, migration verification, and runtime smoke checks

## Non-Goals

- No data reset, no force-kill of PostgreSQL, no unrelated UI changes, no secret rotation

## Constraints

- Preserve existing data; use the repository's Go/Svelte/Rust boundaries; never expose raw database errors to clients

## Decisions

- Treat recovery mode as a transient dependency outage; require an elevated service restart and apply only missing migrations after read-only schema inspection

## Required Proof

Go focused tests and vet, PostgreSQL pg_isready and schema queries, backend health/auth smoke, crawler runtime log, and diff review

## Remaining Unknowns

- Exact missing migration set until PostgreSQL accepts read-only queries

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
