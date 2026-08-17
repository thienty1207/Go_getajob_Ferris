---
type: baron-plan
title: location-aware-crawler-and-matching
status: in_progress
risk: medium
created: 2026-08-17
updated: 2026-08-17T14:03:17+07:00
verification: not_run
---

# location-aware-crawler-and-matching

## Goal

location-aware-crawler-and-matching

## Scope

- Work tied to this task only.

## Checklist

- [ ] Define the implementation path.
- [ ] Implement the requested change.
- [ ] Record risk-appropriate proof.
- [ ] Record and score the execution trace.

## Progress Log

- 2026-08-17T14:03:17+07:00 - Plan started.
- 2026-08-17T14:06:35+07:00 - Approved implementation plan saved to docs/superpowers/plans/2026-08-17-location-aware-crawler-matching.md. Next: run schema contract RED tests, then implement migration and location normalization.
- 2026-08-17T15:15:44+07:00 - Implementation and direct verification completed: DB migrations 000006/000007, Go canonical location APIs and matching, Rust generic daemon crawler with --once, frontend location contracts/UI. Fresh direct gates passed; Baron proof runner remains blocked by worker/state lock, recovery recorded. Live --once source result was ANOMALY extraction_not_authoritative with no mock job rows; trace score standard/standard passed.
- 2026-08-17T16:32:47+07:00 - Scope expanded with the approved Admin Settings and Crawl Now control: database-backed hours/minutes crawler interval, runtime reload, queue-backed manual crawl requests, status visibility, and a future-extensible settings surface. Proceed with design/spec and TDD implementation; preserve no-mock and source-controlled crawler constraints.
