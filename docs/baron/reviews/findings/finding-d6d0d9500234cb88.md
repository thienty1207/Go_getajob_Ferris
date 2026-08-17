# Baron Review Finding

- ID: `finding-d6d0d9500234cb88`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-16T05:05:48+07:00

## Summary

Initial scan lifecycle used an unsafe data-modifying CTE pattern

## Evidence

- The earlier CTE approach was replaced with a pgx transaction that INSERTs RECEIVED, UPDATEs the same row to PARSING with a guarded current state, and commits both statements.

## Affected Files

- backend/internal/repository/postgres_scans.go

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-16T05:10:50+07:00
- Fix evidence: Replaced the data-modifying CTE with pgx transaction INSERT RECEIVED, guarded UPDATE PARSING, rows-affected check, commit, and bounded commit recovery query.
- Verification: Final repository tests and go vet passed with -mod=readonly; schema contract passed.
