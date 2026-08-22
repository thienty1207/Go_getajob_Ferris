# CV scan summary, loading, and match calibration implementation plan

> Execute in small vertical slices. Keep the user-facing UI compact and avoid adding explanatory helper copy.

## 1. Establish failing contracts

- Add backend tests for `CVSummary` validation and DeepSeek JSON parsing.
- Add matcher tests proving missing fields do not earn free points and support roles score materially below unrelated software roles.
- Add repository/API tests proving a closed job cannot be selected or returned after a scan.
- Add frontend API tests for `phase` and `cv_summary` parsing.
- Run the focused tests and record the expected failures before implementation.

## 2. Implement backend contract

- Extend `model.StructuredProfile` with a bounded summary value and preserve existing structured fields.
- Extend DeepSeek payload/schema prompt and strict validation; keep primary/fallback models and PII redaction.
- Add a forward-only migration for nullable `structured_profiles.summary` JSONB with shape checks consistent with the application validator.
- Persist the summary in `CompleteScan` and return it from the scan repository/API.
- Expose the persisted scan phase while status is processing.

## 3. Implement deterministic score calibration and active-only selection

- Make candidate and completed-match SQL explicitly require approved sources and `jobs.status = 'ACTIVE'`.
- Add deterministic role-family normalization and use it in the 25% role component.
- Remove full-credit defaults for empty job dimensions; preserve the 35/25/15/15/10 weights and score bounds.
- Keep all eligible active jobs visible and rank by score, distance, and stable ID.

## 4. Implement the client result experience

- Parse processing phase and summary with strict bounded client types.
- Replace the existing inline processing card with a compact modal/progress component driven by the real phase.
- Render the CV summary above the job list after completion.
- Keep the results page free of technical instructions and redundant copy.
- Add reduced-motion and mobile layout rules.

## 5. Verify locally and with the real PDF

- Run Go tests/vet, frontend check/tests/build, migration/schema checks, and crawler contract checks if shared contracts are touched.
- Run the real authenticated local API workflow using `C:\Users\tytyb\Downloads\Documents\HoThienTy_IT_Officer.pdf` and the configured DeepSeek key.
- Assert summary presence, active-only results, role-score separation, structured-only persistence, and raw-file cleanup.
- Remove only generated temporary test artifacts and do not commit the supplied CV.

## 6. Handoff and delivery

- Update continuity and Reasonix handoff with completed evidence, remaining risks, and exact next commands.
- Run the Baron review/trace gates with real receipts.
- Review the diff, commit the implementation, and push the approved branch/remote without committing `.env`, build output, Bun modules, or Rust `target` artifacts.
