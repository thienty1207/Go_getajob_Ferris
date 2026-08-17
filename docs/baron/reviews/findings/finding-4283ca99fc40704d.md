# Baron Review Finding

- ID: `finding-4283ca99fc40704d`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-16T04:48:55+07:00

## Summary

Lifecycle persistence failures can leave scans stuck in RECEIVED or PARSING

## Evidence

- scan_service.go creates before PARSING and updates FAILED after processor errors; failed transitions return without a recoverable state or operational signal.

## Affected Files

- backend/internal/service/scan_service.go
- backend/internal/repository/postgres_scans.go

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T05:10:49+07:00
- Fix evidence: Initial lifecycle is now guarded by a real transaction, commit-outcome verification by scan ID, bounded failed-state retry, and idempotent FAILED transition; temporary cleanup errors are logged.
- Verification: Final readonly Go test suite passed, including repository query contracts and service ambiguous-failure tests.
