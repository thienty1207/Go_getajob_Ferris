# Go Backend API Foundation Design

## Goal

Create the first real backend boundary for the client scan flow. The API accepts a CV upload and a Vietnam location/radius, stores only scan lifecycle metadata in PostgreSQL, exposes scan status using the existing frontend contract, and never fabricates a job match.

## Scope

The first backend slice owns:

- `GET /healthz` for database readiness.
- `POST /api/v1/client/scans` for multipart CV intake.
- `GET /api/v1/client/scans/:scan_id` for scan lifecycle and persisted matches.
- PostgreSQL connection and parameterized scan/match reads and writes.
- Upload size/type/signature validation and temporary-file cleanup.
- CORS configuration for the local SvelteKit client and separate per-IP intake/read rate limits.
- Public response/error mapping, without returning internal database/provider errors.

The first slice does not own DeepSeek, text extraction, geocoding, deterministic matching, source crawling, authentication, admin routes, or production deployment. Those concerns are explicit interfaces or later plans, not silent fallbacks.

## Architecture decisions

### API boundary

Gin owns HTTP parsing and response serialization. The handler validates request shape and maps domain errors to stable `{code, message}` errors. The service owns upload validation, temporary file lifetime, scan lifecycle transitions, and the processor boundary. The repository owns PostgreSQL queries and no other layer builds SQL.

The client contract is:

```json
POST /api/v1/client/scans
{
  "scan_id": "uuid",
  "status": "processing"
}
```

```json
GET /api/v1/client/scans/:scan_id
{
  "scan_id": "uuid",
  "status": "processing"
}
```

```json
GET /api/v1/client/scans/:scan_id
{
  "scan_id": "uuid",
  "status": "completed",
  "matches": [
    {
      "id": "uuid",
      "match_percent": 87.5,
      "title": "...",
      "company": "...",
      "location": "...",
      "distance_km": 4.2,
      "employment_type": "Full-time",
      "work_mode": "onsite",
      "salary": { "display": "...", "currency": "VND" },
      "skill_tags": ["Go", "PostgreSQL"],
      "original_url": "https://example.com/jobs/1"
    }
  ]
}
```

Non-2xx responses use the frontend-compatible top-level shape:

```json
{
  "code": "invalid_upload",
  "message": "Tệp CV không hợp lệ."
}
```

`RECEIVED`, `PARSING`, and `MATCHING` map to client status `processing`. `COMPLETED` maps to `completed`. `FAILED` maps to `failed`. Unknown or impossible database states are internal errors, not public strings.

### Processor boundary

```go
type ScanProcessor interface {
    Process(context.Context, uuid.UUID, string, string, float64) error
}
```

The repository atomically records the request and enters `PARSING`; this avoids a partially-created `RECEIVED` scan if the initial transition fails. If the commit acknowledgement is ambiguous, the repository verifies the locally known scan ID with a bounded recovery context and continues when the committed row is already `PARSING`. The processor receives a temporary path, scan ID, location, and kilometer radius. A successful processor must persist the validated structured profile, deterministic matches, and the final `COMPLETED` transition before returning. A failed processor returns a typed code, and the service retries the `FAILED` transition once before returning an internal processing error.

The initial binary injects `UnavailableProcessor`. It fails closed with `parser_not_configured`, which makes the API operationally honest until a real parser/matcher implementation is added. No seed data, fallback profile, sample job, or fake match is present.

### Database boundary

The repository targets the existing `database/migrations/000001_initial_schema.up.sql` schema:

- `scans` stores status, location text, radius in kilometers, and error code.
- `active_job_cache` is the only public job-read source.
- `scan_matches` is joined to `active_job_cache` for completed results.
- Raw CV bytes, raw CV paths, full JD text, and source payloads are never written.

All values from HTTP or path parameters use pgx placeholders. UUID path parameters are parsed before querying. Match responses are capped at 100 rows and skill tags are capped at three values at the API boundary.

### Security boundary

- Bind address defaults to `127.0.0.1:8080`.
- The request body is capped before multipart parsing; the actual copied file is capped independently.
- Only `.pdf`, `.docx`, and `.txt` are accepted. PDF and DOCX require their expected file signatures. Client MIME and filename are not trusted as storage paths.
- Temporary files use `os.CreateTemp` and are removed on every processing exit path.
- CORS allows configured explicit origins only; credentials are not enabled.
- POST intake and GET scan status are rate-limited per source IP with separate in-memory fixed windows and a hard bucket-cardinality cap.
- The server binds to loopback only until scan ownership/authentication is implemented; configuration rejects a non-loopback address rather than relying on UUID secrecy.
- Scan ownership/authentication is unresolved in the product contract. This local foundation therefore does not claim production-grade authorization; unguessable UUIDs are not treated as access control. Authentication and ownership must precede non-local public deployment.

## Operational behavior when PostgreSQL or parsing is unavailable

The API process fails to start if `DATABASE_URL` is missing or the configured database cannot be opened. `/healthz` returns a generic `503` when the pool cannot ping PostgreSQL. A valid intake can be recorded and then become `failed` with `parser_not_configured` until a real processor is wired. No error response contains a DSN, SQL statement, filesystem path, or provider response.

## Verification evidence required

- Unit and handler contract tests with in-memory test doubles only; no runtime fake data.
- `gofmt`, `go test ./...`, `go vet ./...` from `backend/`.
- Static audits proving no raw CV/full JD persistence and no seed/mock records.
- A security review of upload, SQL, CORS, rate limiting, error mapping, IDOR exposure, and dependency usage.
- A live PostgreSQL connection/apply check only if a server and migration runner are available; otherwise report it as an environment limitation.
