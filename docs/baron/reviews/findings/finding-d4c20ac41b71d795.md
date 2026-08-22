# Baron Review Finding

- ID: `finding-d4c20ac41b71d795`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-20T22:18:28+07:00

## Summary

Raw CV orphan younger than startup threshold may remain for process lifetime

## Evidence

- NewScanService runs cleanup once; cleanup skips files younger than 15 minutes; there is no delayed or periodic sweep.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-20T22:51:28+07:00
- Fix evidence: ScanService now owns a periodic crash-recovery sweep of the private raw-CV temp directory; recent startup orphans are preserved during the active safety window and then removed without waiting for another restart
- Verification: go test ./internal/service -count=1 -timeout=90s passed including TestTemporaryCVRecoveryEventuallyRemovesRecentStartupOrphan and TestCloseStopsTemporaryCVRecoveryTask
