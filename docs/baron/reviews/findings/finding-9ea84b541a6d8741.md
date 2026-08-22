# Baron Review Finding

- ID: `finding-9ea84b541a6d8741`
- Status: `closed`
- Severity: `high`
- Recorded: 2026-08-20T17:31:42+07:00

## Summary

Home section media loader passes publicOnly directly as includeInactive, reversing active media visibility

## Evidence

- After the section predicate fix, ListHomeSections still called loadMedia(..., publicOnly); listHomeSectionMediaQuery interprets true as include inactive, so public reads exposed inactive media and admin reads omitted inactive media.

## Affected Files

- none recorded

## Closure Rule

A finding closes only after both fix evidence and verification are recorded.

## Closure Evidence

- Closed: 2026-08-20T17:32:33+07:00
- Fix evidence: ListHomeSections now passes includeInactiveForHomeRead(publicOnly), which maps publicOnly=true to active media only and admin publicOnly=false to all media drafts. Added a truth-table regression test.
- Verification: go test ./internal/repository -run 'TestListHomeSectionsQueryKeepsAdminDraftsAndFiltersPublicRows|TestHomeSectionMediaReadScopeKeepsDraftMediaAdminOnly' -count=1 passed; go vet ./... passed; public HTTP handler contract passed.
