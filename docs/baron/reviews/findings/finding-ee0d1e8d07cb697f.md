# Baron Review Finding

- ID: `finding-ee0d1e8d07cb697f`
- Status: `closed`
- Severity: `medium`
- Recorded: 2026-08-16T04:57:11+07:00

## Summary

Scan retrieval lacked a dedicated GET rate limit

## Evidence

- Security review found GET status route had no limiter; a dedicated read limiter is now being added and must be verified.

## Affected Files

- backend/internal/httpapi/router.go
- backend/internal/httpapi/middleware.go

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T05:00:10+07:00
- Fix evidence: GET scan status now has a dedicated read limiter configured separately from POST intake.
- Verification: Dedicated GET rate-limit contract test passed.
