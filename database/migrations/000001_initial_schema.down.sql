BEGIN;

DROP VIEW IF EXISTS public.active_job_cache;
DROP TABLE IF EXISTS public.scan_matches;
DROP TABLE IF EXISTS public.scans;
DROP TABLE IF EXISTS public.structured_profiles;
DROP TABLE IF EXISTS public.job_cache;
DROP TABLE IF EXISTS public.source_crawl_runs;
DROP TABLE IF EXISTS public.job_sources;
DROP FUNCTION IF EXISTS public.is_structured_record_array(jsonb, text[], integer);

-- pgcrypto is intentionally retained because it may be shared by other schemas.

COMMIT;
