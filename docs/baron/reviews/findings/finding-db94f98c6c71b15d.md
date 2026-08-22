# Baron Review Finding

- ID: `finding-db94f98c6c71b15d`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-20T17:27:20+07:00

## Summary

Home section repository inverts publicOnly filtering: public API can return inactive CMS data while admin can omit inactive drafts

## Evidence

- Code review traced home_section_handler.go ListPublic(true)/ListAdmin(false) into postgres_home_sections.go WHERE ::boolean OR active predicate; this reverses the intended admin-all/public-active boundary and can cause admin draft loss or public leakage.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-20T17:29:03+07:00
- Fix evidence: Changed listHomeSectionsQuery to use WHERE NOT ::boolean OR active predicate, so admin reads include inactive drafts and public reads filter inactive sections/media at PostgreSQL. Added repository regression test.
- Verification: go test ./internal/repository -run TestListHomeSectionsQueryKeepsAdminDraftsAndFiltersPublicRows -count=1 passed; go test ./internal/httpapi -run TestPublicHomeSectionsReturnsCloudinaryMetadataWithoutStorageIdentifiers -count=1 passed; go vet ./... passed; full go test ./... was blocked only by Windows Application Control policy blocking temporary cloudinary.test.exe and service.test.exe.
