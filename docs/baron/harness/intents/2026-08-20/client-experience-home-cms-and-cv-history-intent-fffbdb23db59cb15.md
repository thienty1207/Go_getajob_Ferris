# Baron Intent Brief

- ID: `intent-fffbdb23db59cb15`
- Title: client-experience-home-cms-and-cv-history
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-20T14:10:19+07:00

## Current Behavior

Client OAuth exists but local host/state cookie flow needs repair and live proof; Home mixes upload and result state; scans are not owned by client users; admin Users and CV screens are UI-only; Home sections have no CMS contract.

## Target Behavior

A location-only client flow with reliable Google login, Sugoi-oniichan branding, a dedicated scan/results route, authenticated structured CV history, real admin user/CV data, and four database-backed Home sections managed by admin.

## Scope

Frontend client redesign, OAuth runtime repair, scan ownership and history, deterministic location-only matching input, DeepSeek structured parsing, admin user/CV wiring, Home section CMS APIs and Cloudinary image metadata, responsive and accessibility verification.

## Non-Goals

- No raw CV retention; no radius filter; no mock data; no arbitrary web crawler expansion; no client profile editing; no copying CV Genius proprietary content or assets.

## Constraints

- Preserve current dirty working tree; use PostgreSQL and existing Go/Svelte/Rust boundaries; keep admin/client authentication separate; validate DeepSeek JSON before storage; never expose or log secrets or unnecessary PII.

## Decisions

- Confirmed Option A: require Google login before CV submission. Brand is Sugoi-oniichan with orange Sugoi and blue oniichan; product is Go get a job ferris; Home results placeholder is removed; scan results live on a separate reload-safe route; structured profile and scan metadata persist while temporary CV files are deleted after parsing.

## Required Proof

Backend Go tests and vet; database migration validation; frontend unit tests, svelte-check, production build; auth ownership and CSRF tests; upload validation tests; desktop and mobile browser smoke tests; security review; live Google OAuth, DeepSeek, PostgreSQL, and Cloudinary smoke tests with secrets redacted.

## Remaining Unknowns

- Live Google consent/browser availability, production domain and HTTPS redirect values, and whether legacy anonymous scans should be retained indefinitely are not yet confirmed; do not guess.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
