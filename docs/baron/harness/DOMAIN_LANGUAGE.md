# Product Domain Language

## Rules

- Add terms only from user, repository, product, or verified runtime evidence.
- Mark disputed or unclear meanings as `ambiguous`.
- Do not promote a term to verified without an evidence path.

## Terms

| Term | Meaning | Status | Evidence |
| --- | --- | --- | --- |
| Sugoi-oniichan | Product owner/brand displayed in the client experience; `Sugoi` uses orange and `oniichan` uses blue. It is not the product name. | confirmed | User direction on 2026-08-20; implementation plan `docs/superpowers/plans/2026-08-20-client-experience-home-cms-cv-history.md`. |
| Go get a job ferris | Public product/site name for the CV-to-job matching experience. It must not replace Sugoi-oniichan as the brand lockup. | confirmed | User direction on 2026-08-20; implementation plan. |
| Structured CV history | Owner-scoped scan metadata, validated structured profile, status, timestamps, selected canonical location, and deterministic matches. It explicitly excludes the uploaded raw file and extracted raw text. | confirmed | Existing privacy rule plus user request for saved submitted CVs on 2026-08-20. |
| Home section | One of four fixed, admin-managed content slots rendered beneath the Home upload area; slots 1–3 are alternating content/image blocks and slot 4 is an ordered horizontal image strip. | confirmed | User Home expansion request on 2026-08-20. |
