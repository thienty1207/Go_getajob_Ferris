# Baron Review Finding

- ID: `finding-f649548dadbde266`
- Status: `closed`
- Severity: `medium`
- Recorded: 2026-08-15T20:49:59+07:00

## Summary

Structured profile JSONB initially accepted arbitrary nested records outside the approved schema

## Evidence

- Initial review identified education/certifications JSONB arrays as unrestricted beyond array type in database/migrations/000001_initial_schema.up.sql; fixed by immutable public.is_structured_record_array with allowed keys, scalar-only values, 20-record cap, and 512-character string cap.

## Affected Files

- database/migrations/000001_initial_schema.up.sql

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-15T20:50:06+07:00
- Fix evidence: Added public.is_structured_record_array and education/certification shape constraints with approved keys, scalar-only values, record cap, and string cap.
- Verification: pwsh -NoProfile -File database/tests/validate-schema.ps1 passed
