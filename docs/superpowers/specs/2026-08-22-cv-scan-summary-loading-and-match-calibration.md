# CV scan summary, loading, and match calibration

## Goal

Make the client scan flow trustworthy and visually clear without adding noisy UI copy:

- show a real scan phase while the backend is processing;
- return a bounded AI summary of the structured CV;
- show only eligible active jobs;
- keep the approved deterministic score weights while preventing unrelated roles from receiving inflated scores.

## Approved behavior

The result page keeps one compact loading modal during processing. It uses the persisted scan lifecycle (`PARSING`, `MATCHING`) as the source of truth for the active phase. The UI does not display technical implementation notes or instructional paragraphs.

After completion, the page renders one summary panel followed by the ranked job list. The summary is structured data, not raw CV text and not a hiring prediction.

Every job from an approved source that is still `ACTIVE` and assigned to the selected canonical Job Location remains eligible. A job with weak evidence is shown with a low deterministic CV Match %, rather than being silently removed by a newly invented threshold. `CLOSED` jobs are excluded in both candidate selection and result projection.

## Data contract

`StructuredProfile` gains a bounded `CVSummary` value:

- `headline`: one concise role direction;
- `overview`: one short paragraph;
- `target_roles`: up to five role labels;
- `strengths`: up to five concise items;
- `gaps`: up to four concise items.

DeepSeek must return exactly one JSON object containing the existing structured profile fields plus `summary`. Go validates every scalar, list count, and string length before persistence. The provider prompt explicitly forbids name, email, phone, address, photo, raw CV text, and unbounded prose. Existing rows remain compatible with a null/empty summary.

The completed scan response adds `cv_summary` beside `matches`. The client parser rejects malformed or oversized summary data. CV history remains structured-profile-only and must not expose raw uploads.

## Matching contract

The weights remain exactly:

- required skills: 35%;
- role relevance: 25%;
- experience: 15%;
- seniority: 15%;
- preferred skills/domain: 10%.

Missing job fields provide no positive evidence instead of silently receiving the full component weight. Role relevance uses deterministic normalized role-family aliases for common groups such as support/helpdesk, software/development, data, design, security, operations, and sales; explicit unrelated families receive no role points. Token overlap remains the fallback for unknown roles. No provider call is used during scoring.

## Loading contract

The scan status endpoint exposes a bounded processing phase (`parsing` or `matching`) while retaining the stable top-level `processing` status. The client displays a centered modal with an animated document/spinner, phase progress, and accessible status attributes. Progress represents lifecycle phases, not an invented percentage from the provider. Reduced-motion users receive a non-animated equivalent.

## Verification

- backend unit tests cover closed-job exclusion, missing-field score behavior, role-family separation, and support-to-support versus support-to-software scores;
- parser tests cover summary validation, unknown fields, length bounds, and PII exclusion;
- frontend API/component tests cover processing phases, summary parsing, and compact result rendering;
- schema/migration tests cover nullable summary compatibility;
- the real local API is exercised with PostgreSQL, the configured DeepSeek provider, and `HoThienTy_IT_Officer.pdf`;
- the live check verifies a completed summary, active-only jobs, materially lower unrelated-role scores, and zero retained raw CV files without logging CV contents or PII.
