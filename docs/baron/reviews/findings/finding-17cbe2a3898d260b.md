# Baron Review Finding

- ID: `finding-17cbe2a3898d260b`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-16T04:48:54+07:00

## Summary

Non-finite MAX_RADIUS_KM or direct radius input can bypass radius validation

## Evidence

- config.go parsePositiveFloat and service validateScanInput accept NaN because comparisons with NaN are false; add finite checks and regression tests.

## Affected Files

- backend/internal/config/config.go
- backend/internal/service/scan_service.go

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T05:00:08+07:00
- Fix evidence: parsePositiveFloat now rejects NaN and Inf; service validation rejects non-finite configured and request radii; regression tests cover NaN and both infinities.
- Verification: Focused config and service non-finite radius tests passed.
