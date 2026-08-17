# Baron Intent Brief

- ID: `intent-f9f46baab88a9643`
- Title: backend-env-database-comment-finalization
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-16T09:25:24+07:00

## Current Behavior

Backend required DATABASE_URL, did not load backend/.env, local PostgreSQL migrations were not applied, and production Go functions had sparse explanatory comments.

## Target Behavior

Backend loads local backend/.env without allowing generic OS variables to override database fields, connects to the project PostgreSQL database, migrations are applied and verified, and important backend/database boundaries explain their invariants for maintenance.

## Scope

backend config loader, Go comments in backend production code, PostgreSQL database creation and migrations, local API smoke verification, and related documentation.

## Non-Goals

- Do not add promotion seed data or mock runtime images; do not build the admin UI; do not change crawler, matcher, or DeepSeek behavior.

## Constraints

- Keep backend/.env local and ignored; never print credentials; keep API loopback-only; admin promotion writes remain disabled unless a server-side token is explicitly configured.

## Decisions

- Use DATABASE_URL when supplied; otherwise LoadLocal reads .env/backend/.env and builds a PostgreSQL URL from local fields with project database default gogetsomefoodferris.

## Required Proof

config tests, Go tests, go vet, gofmt, schema validator, live PostgreSQL schema queries, API health/public/admin smoke, secret/log scan, and explicit race-test limitation.

## Remaining Unknowns

- Real admin authentication/session contract remains undefined; race detector requires GCC; production deployment secret manager is not configured.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
