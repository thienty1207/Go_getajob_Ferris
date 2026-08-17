# Baron Intent Brief

- ID: `intent-97f44e38381c4d75`
- Title: database-foundation
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-15T20:22:31+07:00

## Current Behavior

Repository has a completed client frontend but no database schema or migrations; backend, crawler, and database runtime are not implemented.

## Target Behavior

A self-hosted PostgreSQL database foundation exists with versioned reversible migrations for sources, crawl runs, structured CV profiles, scans, job cache, and deterministic scan matches, without raw CV or full JD persistence and without seed/mock data.

## Scope

Create database/README.md and database/migrations/000001_initial_schema.up.sql plus 000001_initial_schema.down.sql; validate migration structure and rollback safety without implementing backend, crawler, admin UI, or production database deployment.

## Non-Goals

- Do not implement Go backend, Rust crawler, DeepSeek adapter, admin UI, authentication, production deployment, or seed/mock records.

## Constraints

- Database is self-hosted PostgreSQL; raw CV files are not stored long-term; full JD text is not stored long-term; currency remains source currency; distance uses km; only ACTIVE sources and jobs are user-visible.
- Migration must be versioned, reversible, idempotent where practical, and avoid destructive data transformation.

## Decisions

- Use normalized relational columns for queryable fields and JSONB only for structured nested education/certification data.
- Store score components with the agreed 35/25/15/15/10 weights and enforce their bounds at the database boundary.

## Required Proof

Review migration SQL, run static schema checks, verify no seed/mock data, and run available PostgreSQL migration validation if a local PostgreSQL client/server is available.

## Remaining Unknowns

- Backend authentication and scan ownership contract are not yet finalized; schema must not pretend user accounts exist.
- Migration runner and production PostgreSQL deployment topology are not yet selected.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
