BEGIN;

CREATE TABLE public.locations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name text NOT NULL,
    province text NOT NULL,
    country text NOT NULL DEFAULT 'Vietnam',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT locations_display_name_not_blank CHECK (btrim(display_name) <> ''),
    CONSTRAINT locations_province_not_blank CHECK (btrim(province) <> ''),
    CONSTRAINT locations_country_not_blank CHECK (btrim(country) <> '')
);

CREATE UNIQUE INDEX locations_canonical_key_uidx
ON public.locations (
    lower(btrim(display_name)),
    lower(btrim(province)),
    lower(btrim(country))
);

ALTER TABLE public.job_cache
ADD COLUMN location_id uuid;

ALTER TABLE public.job_cache
ADD CONSTRAINT job_cache_location_fk
FOREIGN KEY (location_id) REFERENCES public.locations (id) ON DELETE SET NULL;

CREATE INDEX job_cache_location_idx ON public.job_cache (location_id) WHERE location_id IS NOT NULL;

DROP VIEW IF EXISTS public.active_job_cache;

CREATE VIEW public.active_job_cache AS
SELECT
    jobs.id,
    jobs.source_id,
    sources.source_key,
    jobs.source_job_key,
    jobs.content_hash,
    jobs.title,
    jobs.company,
    COALESCE(locations.display_name, jobs.location_text) AS location_text,
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
    jobs.updated_at,
    jobs.location_id,
    jobs.location_text AS source_location_text
FROM public.job_cache AS jobs
JOIN public.job_sources AS sources ON sources.id = jobs.source_id
LEFT JOIN public.locations AS locations ON locations.id = jobs.location_id
WHERE sources.approval_status = 'ACTIVE'
  AND jobs.status = 'ACTIVE';

COMMENT ON TABLE public.locations IS 'Admin-approved canonical Vietnamese job locations.';
COMMENT ON COLUMN public.job_cache.location_id IS 'Optional admin-approved canonical location; location_text remains the source value.';
COMMENT ON COLUMN public.active_job_cache.source_location_text IS 'Original location text received from the job source.';

COMMIT;
