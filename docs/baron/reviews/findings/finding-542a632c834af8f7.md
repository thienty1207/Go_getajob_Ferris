# Baron Review Finding

- ID: `finding-542a632c834af8f7`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-20T22:47:06+07:00

## Summary

Cloudinary configuration and provider errors can disclose credentials through startup logs

## Evidence

- backend/internal/cloudinary/uploader.go wraps parser/provider errors and backend/cmd/api/main.go logs the returned error; malformed URLs may include CLOUDINARY_URL credentials

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-20T23:39:35+07:00
- Fix evidence: Cloudinary URL parsing, SDK upload/destroy errors, embedded provider errors, and invalid destroy results now return stable credential-free error categories before main logs them.
- Verification: Cloudinary package tests passed all configuration/provider redaction cases; live Cloudinary upload, delivery, and destroy completed without exposing configuration values.
