---
type: baron-plan
title: admin-job-link-status-button-fix
status: interrupted
risk: medium
created: 2026-08-17
updated: 2026-08-17T12:28:40+07:00
verification: not_run
---

# admin-job-link-status-button-fix

## Goal

admin-job-link-status-button-fix

## Scope

- Work tied to this task only.

## Checklist

- [ ] Define the implementation path.
- [ ] Implement the requested change.
- [ ] Record risk-appropriate proof.
- [ ] Record and score the execution trace.

## Progress Log

- 2026-08-17T12:18:51+07:00 - Plan started.
- 2026-08-17T12:28:28+07:00 - Root cause confirmed: CORS preflight omitted PATCH. Added PATCH to backend/internal/httpapi/middleware.go and regression coverage in handler_test.go. Direct Go/frontend tests, fresh 18080 preflight, check, and build passed. Trusted backend proof runner blocked with no receipt; plan will remain interrupted until runner lock clears.
- 2026-08-17T12:28:40+07:00 - Interrupted: interrupted
