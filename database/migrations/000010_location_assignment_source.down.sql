BEGIN;

ALTER TABLE public.job_cache DROP CONSTRAINT IF EXISTS job_cache_location_assignment_source_check;

ALTER TABLE public.job_cache DROP COLUMN IF EXISTS location_assignment_source;

COMMIT;
