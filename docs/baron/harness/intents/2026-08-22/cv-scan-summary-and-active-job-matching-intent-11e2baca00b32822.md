# Baron Intent Brief

- ID: `intent-11e2baca00b32822`
- Title: CV scan summary and active-job matching
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-22T19:23:50+07:00

## Current Behavior

Scan results show an inline loading card, no persisted CV summary, and matching can over-score unrelated roles; closed jobs must not reach client results.

## Target Behavior

Scan results show a compact phase-based loading modal, a bounded structured CV summary above active job matches, and deterministic role-aware scores with closed jobs excluded server-side.

## Scope

backend database DeepSeek parser matching repository client scan API and scan results UI

## Non-Goals

- do not persist raw CV text or PII; do not add UI help text; do not hide active jobs by an arbitrary threshold unless separately approved

## Constraints

- preserve fixed weights 35/25/15/15/10; use actual provider response; no mock job/profile data

## Decisions

- show every eligible ACTIVE job with a low deterministic score when role evidence is weak

## Required Proof

unit tests backend and frontend, migration/schema checks, real local API PostgreSQL DeepSeek test with supplied PDF, raw CV cleanup check, and production build

## Remaining Unknowns

- exact summary wording from provider is bounded by schema validation

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
