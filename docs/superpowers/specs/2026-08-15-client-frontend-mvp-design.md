# Client Frontend MVP Design

**Status:** Approved by user on 2026-08-15

## Product job

Help a job seeker in Vietnam upload a CV, choose a location and radius in kilometres, and reach relevant active jobs through a clear, trustworthy client flow. The frontend must explain that AI parses the CV while the deterministic matching engine calculates `CV Match %`.

## Scope

This tranche implements only the public client surface in `frontend/`. It does not implement admin pages, backend services, database, crawler, or DeepSeek integration.

The client is API-first. It must never use mock data, fake API responses, seeded job rows, fallback sample results, or hard-coded admin/client records. If the API is unavailable, the UI shows a real error or empty state.

## Information architecture

Client routes are physically separated from the future admin surface:

```text
src/routes/
├── client/
│   └── +page.svelte
└── admin/
    └── (reserved for a later tranche)
```

Client-specific components and API code live under `src/lib/client/`; shared HTTP/types/styles live under `src/lib/shared/`. Client code must not import future admin internals.

The first client route is `/client`. The root route redirects to `/client` so the public entrypoint stays discoverable without mixing route ownership.

## Client interaction states

The upload flow has these observable states:

```text
idle → selected → submitting → polling → success
                                  ├── empty
                                  └── error
```

Validation happens before the network request:

- Accepted extensions: PDF, DOCX, TXT.
- Maximum file size: 10 MB.
- Location is required.
- Radius is required and expressed in kilometres.

The submit button is disabled unless the form is valid. During submission/polling, the file input and action controls are disabled and the UI exposes a live status message. Retry preserves the selected location/radius and lets the user choose the file again if the upload failed.

## API boundary

The frontend owns no matching logic. It consumes a real API contract:

```text
POST /api/v1/client/scans
Content-Type: multipart/form-data
Fields: cv, location, radius_km
Response: 202 { scan_id, status: "processing" }
```

The client then polls:

```text
GET /api/v1/client/scans/:scan_id

Processing:
{ scan_id, status: "processing" }

Completed:
{
  scan_id,
  status: "completed",
  matches: [
    {
      id,
      match_percent,
      title,
      company,
      location,
      distance_km,
      employment_type,
      work_mode,
      salary,
      skill_tags,
      original_url
    }
  ]
}

Failed:
{ scan_id, status: "failed", message }
```

The API base URL comes from `PUBLIC_API_BASE_URL`. No endpoint response is synthesized in the browser. The polling interval and timeout are explicit client constants and show a recoverable error when exceeded.

## Public result contract

Each result renders only:

- `CV Match %`.
- Job title.
- Company.
- Location.
- Distance in km, when available.
- Employment type.
- Remote, Hybrid, or Onsite.
- Salary exactly as supplied by the source, when available.
- At most three skill tags.
- Original job URL.

Remote jobs do not receive an invented distance. Missing salary is rendered as an honest undisclosed state.

## Visual brief fingerprint

```text
Mode: redesign of an unimplemented surface, grounded in the approved UI Demo
Product job: upload CV and reach trustworthy matched-job links quickly
Audience and pressure: Vietnam job seekers making a high-intent, low-patience decision
Existing signals to preserve: Ferris identity, black/red visual language, CV Match score, direct-to-source links
Chosen macrostructure: focused form first, results below; it keeps the primary action and its outcome in one client flow
Product-specific visual signals: deterministic score ring, km/location context, source-first job cards
Generic defaults rejected: fake SaaS dashboard metrics, purple AI gradients, sample job cards that disguise a missing API
Unknowns: exact backend auth contract and final API deployment URL
```

## Accessibility and responsive requirements

- Use native labels, buttons, file input, and links.
- Provide a visible focus ring that matches the red accent.
- Announce upload/processing/error status through an `aria-live` region.
- Keep drag-and-drop as an enhancement; keyboard file selection remains available.
- Preserve usable touch targets and readable text at narrow widths.
- Respect `prefers-reduced-motion` for score/status transitions.
- Test narrow and wide layouts with long job titles, long company names, missing salary, and remote jobs.

## Non-goals

- Admin pages or admin navigation.
- Backend, database, crawler, DeepSeek calls, or real authentication implementation.
- Full JD storage or display.
- CV parsing in the browser.
- Mock data of any kind in runtime UI.
