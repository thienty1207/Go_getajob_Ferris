# Go get a job, Ferris!

> CV-to-job matching tool using curated/approved employer career sources.

## Product Goal

The user uploads a CV, chooses a location and radius, and receives active jobs ranked by a deterministic **CV Match %**. The app shows only the minimum public metadata and sends the user to the employer's original career page to read the full JD and apply.

## Tech Stack

- Frontend: SvelteKit + TypeScript + Bun
- Backend: Go + Gin
- Database: self-hosted PostgreSQL
- Crawler: Rust + Spider
- AI: DeepSeek V4 API

## Core User Flow

```text
Upload CV
  ↓
Extract CV text
  ↓
DeepSeek parses CV once
  ↓
Structured CV Profile
  ↓
Choose Location + Radius
  ↓
Query ACTIVE cached jobs
  ↓
Go Matching Engine
  ↓
CV Match %
  ↓
Sort highest → lowest
  ↓
Show:
- CV Match %
- Job title
- Company
- Location
- Original job URL
```

No keyword is required.

## Source Approval

The crawler does not roam the open Internet.

```text
Find official company career page
  ↓
Review externally
  ↓
Check:
- official source?
- robots.txt?
- Terms / Legal?
- API / ATS / feed?
- automated crawling restrictions?
- commercial reuse restrictions?
  ↓
Human decides
  ↓
If approved:
Add source in Admin
  ↓
Status = ACTIVE
```

Hard rule:

```text
Source not ACTIVE
→ crawler must not crawl it
```

## Legal / Data Risk

The main structural risk of this idea is data-source legality and sustainability.

A public career page is not automatically permission to systematically crawl, cache and commercially reuse its job inventory.

Prefer:

- official public APIs
- RSS / public feeds
- ATS feeds intended for external use
- explicit company permission
- company-submitted jobs
- partnership agreements

If permission is unclear, keep the source in `REVIEW` or reject it.

## Generic Crawler

Do not create one Rust file per company by default.

Avoid:

```text
nab.rs
vng.rs
axon.rs
```

Use:

```text
ACTIVE source
  ↓
Rust + Spider
  ↓
Fetch / discover pages
  ↓
Generic extraction
  ↓
Reconcile with PostgreSQL
```

If generic extraction fails:

```text
AI-assisted structure understanding
  ↓
Save extraction rule/config
  ↓
Future runs reuse rule
```

Custom adapters are last-resort exceptions.

## Shared Job Cache

```text
Crawler discovers Job A
  ↓
Job A is new
  ↓
DeepSeek parses JD once
  ↓
Structured Job saved
  ↓
Future users reuse Job A
```

Do not reparse unchanged jobs for each user.

The cache can grow:

```text
30 → 500 → 5,000 → 100,000+
```

## Job Status

```text
ACTIVE
VERIFYING
CLOSED
EXPIRED
DISABLED
```

Only `ACTIVE` jobs are shown to users.

## Known Problems and Solutions

### Job closes before deadline

Example:

```text
Official deadline: 30/09
Company removes job: 31/08
```

Solution:

```text
ACTIVE
  ↓
job disappears
  ↓
VERIFYING
  ↓
recheck confirms missing
  ↓
CLOSED
```

The real source state overrides the future deadline.

### Job has no deadline

Do not let AI invent one.

```text
present → ACTIVE
missing → VERIFYING
repeatedly missing → CLOSED
```

### Company changes career domain or URL

Never mass-close all jobs.

```text
source URL failed
  ↓
REVIEW / SOURCE ERROR
  ↓
Admin updates approved URL
  ↓
Crawler reconciles again
```

### Temporary source failure

Examples:

- HTTP 500 / 502
- timeout
- DNS issue
- network failure
- Cloudflare error

Solution:

```text
SOURCE ERROR
  ↓
retry later
```

Do not close jobs.

### Site redesign breaks extraction

Example:

```text
Yesterday: 82 jobs
Today: 0 jobs
```

Treat this as:

```text
SOURCE ANOMALY
```

Do not assume all 82 jobs closed.

### Job URL changes

Do not use URL as the only identity.

Use signals such as:

- source job ID
- company
- title
- location
- source URL

If it is the same job, update the URL instead of creating a duplicate.

### JD changes

Use a content hash.

```text
old hash != new hash
  ↓
reparse that job once
```

### 100,000 cached jobs

Use delta refresh:

```text
99,940 unchanged → reuse
35 new            → parse
15 changed        → reparse
10 ambiguous      → AI fallback if needed
```

Never call AI across the whole cache.

## DeepSeek Responsibilities

Use AI for:

1. CV → structured CV profile
2. New/changed JD → structured job
3. Ambiguous extraction fallback

Do not use AI for:

- crawling
- radius
- duplicate detection
- expiration
- DB lookup
- sorting
- match %
- scheduler
- status transitions
- cache checks

## CV Match

Do not call it:

```text
94% chance of getting hired
```

Use:

```text
94% CV Match
```

Possible factors:

- skill overlap
- role relevance
- seniority fit
- experience
- domain relevance

Same CV + same structured job should produce the same score.

## Admin

### Sources

```text
NAB       ACTIVE
VNG       ACTIVE
AXON      ACTIVE
TikTok    REVIEW
```

### Job Cache

```text
Total        30,000
Active       24,500
Verifying        80
Closed        4,100
Expired       1,320
```

## Scheduler

The crawler runs on the server independently of users.

Example:

```text
00:00
06:00
12:00
18:00
```

The developer laptop does not need to stay online.

## Suggested Structure

```text
go-get-a-job-ferris/
├── frontend/
├── backend/
│   ├── cmd/api/main.go
│   └── internal/
│       ├── handler/
│       ├── service/
│       ├── repository/
│       ├── matching/
│       ├── ai/
│       ├── location/
│       ├── model/
│       └── router/
├── crawler/
│   └── src/
│       ├── main.rs
│       ├── config.rs
│       ├── crawler/
│       │   ├── mod.rs
│       │   ├── spider_engine.rs
│       │   ├── source_crawler.rs
│       │   └── page_discovery.rs
│       ├── extractor/
│       │   ├── mod.rs
│       │   ├── job_extractor.rs
│       │   └── extraction_rule.rs
│       ├── reconcile/
│       │   ├── mod.rs
│       │   ├── job_reconciler.rs
│       │   ├── duplicate_detector.rs
│       │   └── source_health.rs
│       ├── scheduler/
│       │   ├── mod.rs
│       │   └── crawl_scheduler.rs
│       ├── storage/
│       │   ├── mod.rs
│       │   ├── job_store.rs
│       │   └── source_store.rs
│       └── model/
│           ├── mod.rs
│           ├── job.rs
│           └── source.rs
├── database/
│   ├── migrations/
│   └── seeds/
├── docs/
└── README.md
```

Rust rule:

- `mod.rs` is for module declarations/re-exports.
- Real logic belongs in clearly named files.
- Do not create a folder containing only `mod.rs` when a single file is enough.

## Coding Principles

- Keep Go and Rust straightforward.
- Avoid macro-heavy Rust, trait labyrinths and unnecessary generics.
- Avoid enterprise-style Go abstractions without a real need.
- Use descriptive names.
- Keep files focused.
- Do not create duplicate implementations like `matching_v2.go`.
- Ignore generated folders such as `crawler/target/` and `frontend/node_modules/`.

## Main Validation Question

Before scaling the product, validate the source model:

```text
Review ~20 target career sources
  ↓
How many have clearly usable APIs/feeds/permission?
```

The major unresolved risk is not implementation difficulty.

It is:

> **Can enough high-quality job data be legally and sustainably obtained to make the product useful?**
