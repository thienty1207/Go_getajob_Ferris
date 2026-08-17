BEGIN;

DROP VIEW IF EXISTS public.active_job_cache;
ALTER TABLE public.job_cache DROP CONSTRAINT IF EXISTS job_cache_location_fk;
ALTER TABLE public.job_cache DROP COLUMN IF EXISTS location_id;
DROP TABLE IF EXISTS public.locations;

CREATE VIEW public.active_job_cache AS
SELECT
    jobs.id,
    jobs.source_id,
    sources.source_key,
    jobs.source_job_key,
    jobs.content_hash,
    jobs.title,
    jobs.company,
    jobs.location_text,
    jobs.latitude,
    jobs.longitude,
    jobs.role,
    jobs.required_skills,
    jobs.preferred_skills,
    jobs.seniority,
    jobs.minimum_experience_years,
    jobs.domains,
    jobs.employment_type,
    jobs.work_mode,
    jobs.salary_min,
    jobs.salary_max,
    jobs.salary_currency,
    jobs.salary_period,
    jobs.salary_source_text,
    jobs.original_url,
    jobs.status,
    jobs.last_seen_at,
    jobs.updated_at
FROM public.job_cache AS jobs
JOIN public.job_sources AS sources ON sources.id = jobs.source_id
WHERE sources.approval_status = 'ACTIVE'
  AND jobs.status = 'ACTIVE';

COMMIT;
