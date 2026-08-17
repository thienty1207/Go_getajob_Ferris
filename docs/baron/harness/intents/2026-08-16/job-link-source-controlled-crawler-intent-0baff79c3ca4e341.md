# Baron Intent Brief

- ID: `intent-0baff79c3ca4e341`
- Title: job-link-source-controlled-crawler
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-16T19:49:29+07:00

## Current Behavior

The admin Job Link screen is UI-only and the root crawler directory has no implementation. PostgreSQL already defines job_sources, source_crawl_runs, job_cache, and active_job_cache, while the Go API has no source-management endpoints or crawler runtime.

## Target Behavior

An authenticated admin can manage one URL per Job Link through a real Go API backed by job_sources, and a Rust crawler in crawler/ can load only ACTIVE sources, enforce host/path scope, produce structured job observations, record crawl health, and reconcile job cache state without raw JD persistence or mock production rows.

## Scope

Implement Job Link CRUD contract, source URL normalization and authorization boundary, crawler core interfaces, PostgreSQL-backed source/job/crawl-run integration, and deterministic reconciliation tests. Do not choose or crawl a real company source in this tranche.

## Non-Goals

- Do not add DeepSeek parsing, CV matching, client authentication, a generic open-web spider, a company-specific production adapter, or fabricated crawler rows.
- Do not expose raw JD, raw HTML, source payloads, or credentials.

## Constraints

- Crawl only ACTIVE job_sources and enforce approved hostname/path scope including redirect checks.
- Preserve existing v1 admin auth/CSRF boundary and existing job-cache response compatibility.

## Decisions

- Use the existing job_sources table rather than a duplicate job_links table; the UI exposes only URL while backend derives internal source metadata and keeps approval state server-owned.
- Use Rust + Spider for crawler runtime as specified by idea/GO_GET_A_JOB_FERRIS.md; keep Go as API and persistence owner.

## Required Proof

Go unit/handler/repository tests, Rust unit/integration tests, schema validation, gofmt/go vet/go test, cargo fmt/check/test, authenticated API smoke checks where PostgreSQL is available, security review, and Baron trace score.

## Remaining Unknowns

- The first legally approved real source and its exact feed/API/ATS shape remain unknown until external source review.
- Whether the local PostgreSQL instance has the latest migrations applied must be proven during smoke verification.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
