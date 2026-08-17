-- Development-only fixture for verifying the admin Job Cache screen.
-- This is deliberately not a migration and must never be treated as crawler output.
-- REVIEW/DISABLED keeps the row out of the public active_job_cache view.
BEGIN;

INSERT INTO public.job_sources (
    source_key,
    display_name,
    base_url,
    source_type,
    approval_status
)
VALUES (
    'development-fixture',
    'Development Fixture (not a live source)',
    'https://example.invalid',
    'COMPANY_SUBMITTED',
    'REVIEW'
)
ON CONFLICT (source_key) DO NOTHING;

WITH fixture_source AS (
    SELECT id
    FROM public.job_sources
    WHERE source_key = 'development-fixture'
)
INSERT INTO public.job_cache (
    source_id,
    source_job_key,
    content_hash,
    title,
    company,
    location_text,
    role,
    required_skills,
    preferred_skills,
    seniority,
    minimum_experience_years,
    domains,
    employment_type,
    work_mode,
    original_url,
    status,
    last_seen_at
)
SELECT
    id,
    'development-fixture-job-001',
    '0000000000000000000000000000000000000000000000000000000000000001',
    'Software Engineer - Development Fixture',
    'Development Fixture (not a live job)',
    'Ho Chi Minh City, Vietnam',
    'Software Engineer',
    ARRAY['Go', 'PostgreSQL']::text[],
    ARRAY['TypeScript', 'Docker']::text[],
    'MID',
    2,
    ARRAY['software', 'backend']::text[],
    'FULL_TIME',
    'HYBRID',
    'https://example.invalid/job/development-fixture-job-001',
    'DISABLED',
    now()
FROM fixture_source
ON CONFLICT (source_id, source_job_key) DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    title = EXCLUDED.title,
    company = EXCLUDED.company,
    location_text = EXCLUDED.location_text,
    role = EXCLUDED.role,
    required_skills = EXCLUDED.required_skills,
    preferred_skills = EXCLUDED.preferred_skills,
    seniority = EXCLUDED.seniority,
    minimum_experience_years = EXCLUDED.minimum_experience_years,
    domains = EXCLUDED.domains,
    employment_type = EXCLUDED.employment_type,
    work_mode = EXCLUDED.work_mode,
    original_url = EXCLUDED.original_url,
    status = 'DISABLED',
    last_seen_at = EXCLUDED.last_seen_at,
    updated_at = now();

COMMIT;
