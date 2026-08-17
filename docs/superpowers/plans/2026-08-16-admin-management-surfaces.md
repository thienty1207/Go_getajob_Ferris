# Admin management surfaces implementation plan

> **For the implementation agent:** execute this plan inline in the current
> workspace. Keep the scope frontend-only and preserve the no-mock boundary.

**Goal:** Replace the admin header/account duplication with a collapsible
sidebar shell and add UI-only management surfaces for crawler sources, CV
profiles, client users, and canonical job locations.

**Architecture:** Keep `AdminShell.svelte` as the single authenticated shell,
use shared `admin.css` classes for the new cards/forms/tables, and add one
route per management surface under `frontend/src/routes/admin`. Do not add
API calls or alter Go/PostgreSQL in this tranche. New routes use empty local
state only for form controls and explicit unavailable/empty states; they do
not display fake rows.

**Tech stack:** Svelte 5 runes, SvelteKit file routes, TypeScript, existing
Ferris admin tokens/styles, Vitest/Svelte check, Bun.

## Task 1: Refactor the authenticated shell

**Files:**

- `frontend/src/lib/admin/components/AdminShell.svelte`
- `frontend/src/lib/admin/styles/admin.css`

1. Remove the admin topbar and its duplicate desktop account controls.
2. Move account identity and logout to a bottom-anchored sidebar account block
   for all viewport sizes.
3. Add a single arrow toggle that controls the sidebar on desktop and mobile,
   with Escape and mobile backdrop behavior preserved.
4. Add navigation entries for Crawl Sources, CV Profiles, Users, and Locations.
5. Default the sidebar open on wide screens and closed on mobile after mount;
   keep the content full-width whenever it is closed.
6. Update CSS breakpoints, focus states, and reduced-motion behavior without
   changing the existing page-card visual language.

## Task 2: Add the crawl-source allowlist UI

**Files:**

- `frontend/src/routes/admin/sources/+page.svelte`
- `frontend/src/lib/admin/styles/admin.css`

1. Add fields for source name, explicitly allowed URL/path, source type, and
   active state.
2. Explain that a later crawler is restricted to approved source scope and that
   the backend must enforce it.
3. Render an empty approved-source state and a disabled/pending save action
   labelled as UI-only until the API exists.

## Task 3: Add CV profile management UI

**Files:**

- `frontend/src/routes/admin/cvs/+page.svelte`
- `frontend/src/lib/admin/styles/admin.css`

1. Show the structured profile fields that the existing product contract
   retains: roles, skills, years of experience, seniority, domains, education,
   and certifications.
2. Provide view/delete intent only; omit edit controls and raw CV links.
3. Use a real-empty state and a disabled delete action until the later API
   contract is available.

## Task 4: Add user-management UI

**Files:**

- `frontend/src/routes/admin/users/+page.svelte`
- `frontend/src/lib/admin/styles/admin.css`

1. Add search, account status, and role filter controls as future API inputs.
2. Explain that client login is not implemented in this tranche.
3. Render no user rows and no fake counts; keep future actions disabled.

## Task 5: Add canonical location-management UI

**Files:**

- `frontend/src/routes/admin/locations/+page.svelte`
- `frontend/src/lib/admin/styles/admin.css`

1. Add canonical location fields for display name, province/city, and country.
2. Explain the future `location_id` relationship and that corrected canonical
   locations become the authority for public filters and matching.
3. Render an empty registry and a disabled/pending create/reassignment action.

## Task 6: Record product memory and verify

**Files:**

- `C:/Users/tytyb/.codex/memories/extensions/ad_hoc/notes/20260816-admin-management-logic.md`

1. Record the approved source allowlist, structured-CV retention, future client
   user management, canonical location authority, and no-mock UI rule.
2. Run frontend typecheck, tests, build, route/source audits, and the responsive
   state review.
3. Record Baron proof/gate evidence and leave browser screenshot state as
   unverified if the environment has no browser runner.
