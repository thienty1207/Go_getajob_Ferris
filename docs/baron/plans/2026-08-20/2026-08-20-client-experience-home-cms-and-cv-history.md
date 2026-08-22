---
type: baron-plan
title: client-experience-home-cms-and-cv-history
status: in_progress
risk: medium
created: 2026-08-20
updated: 2026-08-20T14:00:56+07:00
verification: not_run
---

# client-experience-home-cms-and-cv-history

## Goal

Plan and, only after explicit confirmation, deliver the authenticated client redesign, location-only CV workflow, structured CV history, real admin user/CV data, and four-section Home CMS.

## Scope

- Detailed implementation plan: `docs/superpowers/plans/2026-08-20-client-experience-home-cms-cv-history.md`.
- Current phase is planning/confirmation only. No feature implementation is authorized yet.
- Preserve the existing dirty working tree and the prior incomplete crawler/matching work.
- Keep raw CV files temporary, keep admin/client sessions separate, and use no mock data.

## Checklist

- [x] Define the implementation path.
- [x] Record API, data, privacy, responsive, and recovery boundaries.
- [ ] Receive explicit user confirmation for login-before-scan Option A and the tranche order.
- [ ] Implement the requested change.
- [ ] Record risk-appropriate proof.
- [ ] Record and score the execution trace.

## Progress Log

- 2026-08-20T14:00:56+07:00 - Plan started.
- 2026-08-20 - Detailed six-tranche plan written. Implementation is paused pending user confirmation; no product code changed under this plan.
- 2026-08-20T14:04:27+07:00 - Detailed six-tranche implementation plan saved at docs/superpowers/plans/2026-08-20-client-experience-home-cms-cv-history.md. Product language, Reasonix continuity, API/data/privacy boundaries, test gates, and interruption recovery are recorded. No product implementation started; awaiting explicit confirmation of login-before-scan Option A.
- 2026-08-20T15:51:35+07:00 - Implementation tranches 0-4 are now present: client OAuth/session/profile, owner-scoped location-only CV scan/history, dedicated scan polling route, real admin Users/CV APIs, and fixed four-slot Home CMS with Cloudinary metadata. Automated Go/Bun/Rust/schema/diff gates pass. Remaining: external Google/DeepSeek/Cloudinary live proof and final review; do not claim full completion yet.
- 2026-08-20T16:19:25+07:00 - Tranches 0–4 and scan/Home hardening implemented. Automated gates are green: Go test/vet, Bun check/test/build, Rust test/check, live PostgreSQL contracts, schema validation, public HTTP smoke, and diff check. Keep plan in_progress: external Google OAuth, DeepSeek scan, Cloudinary CMS upload/update/delete, and final review receipts remain.
