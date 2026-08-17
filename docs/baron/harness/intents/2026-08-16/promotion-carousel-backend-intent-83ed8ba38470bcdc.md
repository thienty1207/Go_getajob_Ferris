# Baron Intent Brief

- ID: `intent-83ed8ba38470bcdc`
- Title: promotion-carousel-backend
- Risk: `medium`
- Confirmation: `needs_confirmation`
- Updated: 2026-08-16T07:34:42+07:00

## Current Behavior

Client has a static left hero panel and no promotion storage or promotion API; the admin UI is not implemented.

## Target Behavior

Client renders up to three active promotion images from a backend endpoint, while a separated admin write boundary can upload or replace a slot and PostgreSQL persists the image metadata and bytes.

## Scope

Add the promotion schema, Go repository/service/HTTP contracts, client API/type boundary, and accessible max-three carousel; do not build the admin UI or integrate unrelated crawler/matcher work.

## Non-Goals

- Do not add mock or placeholder promotion images; do not expose an unauthenticated public admin upload; do not implement the full authentication system; do not alter scan/job contracts.

## Constraints

- No runtime mock data; promotion slots are 1..3; public client reads only active records; image uploads are bounded and signature-validated; public image URLs serve stored bytes without raw provider payloads.
- Admin write route is disabled unless a server-side promotion admin token is configured and the service remains loopback-only until real admin authentication exists.

## Decisions

- Store at most three promotion image binaries plus metadata in PostgreSQL so admin replacement changes client content without a separate storage service; serve image bytes through a dedicated read route with content hash/ETag.
- Use an additive /api/v1/client/promotions read contract and /api/v1/admin/promotions/{slot} write contract; keep admin/client folders and routes separate.

## Required Proof

Migration contract checks, Go unit/handler/repository tests, frontend API/carousel tests, gofmt/go test/go vet, frontend check/build, focused security review, and desktop/mobile browser screenshots with no mock runtime assets.

## Remaining Unknowns

- Final admin authentication/session contract and production object-storage decision remain unknown; the local token gate is only a transitional safety boundary.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
