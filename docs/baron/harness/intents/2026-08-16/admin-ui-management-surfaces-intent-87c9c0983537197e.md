# Baron Intent Brief

- ID: `intent-87c9c0983537197e`
- Title: admin-ui-management-surfaces
- Risk: `medium`
- Confirmation: `confirmed`
- Updated: 2026-08-16T14:44:17+07:00

## Current Behavior

Admin authentication, promotions, and the existing structured job-cache screen are implemented. The admin shell still has a header-level account area and only the overview/promotions/job-cache navigation; source allowlisting, CV management, user management, and canonical location management have no UI surfaces yet.

## Target Behavior

The authenticated admin workspace has a collapsible sidebar on desktop and mobile with account actions at its bottom, plus UI-only screens for source allowlisting, read/delete CV profile management, future client-user management, and canonical job locations. The crawler contract is source-controlled: it may crawl only an explicitly approved source URL/path, while corrected canonical locations become the source of truth for job listing and matching.

## Scope

Frontend admin shell, navigation, CSS, and UI-only routes/components for crawl sources, CV profiles, users, and locations. Do not implement backend endpoints, database migrations, Cloudinary changes, or fake rows.

## Non-Goals

- Backend/API/database implementation for the new screens; client authentication; editing CV profiles; arbitrary web crawling; inserting mock records into the UI.

## Constraints

- Preserve existing admin visual language and real API boundaries. Do not invent data when APIs are absent. CV UI must represent structured profile metadata only and must never expose raw CV files. Location correction must be modeled as a future canonical location_id relationship.

## Decisions

- Use a fixed sidebar shell with an accessible arrow toggle and mobile backdrop; keep page-specific content in separate admin routes. Use empty states and disabled/pending controls for unimplemented API operations.

## Required Proof

Frontend Svelte typecheck, unit tests, production build, route/source audit, responsive-state review, and code review evidence. Browser screenshots remain unverified unless a browser runner is available.

## Remaining Unknowns

- Exact backend contracts and persistence workflows for source allowlists, CV deletion, client users, and canonical location reassignment will be defined in the later backend tranche.

## Agent Rules

- Read project, Vault, plan, Harness, and prior decisions before asking the user.
- Ask one missing high-value question at a time.
- Do not treat unknowns as facts.
- Medium/high-risk implementation requires this intent to be confirmed.
