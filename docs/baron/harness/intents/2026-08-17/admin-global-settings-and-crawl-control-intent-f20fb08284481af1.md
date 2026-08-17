# Baron Intent Brief

- ID: `intent-f20fb08284481af1`
- Title: admin-global-settings-and-crawl-control
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-17T16:32:47+07:00

## Current Behavior

Job Link UI has no immediate crawl request action, crawler interval is loaded from local configuration, and no authenticated Admin Settings surface exists for runtime product settings.

## Target Behavior

Authenticated admins can queue a real Crawl Now request per ACTIVE Job Link, observe pending/running/completed/result status, and edit a database-backed crawler interval in hours and minutes; the Rust daemon reloads that setting while running and periodically crawls all ACTIVE Job Links.

## Scope

PostgreSQL migration, Go settings and crawl-request API/repositories/services/handlers, Rust request queue and runtime scheduler, Svelte admin Settings page, Job Link Crawl Now UI, tests and docs.

## Non-Goals

- No mock jobs, no unrestricted web crawl, no site-specific adapter, no raw CV/JD persistence, no public client settings, no arbitrary secret storage in the settings table.

## Constraints

- Only ACTIVE approved Job Links are crawlable; crawler remains allowlist-only and fail-closed; manual requests remain pending when crawler is offline; source/parser errors never count as missing; interval is stored in seconds with an hours/minutes UI; .env remains for secrets and connection aliases.

## Decisions

- Use a PostgreSQL-backed request queue with idempotent pending/running dedupe; use a typed settings registry with crawler interval as the first key; reload the interval at runtime without restarting the daemon; run a full ACTIVE-source cycle immediately on daemon startup and then according to the configured interval.

## Required Proof

Fresh Go tests and vet, Rust tests/check, frontend bun check/test/build, schema contract, PostgreSQL settings/request smoke, authenticated API contract tests, and Baron trace score.

## Remaining Unknowns

- Exact future setting keys and production service-manager deployment are not in scope for this tranche.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
