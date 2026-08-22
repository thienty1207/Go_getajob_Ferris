# Baron Review Finding

- ID: `finding-f6587cb1caaadf2b`
- Status: `closed`
- Severity: `important`
- Recorded: 2026-08-20T22:18:32+07:00

## Summary

Auth me endpoint rotates the sole CSRF token and invalidates other tabs

## Evidence

- Both admin and client Me handlers call RefreshCSRF on every request while repositories store only one current digest.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-20T23:37:48+07:00
- Fix evidence: Admin and client auth now keep a stable HttpOnly role-specific CSRF cookie for the session; /auth/me reuses a valid token and performs only one stable compatibility refresh, while frontend bootstraps coalesce concurrent requests.
- Verification: Go admin/client auth service and HTTP package tests passed, including two consecutive /me calls and legacy refresh; frontend client-auth-store Vitest passed 5 tests including concurrent same-tab bootstrap; full focused Go vet passed.
