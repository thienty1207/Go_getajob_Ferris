# Baron Intent Brief

- ID: `intent-1dadc16d46eed211`
- Title: backend-api-foundation
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-16T04:24:45+07:00

## Current Behavior

The repository has a completed client frontend and PostgreSQL schema, but no Go backend runtime or client scan API.

## Target Behavior

A Go Gin API connects to self-hosted PostgreSQL, accepts validated CV uploads only through temporary files, exposes health and client scan status endpoints, persists scan lifecycle state, and returns the existing frontend contract without mock job data.

## Scope

Implement backend/ Go module with config, database connection, scan repository, intake service, upload validation, HTTP handlers/router, rate limit and CORS middleware, tests, and local run documentation; keep DeepSeek, crawler, authentication, admin UI, and match computation behind explicit next-step interfaces.

## Non-Goals

- Do not add mock or seed jobs/CVs/matches.
- Do not implement DeepSeek parsing, Rust crawler, deterministic matcher, authentication, admin API, or production deployment.

## Constraints

- Raw CV files are temporary and deleted after processing; only structured profiles may be persisted by a future processor.
- Use PostgreSQL parameterized queries; distances are kilometers; source currency is not converted.
- Public responses must match frontend snake_case contract and errors must not expose internal details.

## Decisions

- Use Gin handlers plus service/repository interfaces; the initial runtime injects an explicit unavailable processor so the API fails closed instead of fabricating completed matches.
- Bind locally by default and allow CORS/rate limits through environment configuration.

## Required Proof

Run gofmt, go test ./..., go vet ./..., static no-mock/raw-CV audit, API contract tests, and security review; report live PostgreSQL connectivity separately if unavailable.

## Remaining Unknowns

- Scan ownership/authentication contract remains unresolved and must be added before non-local public deployment.
- DeepSeek processor and source/crawler adapters are not yet selected.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
