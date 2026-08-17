# Baron Review Findings

- `finding-ed3d71c5113e2f1f` - Structured profile JSONB is not shape- or privacy-constrained
- `finding-534faff68649668c` - PostgreSQL 16 apply/rollback compatibility is unverified
- `finding-0557051d271dcf84` - Lifecycle reconciliation counter is not indexed as designed
- `finding-463015fcfaaced96` - scan_matches.job_id lacks a supporting leading index
- `finding-f649548dadbde266` - Structured profile JSONB initially accepted arbitrary nested records outside the approved schema
- `finding-84a296523a004e51` - Public job reads could bypass source approval if they queried job_cache directly
- `finding-17cbe2a3898d260b` - Non-finite MAX_RADIUS_KM or direct radius input can bypass radius validation
- `finding-f6ed6ccebf663b5b` - Per-IP rate limiter has no hard cap for many active source keys
- `finding-4283ca99fc40704d` - Lifecycle persistence failures can leave scans stuck in RECEIVED or PARSING
- `finding-8c8d0f04477672f1` - Scan status and matches are accessible without ownership authentication
- `finding-ee0d1e8d07cb697f` - Scan retrieval lacked a dedicated GET rate limit
- `finding-d6d0d9500234cb88` - Initial scan lifecycle used an unsafe data-modifying CTE pattern
- `finding-10f9c5bb182487c1` - Failed-state retry was not idempotent after an ambiguous commit
- `finding-a3f87d2a15b84ba8` - Limiter cap rejected all new source keys while full
- `finding-2a462e488db2c71f` - Crawler DNS validation is TOCTOU-prone: Spider re-resolves source hostname after one public DNS preflight, leaving a DNS-rebinding SSRF path; destination binding and negative proof are required.
