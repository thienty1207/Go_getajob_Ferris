# Ferris crawler

`crawler/` is the source-controlled, no-mock crawl boundary for Ferris. It is
not an open-web scraper. The executable reads only `job_sources` rows whose
`approval_status` is `ACTIVE`, applies the registered URL/path scope to every
discovered page, resolves the source host before fetching and accepts only
globally routable unicast DNS results, pins those addresses into Spider's HTTP
client, rejects ambiguous encoded traversal paths, checks the registered path
before every HTTP body request, disables redirects, and writes only structured
job fields plus crawl-run health metadata. Source location text is normalized
deterministically and resolved against admin-managed canonical locations or
aliases; an unknown location remains unresolved instead of being guessed. When
JSON-LD exposes valid coordinates, the crawler stores them for radius matching.
Robots policy is fetched separately
from the origin-root `/robots.txt` using the same pinned client and redirect
policy, then evaluated against the registered path and every page request.
The root robots request is an explicit metadata-only exception to the Job Link
page scope; it never enters extraction or persistence.

Go and Rust use the same Job Link URL rules: default ports are removed,
explicit ports must be valid, terminal DNS dots and textual IPv6 variants are
canonicalized for identity comparison, fragments are dropped, and query
strings are retained without treating query escapes as path traversal. Robots
bodies are capped at 1 MiB and page bodies at 10 MiB even when a server omits
`Content-Length` and streams the response in chunks.

The default adapter reads server-rendered `JobPosting` JSON-LD from pages in
the registered Job Link scope. It maps only structured fields and creates no
row when the page has no usable JobPosting object. A client-rendered listing
whose data is served by an endpoint outside the registered path remains a
source-contract limitation; the crawler does not silently widen the scope.

An adapter must also mark each returned observation batch as authoritative.
An empty or low-confidence extraction is recorded as `ANOMALY` and cannot
reconcile missing jobs; only an explicitly authoritative empty batch may close
or reconcile the source's previously observed jobs.

The Go API owns the authenticated Job Link/source-registry contract. The Rust
worker is the persistence adapter for its own crawl run and job-cache write
transaction over the same PostgreSQL schema; no second database or local
cache is introduced. Job-cache upserts, missing-job reconciliation, and the
final crawl-run status commit together, so a failed cycle cannot close jobs or
leave a partial observation batch behind. If the commit acknowledgement is
ambiguous, the worker reconnects and checks the run ID before deciding whether
an anomaly status needs to be written; it never overwrites an already-finished
run. The transaction also locks and rechecks the source's `ACTIVE` status and
base URL snapshot before writing, so an admin approval or URL change cannot
silently attach an old crawl to a new boundary. If recovery remains unresolved,
the process exits with an error instead of reporting success.

## Run continuously

Run from `crawler/`:

```powershell
cargo run
```

The process loads `crawler/.env` automatically, then continuously reloads the
ACTIVE Job Link registry and reconciles each source. It uses the PostgreSQL
setting `app_settings.crawler.interval_seconds` between full cycles, with a
six-hour fallback when that row does not exist. The setting is reloaded while
the process is running, so changing it in Admin Settings does not require a
crawler restart. A VPS process manager only needs to keep this one binary
alive; no manual crawl command is needed for every cycle. `CRAWL_INTERVAL_SECONDS`
is only a local bootstrap fallback and is bounded to 15 minutes through 7 days.
An exported `DATABASE_URL` wins; otherwise the existing local aliases
(`username`, `password`, `port`, and optional `host`/`database`) are used to
build a local PostgreSQL URL. No credential is printed.

For a bounded local verification run, use:

```powershell
cargo run -- --once
```

This mode drains pending manual requests first, then performs one full cycle
and exits.

If there are no `ACTIVE` Job Links, the current cycle records nothing and the
daemon waits for the next cycle. A pending Admin “Crawl ngay” request is kept
in `crawl_requests` if this process is offline and is consumed on the next
startup. Only `ACTIVE` links are crawled, and a newly added link must still
represent a reviewed source boundary before it is activated. Press Ctrl+C for
a graceful shutdown.

## Checks

```powershell
cargo fmt --all -- --check
cargo test
cargo check

# Explicit PostgreSQL contract gate; it rolls back all temporary rows.
$env:DATABASE_URL = 'postgres://user:password@127.0.0.1:5432/gogetsomefoodferris?sslmode=disable'
cargo test --test postgres_contract -- --ignored
```

The pure reconciliation rule is deterministic: a missing job reaches
`VERIFYING` after one healthy missing cycle and `CLOSED` after two. Source,
parser, and anomaly failures never count as missing. A resolved canonical
`location_id` is used for client filtering; unresolved rows remain visible to
the admin review flow. Migration `000007_location_key_normalization` keeps
legacy PostgreSQL location keys equivalent to the crawler normalizer.
