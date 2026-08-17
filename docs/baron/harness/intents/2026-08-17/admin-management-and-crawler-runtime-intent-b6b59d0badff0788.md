# Baron Intent Brief

- ID: `intent-b6b59d0badff0788`
- Title: admin-management-and-crawler-runtime
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-17T18:57:48+07:00

## Current Behavior

Admin Job Link and Job Location lists are not consistently paginated; Job Cache has no search or inline location assignment, renders duplicate lifecycle badges, and crawler settings expose no persisted runtime heartbeat or next-cycle evidence.

## Target Behavior

Admin Job Link, Job Location, and Job Cache use server-side pagination capped at 10; Job Cache searches and assigns canonical locations inline; sidebar groups Manage Job and Manage User; Settings shows truthful crawler online/running/offline state and next-cycle countdown backed by PostgreSQL runtime state.

## Scope

PostgreSQL migration, Go admin API/repositories/services/handlers, Rust crawler runtime heartbeat/scheduler state, Svelte admin Job Link/Job Location/Job Cache/Settings/sidebar UI and tests.

## Non-Goals

- No mock data, no raw JD storage, no source-specific crawler adapter, no public client settings, no arbitrary crawl scope expansion in this tranche.

## Constraints

- Page size maximum is 10; job_cache.location_id remains the canonical assignment; Job Cache owns assignment UI; crawler countdown is derived from persisted next_cycle_at and heartbeat, not a browser-only timer.

## Decisions

- Keep PATCH /admin/jobs/:id/location contract and move its UI usage into Job Cache; add paged Admin Location response while preserving unpaged public client locations; use a singleton crawler runtime row for heartbeat and next-cycle state.

## Required Proof

Fresh Go tests and vet, Rust tests/check, frontend bun check/test/build, migrated PostgreSQL contract and authenticated API smoke, desktop/mobile screenshots or browser smoke for changed admin surfaces, Baron trace score.

## Remaining Unknowns

- Exact production service-manager deployment for the Rust daemon remains outside this tranche.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
