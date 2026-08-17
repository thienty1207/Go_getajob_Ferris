# Baron Review Finding

- ID: `finding-10f9c5bb182487c1`
- Status: `closed`
- Severity: `medium`
- Recorded: 2026-08-16T05:05:49+07:00

## Summary

Failed-state retry was not idempotent after an ambiguous commit

## Evidence

- The guarded status SQL now treats FAILED with the same error_code as an idempotent success path; service retries once and a test simulates apply-then-error.

## Affected Files

- backend/internal/repository/postgres_scans.go
- backend/internal/service/scan_service.go

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T05:10:50+07:00
- Fix evidence: FAILED with the same error code is now idempotent in SQL, and service retries once; apply-then-error behavior has a focused test.
- Verification: Final backend Go test suite passed with -mod=readonly.
