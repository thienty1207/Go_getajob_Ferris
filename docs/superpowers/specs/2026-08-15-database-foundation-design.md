# Database Foundation Design

**Date:** 2026-08-15  
**Status:** Approved by product owner in conversation  
**Scope:** PostgreSQL schema and reversible migrations only

## Goal

Create the first self-hosted PostgreSQL foundation for Go Get a Job, Ferris! so the future Go API and Rust crawler have one explicit, privacy-aware source of truth for approved job sources, crawl health, structured CV profiles, scans, cached jobs, and deterministic match results.

## Current and target state

The repository currently contains a completed client frontend and its API client contract, but no database schema, migration, backend repository, crawler, or runtime database. The target is a versioned PostgreSQL migration that can be applied to an empty database and rolled back in dependency order without inserting product data.

This tranche does not deploy PostgreSQL, implement the Go API, or create crawler/admin behavior. It establishes the storage contract those components will consume.

## Chosen approach

Use normalized PostgreSQL tables for fields that the backend must filter, join, constrain, or index. Use `jsonb` only for nested structured values whose internal shape is owned by the validated CV/job schema, such as education and certifications. Store no raw CV content, full job description, raw HTML, or source response payload.

Use one additive `000001_initial_schema.up.sql` migration and one dependency-ordered `000001_initial_schema.down.sql` rollback. Business state transitions remain in Go services; SQL constraints enforce allowed values and numeric invariants without hidden triggers.

## Tables and ownership

### `job_sources`

The source registry is the crawler/admin boundary. A source is crawlable only when `approval_status = 'ACTIVE'`. The table records the approved source identity and permission evidence URLs without assuming that an unreviewed public page may be crawled.

### `source_crawl_runs`

Each source run records whether the observation was healthy, a source failure, a parser failure, or an anomaly. Missing-job counters may only advance after a healthy run; the migration deliberately does not encode that transition as a trigger.

### `job_cache`

This is the durable structured job cache. `source_id + source_job_key` is the source-scoped identity. `content_hash` detects changes so DeepSeek is only called for new or changed jobs. The table stores public metadata and structured matching fields, not the full JD.

### `structured_profiles`

This table stores only the validated CV schema: roles, skills, years of experience, seniority, domains, education, and certifications. Education and certification arrays are limited to structured scalar records with an allowlist of keys and a bounded string length; arbitrary nested payloads are rejected by a database-side immutable validator. The raw upload is outside this schema and must be deleted by the future backend after parsing. `expires_at` allows retention to be enforced when anonymous scan policy is finalized.

### `scans`

This table stores a user's scan request and processing state, including the location/radius query needed for matching. `profile_id` may be empty while a scan is `RECEIVED` or `PARSING`, then becomes required before `MATCHING` or `COMPLETED`. It intentionally does not create a user/account table because authentication and ownership contracts are not finalized. UUID scan IDs are non-sequential; the future API must still enforce ownership/access control rather than treating an ID as a permission.

### `scan_matches`

This table stores one deterministic score per scan/job pair, including the five agreed weighted components and a distance snapshot in kilometers. Database checks enforce each component's maximum and the total range; the Go matcher owns the formula and sorting.

## Privacy and retention

- There is no raw CV column, raw CV file path, raw JD column, full description column, or raw crawler payload column.
- Structured profile JSON is schema data, not a copy of the original CV. It must be redacted before persistence by the future Go parser.
- Contact and identity fields such as name, email, phone, address, and photo are intentionally absent.
- `expires_at` on profiles and scans gives the backend a bounded-retention hook while the final consent/auth policy remains unknown.
- Salary is stored as source values and source display text; no currency conversion is represented in the schema.

## Job lifecycle support

The schema supports the agreed states `ACTIVE`, `VERIFYING`, `CLOSED`, `EXPIRED`, and `DISABLED`. `missing_healthy_cycles` is bounded and indexed for reconciliation. Public job reads must use the `active_job_cache` view, which joins job status with source approval and exposes only jobs whose job and source are both `ACTIVE`. The backend/crawler must implement:

1. Explicit source closure moves a job to `CLOSED` immediately.
2. One missing observation in a healthy crawl moves an active job to `VERIFYING`.
3. Two consecutive missing observations in healthy crawl cycles move it to `CLOSED`.
4. Source errors, HTTP failures, parser failures, and anomalies do not increment missing cycles.

## Match score contract

The stored components map exactly to the product weights:

| Component | Maximum points |
| --- | ---: |
| Required skills | 35 |
| Role relevance | 25 |
| Experience | 15 |
| Seniority | 15 |
| Preferred skills/domain | 10 |
| Total | 100 |

The database accepts decimal points for deterministic rounding, but every component and total is constrained to its agreed range. The frontend displays `match_percent` as CV Match %, not as a hiring probability.

## Migration and rollback

The `up` migration creates `pgcrypto` if needed for UUID generation and then creates tables, constraints, indexes, and comments. The rollback drops dependent tables before their parents and intentionally leaves the shared `pgcrypto` extension installed; an operator may remove it separately only when the database has no other dependency on it.

There is no existing schema or legacy data, so no expand/migrate/contract window or data backfill is required for this initial migration. Before applying to a non-empty environment, the operator must take a PostgreSQL backup and run the migration against staging first.

## Verification

The repository will include a dependency-free PowerShell schema contract test that verifies required tables/columns/constraints, the absence of raw/mock/seed payloads, and rollback order. If a PostgreSQL client/server becomes available, the migration must additionally be applied to an empty database and rolled back with `psql`; the current development environment has no `psql`, `pg_isready`, Docker, or Podman executable, so static verification is the available proof for this tranche.
