# Baron Review Finding

- ID: `finding-d570fa330bfc777f`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-20T22:18:29+07:00

## Summary

Client CV deletion can leave an orphaned structured profile

## Evidence

- deleteClientCVQuery uses sibling data-modifying CTEs under one snapshot; profile NOT EXISTS can still observe the scan deleted by the sibling CTE.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-20T23:37:48+07:00
- Fix evidence: Client and admin CV deletion now use an explicit Read Committed transaction: owner-scoped scan DELETE RETURNING profile_id, profile FOR UPDATE lock, and a subsequent NOT EXISTS cleanup statement, preventing snapshot and concurrent-final-reference orphans.
- Verification: go test -tags=integration ./internal/repository -run TestPostgres(Client|Admin)CVDeleteTransaction -count=1 passed all 7 real PostgreSQL cases; package tests and go vet passed; live authenticated CV upload/delete left 0 scan rows and 0 profile rows.
