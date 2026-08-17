# Job Link and source-controlled crawler design

## Goal

Make the Job Link screen a real admin contract and create the first runnable
crawler boundary without allowing open-web crawling, fabricated jobs, or raw
job-document retention.

## User-facing behavior

The admin enters one URL. The API derives the source identity and display name;
the UI does not ask a non-technical operator to choose a source type, adapter,
or crawl strategy.

- `POST /api/v1/admin/job-links` creates an approved source from the URL.
- `GET /api/v1/admin/job-links` returns bounded source metadata and latest run
  status, never credentials or fetched page content.
- `PATCH /api/v1/admin/job-links/:id` replaces the crawl boundary while keeping
  the source identity stable and refreshing the approval actor/time.
- `DELETE /api/v1/admin/job-links/:id` disables the source instead of deleting
  its row, preserving job-cache and crawl-run history.

Adding or editing a link is an explicit admin approval action. The API stores
the existing database source type `EXPLICIT_PERMISSION` because this simple
surface has no source-classification field; a future review workflow can
reclassify it without changing the Job Link UI. The crawler still reads only
`approval_status = ACTIVE`.

## URL boundary

The Go service validates and normalizes URL input before persistence:

- only `http` and `https` are accepted;
- username/password components are rejected;
- the host is required and the fragment is removed;
- default ports are removed, explicit ports must be in `1..=65535`, and
  terminal DNS dots and textual IPv6 variants are canonicalized for identity;
- the path is normalized to a stable scope with a single trailing slash;
- query strings are retained, but path-only encoded dot/slash/backslash escapes
  are rejected before URL decoding;
- the URL remains the configured source boundary; it is not a generic search
  seed.

The Rust crawler repeats the boundary check for every discovered URL. A
candidate must use the same scheme, equivalent canonical host/IP, and port and
must be equal to or under the registered path. Ambiguous percent-encoded dot,
slash, or backslash path escapes are rejected before comparison. A redirect or
discovered link outside that scope is not fetched. Fragments are ignored for
the boundary and query strings remain allowed metadata. Robots policy is
enforced by the crawler's path-aware fetch engine.

## API contract

State-changing requests use the existing admin session cookie plus the existing
CSRF header. List requests remain session-protected. The API uses the existing
`{code,message}` error shape and returns no raw source payload.

```text
JobLinkCreateRequest { url: string }
JobLinkUpdateRequest { url: string }

JobLink {
  id: string
  url: string
  source_key: string
  display_name: string
  approval_status: REVIEW | ACTIVE | DISABLED
  approved_at: string | null
  approved_by: string | null
  created_at: string
  updated_at: string
  last_crawl_status?: HEALTHY | SOURCE_ERROR | PARSER_ERROR | ANOMALY
  last_crawl_at?: string
}

JobLinkPage { items: JobLink[], page: number, page_size: number, total: number }
```

`REVIEW` is retained for source rows created by an earlier workflow or by a
direct database import. The current one-field admin flow creates and updates
rows as `ACTIVE`; the API still exposes the database state faithfully, so
approval evidence is nullable until a row is actively approved.

The list is capped at 50 rows per request and sorted by `updated_at DESC,
id DESC`. The source key is generated from a UUID and therefore does not
change when an operator corrects a URL.

## Crawler boundary

`crawler/` is a Rust executable/library boundary with these responsibilities:

1. load only active source rows from PostgreSQL;
2. create a crawl-run health record for each attempted source;
3. configure Spider with bounded depth/page count and a server-side URL scope
   callback; resolve only globally routable unicast DNS destinations and pin
   those addresses into a pre-request HTTP fetch engine;
4. pass in-memory pages to a source adapter/extractor boundary;
5. discard page HTML after processing and persist only structured observations;
6. expose deterministic reconciliation rules for content-hash delta updates
   and healthy missing cycles.

Spider's built-in robots support is disabled because it fetches the origin-root
`/robots.txt` without exposing the parsed policy at the Job Link boundary. The
crawler instead fetches that root policy once with the same pinned client and
redirect restrictions, evaluates the registered path before starting Spider,
and checks every page URL in the pre-request fetch engine. The root robots
request is an explicit metadata-only exception to the registered page path;
it never enters extraction or persistence. The post-fetch callback remains as
a second scope check. The policy body is capped at 1 MiB and each page body at
10 MiB, including chunked responses without `Content-Length`.

An adapter returns an observation batch with an explicit `authoritative` flag.
An empty non-authoritative batch is an extraction anomaly and cannot trigger
missing-job reconciliation; an explicitly authoritative empty batch is the
only empty result that may reconcile the source.

No production adapter is selected in this tranche. A source adapter will be
added only after the first source's legal crawl/feed permission and shape have
been reviewed. Until then, the crawler exits cleanly with zero active sources
and reports parser/source state rather than manufacturing `job_cache` rows.

## Reconciliation rules

- a healthy observation with the same `(source_id, source_job_key)` resets
  `missing_healthy_cycles` to zero and makes the row `ACTIVE`;
- a missing item in the first healthy cycle becomes `VERIFYING` with cycle 1;
- a missing item in the second consecutive healthy cycle becomes `CLOSED` with
  cycle 2;
- `SOURCE_ERROR`, `PARSER_ERROR`, and `ANOMALY` never increment missing cycles;
- a non-authoritative observation batch becomes `ANOMALY` and never increments
  missing cycles;
- a source-declared closed job becomes `CLOSED` immediately;
- `content_hash` determines whether an observed structured row is unchanged or
  needs an update.

Healthy observations, missing-job reconciliation, and the final crawl-run
status are committed in one PostgreSQL transaction. That transaction locks and
rechecks the source's `ACTIVE` approval plus the expected base URL before any
job-cache write. If the commit response is ambiguous, the worker reconnects
and inspects the run ID before compensating; an already-finished run is never
overwritten. If recovery cannot resolve the outcome, the worker returns a
non-zero error rather than silently succeeding.

## Non-goals

- no DeepSeek call or CV matching implementation;
- no company-specific adapter before source review;
- no open-web discovery, proxy behavior, redirect outside the allowlist, or
  user-controlled fetch endpoint;
- no raw JD, raw HTML, credentials, or PII in PostgreSQL;
- no mock data or development fixture generated by the crawler.

## Verification

Go tests cover URL normalization, source CRUD validation, admin session/CSRF
protection, nullable approval evidence, soft disable, and parameterized
repository queries. Rust tests cover same-host/path scope,
redirect/discovered-link rejection, fetch-failure classification, and the
exact healthy missing-cycle state machine. The migrated PostgreSQL contract
test is an explicit smoke gate for the atomic run/persist/rollback path. Then
run Go format/test/vet and Cargo format/test/check plus the Baron proof/trace
gates.
