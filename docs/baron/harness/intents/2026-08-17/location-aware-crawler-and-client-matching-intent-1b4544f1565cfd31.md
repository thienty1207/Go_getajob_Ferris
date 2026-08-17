# Baron Intent Brief

- ID: `intent-1b4544f1565cfd31`
- Title: location-aware-crawler-and-client-matching
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-17T14:03:18+07:00

## Current Behavior

Crawler writes structured job_cache rows with optional canonical location; client scan accepts free-text location; admin Location and Job Cache pages are not linked by a filter; CV parser/matcher is unavailable.

## Target Behavior

A generic active-Job-Link crawler resolves source location text to canonical locations when deterministic, preserves unresolved values without guessing, and the client submits a canonical location_id so real matching filters active jobs by location/radius before deterministic CV scoring; admin Location and Job Cache pages are linked by stable filters.

## Scope

database migrations, backend Go repositories/services/handlers, Rust crawler persistence and resolver, frontend admin/client API and pages, and related tests/docs

## Non-Goals

- No site-specific crawler adapter; no unrestricted web crawl; no mock jobs or fabricated matches; no raw CV or full JD persistence; no client authentication implementation.

## Constraints

- Vietnam first; kilometers; source currency unchanged; active approved Job Links only; missing-job lifecycle requires two healthy cycles; CV Match weights remain deterministic 35/25/15/15/10; raw CV is temporary.

## Decisions

- Use canonical locations plus deterministic aliases; unknown locations stay unresolved; Remote jobs are location-independent; Onsite and Hybrid use location/radius; preserve existing endpoints where compatible.

## Required Proof

fresh Go tests, Rust tests and check, frontend bun test/check/build, schema validation, PostgreSQL integration or smoke evidence for location assignment and client contract, and a Baron trace score

## Remaining Unknowns

- Exact production source formats beyond structured JobPosting; multi-location job policy after the first single-location implementation; live DeepSeek availability and real provider credentials.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
