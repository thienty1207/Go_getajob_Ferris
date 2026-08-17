# Baron Review Finding

- ID: `finding-f6ed6ccebf663b5b`
- Status: `closed`
- Severity: `medium`
- Recorded: 2026-08-16T04:48:54+07:00

## Summary

Per-IP rate limiter has no hard cap for many active source keys

## Evidence

- middleware.go only deletes expired buckets after the map exceeds 10000; active distinct keys remain and can grow without a fixed capacity policy.

## Affected Files

- backend/internal/httpapi/middleware.go

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T05:00:09+07:00
- Fix evidence: Rate limiter now has a 10,000 active-bucket hard cap, periodic expired-bucket cleanup, and a cardinality regression test.
- Verification: Focused rate limiter tests passed.
