# Baron Review Finding

- ID: `finding-ed3d71c5113e2f1f`
- Status: `open`
- Severity: `important`
- Recorded: 2026-08-15T20:37:32+07:00

## Summary

Structured profile JSONB is not shape- or privacy-constrained

## Evidence

- database/migrations/000001_initial_schema.up.sql:127-140 only checks that education and certifications are JSON arrays; arbitrary objects/strings, including raw CV or contact data, remain valid despite the structured-only privacy contract in database/README.md:71-78 and docs/superpowers/specs/2026-08-15-database-foundation-design.md:49-55.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.
