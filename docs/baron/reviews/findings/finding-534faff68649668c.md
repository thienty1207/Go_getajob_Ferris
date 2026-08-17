# Baron Review Finding

- ID: `finding-534faff68649668c`
- Status: `open`
- Severity: `important`
- Recorded: 2026-08-15T20:37:33+07:00

## Summary

PostgreSQL 16 apply/rollback compatibility is unverified

## Evidence

- database/tests/validate-schema.ps1:50-119 performs regex/source checks only. The environment has pwsh but no psql, pg_isready, Docker, or Podman, so no live PostgreSQL 16 up/down execution was observed; the static pass cannot prove SQL parsing, extension availability, FK/index creation, or rollback execution.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.
