# Baron Review Finding

- ID: `finding-463015fcfaaced96`
- Status: `open`
- Severity: `minor`
- Recorded: 2026-08-15T20:37:34+07:00

## Summary

scan_matches.job_id lacks a supporting leading index

## Evidence

- database/migrations/000001_initial_schema.up.sql:188-189 defines scan_matches.job_id as an ON DELETE RESTRICT FK, while :190 and :207-207 index scan_id first. PostgreSQL does not auto-index referencing FK columns, so job deletes/referential checks or job-oriented joins may scan scan_matches.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.
