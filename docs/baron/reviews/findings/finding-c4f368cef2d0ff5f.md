# Baron Review Finding

- ID: `finding-c4f368cef2d0ff5f`
- Status: `closed`
- Severity: `important`
- Recorded: 2026-08-20T22:18:33+07:00

## Summary

Deleting the only CV on the last history page shows a false global empty state

## Evidence

- Client CV page reloads the unchanged page after deletion and hides pagination when items is empty even if total remains positive.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-20T23:10:40+07:00
- Fix evidence: Client CV history clamps the current page to the new last page after deletion and refetches before rendering, so deleting the sole row on page 2 no longer shows a false global empty state
- Verification: Vitest cv-history-pagination.spec.ts passed 3 tests and svelte-check found 0 errors and 0 warnings
