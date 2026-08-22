# Simple Home Section Editor Design

## Goal

Make Home section management understandable for non-technical admins while keeping the existing PostgreSQL and Cloudinary contract safe.

## Approved behavior

- Sections 1–3 expose exactly three content inputs: `Tiêu đề`, `Nội dung`, and `Ảnh`.
- The existing `Hiển thị` control remains because it controls publication state, not content authoring.
- Section 4 accepts at most 10 real uploaded images. Each item exposes upload, show/hide, and delete controls; it does not expose alt text, eyebrow, link, or technical ordering fields.
- Desktop section 4 renders one continuously moving horizontal row.
- Mobile section 4 renders two rows, with the rows moving in opposite directions so the content remains visible without horizontal page overflow.
- Pointer hover, keyboard focus, and `prefers-reduced-motion: reduce` pause or reduce movement. No essential action depends on the animation.
- The public Home renders only the title, body, image, and section 4 media. Legacy eyebrow and target URL values are no longer rendered.

## Technical behavior

- The database keeps legacy `eyebrow`, `image_alt_text`, and `target_url` columns for backward compatibility; no destructive migration is needed.
- The Go service derives image alt metadata from the section title, or from a stable generic label when no title is available. Section 4 derives a stable label from the media position.
- The admin frontend omits the technical fields from `FormData` and JSON requests.
- The backend remains compatible with older clients that send the legacy fields, but new writes do not require them and new UI never displays them.
- The maximum section 4 media count is changed from 12 to 10 at the service boundary. Existing records above the new limit are not deleted automatically; the admin list can still show them for cleanup, while new uploads are rejected once 10 active records exist.

## Design fingerprint

Mode: redesign of the section 4 presentation, refine of the admin editor.

Product job: an occasional non-technical admin should publish Home content without understanding CMS or accessibility vocabulary, while a client should see a lightweight brand/content strip.

Audience and pressure: non-technical operator entering a small amount of marketing content; client scanning a page on desktop or a narrow phone viewport.

Existing signals to preserve: Sugoi-oniichan orange/blue brand, dark Ferris visual language, red action accent, and the existing real-data-only CMS boundary.

Chosen macrostructure: simple content editor for slots 1–3 plus a dedicated image-strip manager for slot 4. The split matches the two different content shapes and avoids exposing technical metadata.

Product-specific visual signals: 16:9 content imagery, continuous horizontal brand strip, and two-row mobile strip inspired by the supplied CV Genius reference without copying its assets or proprietary content.

Generic defaults rejected: a CMS form full of SEO fields, a generic static three-card grid for the logo strip, and unbounded horizontal overflow on mobile.

## Acceptance criteria

1. The admin source contains no visible `Eyebrow`, `Alt text`, or `Link` fields in the Home editor.
2. A section save request contains only publication state, title, body, and an optional image file.
3. A media upload request contains only position/state and the image file; the server supplies alt metadata.
4. A section with a title and image but no alt input is accepted and stores derived metadata.
5. A media upload with no alt input is accepted and stores derived metadata.
6. The client does not render legacy eyebrow or target-link UI.
7. The media count contract is 10 for new uploads.
8. Frontend tests, Svelte checks, build, focused Go tests, and Go vet pass. Browser viewport proof is recorded separately; if the local browser cannot authenticate or run, that limitation remains explicit.
