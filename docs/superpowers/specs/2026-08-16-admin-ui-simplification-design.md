# Admin UI simplification design

## Approved scope

Mode: `refine`.

Product job: let a non-technical admin configure the smallest possible set of
operational inputs without explaining crawler internals inside the UI.

Audience and pressure: occasional non-technical operator; the primary action
must be obvious without knowing source registries, ATS terminology, or parser
fields.

Existing signals to preserve: Ferris dark tokens, red action color, compact
admin cards, and the authenticated sidebar shell.

Chosen macrostructure: focused form followed by a directly adjacent record list.
This fits the Job Link task because the operator enters one link, then edits or
deletes links already registered.

Product-specific visual signals: one URL field, a link registry with row
actions, and the sidebar edge control centered vertically with the drawer.

Generic defaults rejected: source-type wizard, explanatory rule cards, and
dashboard-style metadata/eyebrow copy above every page title.

## UI decisions

- Rename the visible navigation label and page title from Crawl Sources to Job
  Link. Keep the existing route path for compatibility.
- Job Link contains only the URL field/action and the registered-link list.
  Edit and delete controls are shown in the list but remain unavailable until
  the backend contract exists. No mock link rows are introduced.
- CV Profiles keeps only User and Role filters. Skill, seniority, status, and
  the schema summary cards are removed.
- Remove visible red eyebrow labels, page subtitles, metadata explanations,
  account-boundary panels, and generated instructional copy from the affected
  admin screens.
- Desktop uses a vertically centered arrow edge control. Mobile uses a
  hamburger while closed and a centered arrow only while the drawer is open.

## Verification

Run Svelte check, unit tests, production build, no-mock audit, and a focused
responsive/static review. Browser screenshot evidence remains unknown if no
browser runner is available.
