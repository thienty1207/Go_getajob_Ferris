# Baron Review Finding

- ID: `finding-a3f87d2a15b84ba8`
- Status: `closed`
- Severity: `medium`
- Recorded: 2026-08-16T05:05:49+07:00

## Summary

Limiter cap rejected all new source keys while full

## Evidence

- The bounded limiter now evicts the oldest active bucket at capacity and has a regression test admitting a new source without exceeding 10000 buckets.

## Affected Files

- backend/internal/httpapi/middleware.go

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T05:10:51+07:00
- Fix evidence: Limiter keeps a hard 10000-bucket cap and evicts the oldest bucket to admit a new source instead of globally rejecting new clients.
- Verification: Final HTTP middleware tests passed, including cap and new-source admission cases.
