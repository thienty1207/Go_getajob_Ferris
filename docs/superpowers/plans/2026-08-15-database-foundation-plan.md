# Database Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a privacy-aware, reversible self-hosted PostgreSQL schema for sources, crawl runs, structured profiles, scans, job cache, and deterministic match results without seed or mock data.

**Architecture:** The database owns durable structured data and integrity constraints. Go will later own authorization, scan state transitions, DeepSeek validation, matching calculations, and source reconciliation. This plan adds only versioned SQL migrations, a database README, and a dependency-free schema contract test.

**Tech Stack:** PostgreSQL 16-compatible SQL, `pgcrypto` UUID generation, PowerShell contract test, plain SQL migrations.

## Global Constraints

- Database is self-hosted PostgreSQL.
- Raw CV files are not stored long-term and no raw CV column is created.
- Full job descriptions, raw HTML, and crawler payloads are not stored long-term.
- Currency remains in the source currency; no conversion columns or logic are added.
- Distance is stored and queried in kilometers.
- Only `ACTIVE` sources may be crawled and only `ACTIVE` jobs may be shown by future services.
- CV Match weights are required skills 35%, role relevance 25%, experience 15%, seniority 15%, preferred skills/domain 10%.
- No seed files, mock records, sample companies, fake jobs, or fallback product data.
- Migrations are versioned, reversible, and do not perform destructive data transformation.
- Database constraints enforce allowed values and numeric bounds; domain transition logic remains in Go.

---

### Task 1: Add the database contract and operating notes

**Files:**
- Create: `database/README.md`
- Reference: `docs/superpowers/specs/2026-08-15-database-foundation-design.md`

**Interfaces:**
- Produces: migration naming convention, application/rollback commands, privacy guarantees, and the exact schema scope used by later tasks.

- [x] **Step 1: Write the database README**

  Document PostgreSQL prerequisites, the migration file order, `psql` apply/rollback commands using environment variables rather than committed credentials, the fact that no seed/mock data exists, and the required backup/staging step before a non-empty database migration.

- [x] **Step 2: Review the README against the approved design**

  Confirm the README names exactly these durable domains: `job_sources`, `source_crawl_runs`, `job_cache`, `structured_profiles`, `scans`, and `scan_matches`. Confirm it does not promise a backend, crawler, authentication, or production deployment in this task.

- [x] **Step 3: Run the documentation/forbidden-content check**

  Run:

  ```powershell
  rg -n -i "seed|mock|sample job|raw_cv|raw_jd|full description|password=" database docs/superpowers/specs/2026-08-15-database-foundation-design.md
  ```

  Expected: only intentional statements explaining that seed/mock/raw-content data is forbidden; no credential value or product record appears.

### Task 2: Write the failing schema contract test

**Files:**
- Create: `database/tests/validate-schema.ps1`
- Test target: `database/migrations/000001_initial_schema.up.sql`
- Test target: `database/migrations/000001_initial_schema.down.sql`

**Interfaces:**
- Consumes: migration SQL text.
- Produces: exit code 0 only when required schema objects, privacy omissions, integrity constraints, and rollback order are present.

- [x] **Step 1: Write assertions before the migration exists**

  The script must fail clearly when either migration file is missing. Once files exist, it must assert the presence of all six tables, the required columns, enum-like `CHECK` constraints, score bounds, unique keys, the absence of forbidden raw/mock/seed tokens in SQL data statements, and the dependency-safe down order.

- [x] **Step 2: Run the contract test to prove RED**

  Run:

  ```powershell
  pwsh -NoProfile -File database/tests/validate-schema.ps1
  ```

  Expected: non-zero exit with a message that the initial migration is missing.

### Task 3: Create the PostgreSQL up migration

**Files:**
- Create: `database/migrations/000001_initial_schema.up.sql`

**Interfaces:**
- Produces: PostgreSQL objects consumed later by Go repositories and the Rust crawler.

- [x] **Step 1: Create extension and shared timestamp/UUID defaults**

  Enable `pgcrypto` with `CREATE EXTENSION IF NOT EXISTS pgcrypto;` and use `gen_random_uuid()` defaults. Do not create application roles or commit credentials.

- [x] **Step 2: Create `job_sources` and `source_crawl_runs`**

  Add source approval fields, source URLs, access mode, crawl run status, counters, timestamps, foreign keys, and indexes. Restrict source approval to `REVIEW`, `ACTIVE`, and `DISABLED`; restrict crawl outcomes to `HEALTHY`, `SOURCE_ERROR`, `PARSER_ERROR`, and `ANOMALY`.

- [x] **Step 3: Create `job_cache`**

  Add source-scoped job identity, `content_hash`, structured matching fields, source-preserving salary fields, original URL, lifecycle status, coordinates, missing healthy-cycle counter, and indexes. Do not add raw JD, description, HTML, or response-payload columns.

- [x] **Step 4: Create `structured_profiles` and `scans`**

  Add only the approved CV schema fields and scan query/state fields. Use `jsonb` for education/certifications, check that arrays are not null, require positive radius, constrain coordinates, and include bounded-retention timestamps.

- [x] **Step 5: Create `scan_matches`**

  Add one unique row per scan/job pair, distance in kilometers, five weighted score components, total `match_percent`, foreign keys, and checks for 35/25/15/15/10 maxima and 0–100 total range.

- [x] **Step 6: Add comments and indexes without hidden lifecycle triggers**

  Comment privacy-sensitive omissions and public fields. Add indexes for source status, active jobs, content hash, scan status, expiry, and match ordering. Do not add triggers that silently change job or scan states.

- [x] **Step 7: Run the schema contract test and inspect the diff**

  Run the contract test from Task 2. Expected: PASS and no forbidden data inserts.

### Task 4: Create the reversible down migration

**Files:**
- Create: `database/migrations/000001_initial_schema.down.sql`

**Interfaces:**
- Consumes: objects created by the up migration.
- Produces: a clean rollback that does not remove the shared `pgcrypto` extension.

- [x] **Step 1: Drop dependent objects in reverse order**

  Drop `scan_matches`, `scans`, `structured_profiles`, `job_cache`, `source_crawl_runs`, and `job_sources` with `IF EXISTS` in that exact dependency-safe order.

- [x] **Step 2: Run static rollback-order assertions**

  Run:

  ```powershell
  pwsh -NoProfile -File database/tests/validate-schema.ps1
  ```

  Expected: PASS, including the assertion that the down migration does not drop `pgcrypto` and does not drop a parent table before its dependents.

### Task 5: Run final database verification

**Files:**
- Verify: `database/README.md`
- Verify: `database/migrations/000001_initial_schema.up.sql`
- Verify: `database/migrations/000001_initial_schema.down.sql`
- Verify: `database/tests/validate-schema.ps1`

**Interfaces:**
- Produces: evidence for schema structure, privacy boundaries, no-mock compliance, and migration rollback ordering.

- [x] **Step 1: Run the schema contract test**

  Run:

  ```powershell
  pwsh -NoProfile -File database/tests/validate-schema.ps1
  ```

  Expected: exit code 0 with all assertions passing.

- [x] **Step 2: Run forbidden-token and seed audit**

  Run:

  ```powershell
  rg -n -i "INSERT\s+INTO|raw_cv|raw_jd|raw_html|full_description|mock|sample|fixture" database
  ```

  Expected: no SQL data inserts and no forbidden persistence columns. Documentation may mention forbidden terms only as policy text.

- [x] **Step 3: Run available PostgreSQL validation when tooling exists**

  Run:

  ```powershell
  if (Get-Command psql -ErrorAction SilentlyContinue) {
    Write-Output "psql available: apply up migration to an empty staging database, inspect \dt and \d, then apply down migration"
  } else {
    Write-Output "psql unavailable: static migration contract is the available local proof"
  }
  ```

  Expected in the current workspace: the explicit unavailable-tool message; do not claim live PostgreSQL execution without a PostgreSQL client/server receipt.

- [x] **Step 4: Record the remaining integration boundary**

  Document that Go repository integration, authentication/scan ownership, crawler reconciliation, DeepSeek validation, and live PostgreSQL execution remain follow-up work outside this database-only tranche.
