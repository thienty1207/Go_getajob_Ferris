# Baron Intent Brief

- ID: `intent-ecf89e0b03c5ee75`
- Title: client-experience-home-cms-and-cv-history
- Risk: `medium`
- Confirmation: `needs_confirmation`
- Updated: 2026-08-20T14:01:10+07:00

## Current Behavior

Client OAuth exists but the localhost/127.0.0.1 callback flow is not proven end to end; Home mixes upload and result state; scans are not owned by client users; admin Users and CV screens are UI-only; Home sections have no CMS contract.

## Target Behavior

A location-only client flow with reliable Google login, Sugoi-oniichan branding, a dedicated scan/results route, authenticated structured CV history, real admin user/CV data, and four database-backed Home sections managed by admin.

## Scope

Frontend client redesign, OAuth runtime repair, scan ownership and history, deterministic location-only matching input, DeepSeek structured parsing, admin user/CV wiring, Home section CMS APIs and Cloudinary image metadata, responsive and accessibility verification.

## Non-Goals

- No raw CV retention; no radius filter; no mock data; no arbitrary web crawler expansion; no client profile editing; no copying CV Genius proprietary content or assets.

## Constraints

- Preserve current dirty working tree; use PostgreSQL and existing Go/Svelte/Rust boundaries; keep admin and client authentication separate; validate DeepSeek JSON before storage; never expose or log secrets or unnecessary PII.

## Decisions

- Brand is Sugoi-oniichan with orange Sugoi and blue oniichan; product is Go get a job ferris; Home results placeholder is removed; scan results live on a separate reload-safe route; structured profile and scan metadata persist while temporary CV files are deleted after parsing.

## Required Proof

Backend Go tests and vet; database migration validation; frontend unit tests, svelte-check, production build; auth ownership and CSRF tests; upload validation tests; desktop and mobile browser smoke tests; security review; live Google OAuth, DeepSeek, PostgreSQL, and Cloudinary smoke tests with secrets redacted.

## Remaining Unknowns

- Whether login must be mandatory before a user can submit a CV scan. Recommended: require login so every scan has an unambiguous owner and history.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
