# Admin management surfaces design

> The UI composition in this document was superseded by
> `2026-08-16-admin-ui-simplification-design.md`. The product rules remain
> valid; the current UI uses the simpler Job Link, CV, Users, and Locations
> surfaces described in that addendum.

## Context

Ferris is a source-controlled job matching product. The current admin UI has a
working authenticated shell, promotion management, and a real job-cache list,
but the shell still reserves a header for account controls and the next core
operations are not represented in the UI.

This tranche is frontend-only. It must not create pretend records, call
non-existent endpoints, or change the Go service/database. Empty states and
explicitly unavailable controls are intentional contract markers until the
backend tranche defines and implements the persistence APIs.

## Design direction

- Mode: `redesign` for the authenticated shell and new route composition;
  `refine` for the existing dark Ferris visual language, tokens, card geometry,
  and typography.
- The sidebar is the persistent navigation surface. It has one accessible
  arrow toggle on desktop and mobile, a mobile backdrop, Escape handling, and
  account actions anchored at the bottom.
- The content area has no duplicate header. Each route owns one page heading and
  its operational explanation.
- New screens use real-empty states rather than sample rows:
  - Crawl sources: an allowlist form and an empty approved-source list.
  - CV profiles: a structured-profile-only read/delete surface and an empty
    state. Editing and raw-file access are absent.
  - Users: a future client-user directory shell with search/filter controls and
    an empty state. It does not imply that client authentication exists.
  - Locations: a canonical location registry shell and a future job-reassignment
    relationship. It does not locally mutate job-cache rows.

## Product rules represented by the UI

### Source-controlled crawler

An admin source record is an explicit permission boundary. The later crawler
may request only the registered URL/path and the allowed source scope. The UI
labels the form as an allowlist, captures source type and active state, and
does not offer a generic “crawl web” action. The backend must later enforce
the same boundary server-side; this UI is not a security boundary by itself.

### CV management

The client upload is parsed into a structured profile. Long-lived admin views
must expose only fields such as roles, skills, experience, seniority, domains,
education, and certifications. The raw file is temporary and is not rendered
or downloaded from this surface. The only planned destructive action is delete;
there is no edit affordance.

### User management

The screen is a future client-account workspace. It intentionally shows no
invented users and no fake status counts. Later API contracts can add search,
role/status, and account actions without changing the navigation model.

### Canonical locations

Crawler-provided location text is not the permanent authority when an admin
corrects it. The later data model should associate a job cache row with a
canonical `location_id`; public location filters and distance/matching logic
must use that canonical location after reassignment. The UI presents this as a
registry and a future reassignment workflow, not as a local-only edit.

## Route and component structure

```text
AdminShell
├── Overview
├── Promotions
├── Job Cache
├── Crawl Sources
├── CV Profiles
├── Users
└── Locations
```

Each new route is self-contained and uses shared admin classes. No route owns a
second shell, account header, or client-side fake store.

## Accessibility and responsive behavior

- The toggle is a real button with `aria-controls`, `aria-expanded`, and a
  visually hidden action label. The arrow direction communicates the current
  state.
- The sidebar is keyboard-dismissible with Escape. On mobile, its backdrop is
  a button with an accessible label.
- Account controls remain reachable at the bottom of the open sidebar on both
  viewport classes.
- Forms use labels, fieldsets where useful, visible focus states, disabled
  pending controls, and readable empty/error states.
- Reduced-motion users get no required transition or animation.

## Verification intent

Run Svelte check, frontend tests, and production build. Audit that new routes
contain no mock rows and that no topbar/account duplicate remains. Review wide,
narrow, empty, unavailable, keyboard, and reduced-motion states. A real browser
screenshot is marked unverified if no browser runner is available in the local
environment.
