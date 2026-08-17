BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION public.is_structured_record_array(
    value jsonb,
    allowed_keys text[],
    max_records integer
)
RETURNS boolean
LANGUAGE sql
IMMUTABLE
STRICT
AS $function$
    SELECT CASE
        WHEN jsonb_typeof(value) <> 'array' THEN false
        WHEN jsonb_array_length(value) > max_records THEN false
        ELSE NOT EXISTS (
            SELECT 1
            FROM jsonb_array_elements(value) AS item
            WHERE CASE
                WHEN jsonb_typeof(item) <> 'object' THEN true
                ELSE (
                    EXISTS (
                        SELECT 1
                        FROM jsonb_object_keys(item) AS key_name(key)
                        WHERE NOT (key = ANY (allowed_keys))
                    )
                    OR EXISTS (
                        SELECT 1
                        FROM jsonb_each(item) AS field(field_key, field_value)
                        WHERE jsonb_typeof(field_value) NOT IN ('string', 'number', 'boolean', 'null')
                           OR (
                               jsonb_typeof(field_value) = 'string'
                               AND char_length(field_value #>> '{}') > 512
                           )
                    )
                )
            END
        )
    END;
$function$;

CREATE TABLE public.job_sources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_key text NOT NULL,
    display_name text NOT NULL,
    base_url text NOT NULL,
    source_type text NOT NULL,
    approval_status text NOT NULL DEFAULT 'REVIEW',
    robots_url text,
    terms_url text,
    approved_at timestamptz,
    approved_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT job_sources_source_key_not_blank CHECK (btrim(source_key) <> ''),
    CONSTRAINT job_sources_display_name_not_blank CHECK (btrim(display_name) <> ''),
    CONSTRAINT job_sources_base_url_http_check CHECK (base_url ~* '^https?://[^[:space:]/?#]+([/:?#]|$)'),
    CONSTRAINT job_sources_robots_url_http_check CHECK (robots_url IS NULL OR robots_url ~* '^https?://[^[:space:]/?#]+([/:?#]|$)'),
    CONSTRAINT job_sources_terms_url_http_check CHECK (terms_url IS NULL OR terms_url ~* '^https?://[^[:space:]/?#]+([/:?#]|$)'),
    CONSTRAINT job_sources_source_type_check CHECK (source_type IN ('OFFICIAL_API', 'PUBLIC_FEED', 'ATS_FEED', 'EXPLICIT_PERMISSION', 'COMPANY_SUBMITTED')),
    CONSTRAINT job_sources_approval_status_check CHECK (approval_status IN ('REVIEW', 'ACTIVE', 'DISABLED')),
    CONSTRAINT job_sources_approval_evidence_check CHECK (approval_status <> 'ACTIVE' OR approved_at IS NOT NULL),
    CONSTRAINT job_sources_approval_actor_check CHECK (approval_status <> 'ACTIVE' OR btrim(coalesce(approved_by, '')) <> '')
);

CREATE UNIQUE INDEX job_sources_source_key_uidx ON public.job_sources (source_key);
CREATE INDEX job_sources_approval_status_idx ON public.job_sources (approval_status);

CREATE TABLE public.source_crawl_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id uuid NOT NULL,
    run_status text NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    pages_seen integer NOT NULL DEFAULT 0,
    jobs_seen integer NOT NULL DEFAULT 0,
    jobs_created integer NOT NULL DEFAULT 0,
    jobs_updated integer NOT NULL DEFAULT 0,
    jobs_missing integer NOT NULL DEFAULT 0,
    error_code text,
    CONSTRAINT source_crawl_runs_source_fk FOREIGN KEY (source_id) REFERENCES public.job_sources (id) ON DELETE RESTRICT,
    CONSTRAINT source_crawl_runs_status_check CHECK (run_status IN ('HEALTHY', 'SOURCE_ERROR', 'PARSER_ERROR', 'ANOMALY')),
    CONSTRAINT source_crawl_runs_finished_after_start_check CHECK (finished_at IS NULL OR finished_at >= started_at),
    CONSTRAINT source_crawl_runs_pages_seen_check CHECK (pages_seen >= 0),
    CONSTRAINT source_crawl_runs_jobs_seen_check CHECK (jobs_seen >= 0),
    CONSTRAINT source_crawl_runs_jobs_created_check CHECK (jobs_created >= 0),
    CONSTRAINT source_crawl_runs_jobs_updated_check CHECK (jobs_updated >= 0),
    CONSTRAINT source_crawl_runs_jobs_missing_check CHECK (jobs_missing >= 0),
    CONSTRAINT source_crawl_runs_error_code_check CHECK (run_status = 'HEALTHY' OR btrim(coalesce(error_code, '')) <> '')
);

CREATE INDEX source_crawl_runs_source_started_idx ON public.source_crawl_runs (source_id, started_at DESC);
CREATE INDEX source_crawl_runs_status_idx ON public.source_crawl_runs (run_status, started_at DESC);

CREATE TABLE public.job_cache (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id uuid NOT NULL,
    source_job_key text NOT NULL,
    content_hash text NOT NULL,
    title text NOT NULL,
    company text NOT NULL,
    location_text text NOT NULL,
    latitude numeric(9, 6),
    longitude numeric(9, 6),
    role text NOT NULL,
    required_skills text[] NOT NULL DEFAULT '{}'::text[],
    preferred_skills text[] NOT NULL DEFAULT '{}'::text[],
    seniority text NOT NULL,
    minimum_experience_years numeric(5, 2),
    domains text[] NOT NULL DEFAULT '{}'::text[],
    employment_type text NOT NULL,
    work_mode text NOT NULL,
    salary_min numeric(18, 2),
    salary_max numeric(18, 2),
    salary_currency text,
    salary_period text,
    salary_source_text text,
    original_url text NOT NULL,
    status text NOT NULL DEFAULT 'ACTIVE',
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    missing_healthy_cycles smallint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT job_cache_source_fk FOREIGN KEY (source_id) REFERENCES public.job_sources (id) ON DELETE RESTRICT,
    CONSTRAINT job_cache_source_job_key_unique UNIQUE (source_id, source_job_key),
    CONSTRAINT job_cache_source_job_key_not_blank CHECK (btrim(source_job_key) <> ''),
    CONSTRAINT job_cache_content_hash_not_blank CHECK (btrim(content_hash) <> ''),
    CONSTRAINT job_cache_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT job_cache_company_not_blank CHECK (btrim(company) <> ''),
    CONSTRAINT job_cache_location_not_blank CHECK (btrim(location_text) <> ''),
    CONSTRAINT job_cache_role_not_blank CHECK (btrim(role) <> ''),
    CONSTRAINT job_cache_seniority_not_blank CHECK (btrim(seniority) <> ''),
    CONSTRAINT job_cache_employment_type_not_blank CHECK (btrim(employment_type) <> ''),
    CONSTRAINT job_cache_work_mode_check CHECK (work_mode IN ('REMOTE', 'HYBRID', 'ONSITE')),
    CONSTRAINT job_cache_coordinates_pair_check CHECK ((latitude IS NULL AND longitude IS NULL) OR (latitude IS NOT NULL AND longitude IS NOT NULL)),
    CONSTRAINT job_cache_latitude_check CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CONSTRAINT job_cache_longitude_check CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180),
    CONSTRAINT job_cache_experience_check CHECK (minimum_experience_years IS NULL OR minimum_experience_years BETWEEN 0 AND 100),
    CONSTRAINT job_cache_salary_min_check CHECK (salary_min IS NULL OR salary_min >= 0),
    CONSTRAINT job_cache_salary_max_check CHECK (salary_max IS NULL OR salary_max >= 0),
    CONSTRAINT job_cache_salary_range_check CHECK (salary_min IS NULL OR salary_max IS NULL OR salary_min <= salary_max),
    CONSTRAINT job_cache_salary_currency_required_check CHECK ((salary_min IS NULL AND salary_max IS NULL) OR salary_currency IS NOT NULL),
    CONSTRAINT job_cache_salary_currency_check CHECK (salary_currency IS NULL OR (btrim(salary_currency) <> '' AND char_length(salary_currency) <= 16)),
    CONSTRAINT job_cache_original_url_http_check CHECK (original_url ~* '^https?://[^[:space:]/?#]+([/:?#]|$)'),
    CONSTRAINT job_cache_status_check CHECK (status IN ('ACTIVE', 'VERIFYING', 'CLOSED', 'EXPIRED', 'DISABLED')),
    CONSTRAINT job_cache_missing_cycles_check CHECK (missing_healthy_cycles BETWEEN 0 AND 2),
    CONSTRAINT job_cache_array_values_check CHECK (
        array_position(required_skills, NULL) IS NULL
        AND array_position(preferred_skills, NULL) IS NULL
        AND array_position(domains, NULL) IS NULL
    )
);

CREATE INDEX job_cache_active_idx ON public.job_cache (status, last_seen_at DESC) WHERE status = 'ACTIVE';
CREATE INDEX job_cache_source_status_idx ON public.job_cache (source_id, status, last_seen_at DESC);
CREATE INDEX job_cache_content_hash_idx ON public.job_cache (content_hash);
CREATE INDEX job_cache_missing_cycles_idx ON public.job_cache (missing_healthy_cycles, last_seen_at DESC) WHERE status IN ('ACTIVE', 'VERIFYING');

CREATE TABLE public.structured_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    roles text[] NOT NULL DEFAULT '{}'::text[],
    skills text[] NOT NULL DEFAULT '{}'::text[],
    years_of_experience numeric(5, 2) NOT NULL DEFAULT 0,
    seniority text NOT NULL,
    domains text[] NOT NULL DEFAULT '{}'::text[],
    education jsonb NOT NULL DEFAULT '[]'::jsonb,
    certifications jsonb NOT NULL DEFAULT '[]'::jsonb,
    schema_version text NOT NULL,
    parser_model text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    CONSTRAINT structured_profiles_roles_values_check CHECK (array_position(roles, NULL) IS NULL),
    CONSTRAINT structured_profiles_skills_values_check CHECK (array_position(skills, NULL) IS NULL),
    CONSTRAINT structured_profiles_domains_values_check CHECK (array_position(domains, NULL) IS NULL),
    CONSTRAINT structured_profiles_experience_check CHECK (years_of_experience BETWEEN 0 AND 100),
    CONSTRAINT structured_profiles_seniority_not_blank CHECK (btrim(seniority) <> ''),
    CONSTRAINT structured_profiles_education_shape_check CHECK (
        public.is_structured_record_array(
            education,
            ARRAY['institution', 'degree', 'field_of_study', 'start_year', 'end_year', 'grade']::text[],
            20
        )
    ),
    CONSTRAINT structured_profiles_certifications_shape_check CHECK (
        public.is_structured_record_array(
            certifications,
            ARRAY['certificate_name', 'issuer', 'issued_year', 'expires_year']::text[],
            20
        )
    ),
    CONSTRAINT structured_profiles_schema_version_not_blank CHECK (btrim(schema_version) <> ''),
    CONSTRAINT structured_profiles_parser_model_not_blank CHECK (btrim(parser_model) <> ''),
    CONSTRAINT structured_profiles_expiry_check CHECK (expires_at IS NULL OR expires_at >= created_at)
);

CREATE INDEX structured_profiles_expires_idx ON public.structured_profiles (expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE public.scans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status text NOT NULL DEFAULT 'RECEIVED',
    profile_id uuid,
    location_text text NOT NULL,
    latitude numeric(9, 6),
    longitude numeric(9, 6),
    radius_km numeric(7, 2) NOT NULL,
    error_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz,
    CONSTRAINT scans_profile_fk FOREIGN KEY (profile_id) REFERENCES public.structured_profiles (id) ON DELETE RESTRICT,
    CONSTRAINT scans_status_check CHECK (status IN ('RECEIVED', 'PARSING', 'MATCHING', 'COMPLETED', 'FAILED')),
    CONSTRAINT scans_profile_ready_check CHECK (status NOT IN ('MATCHING', 'COMPLETED') OR profile_id IS NOT NULL),
    CONSTRAINT scans_location_not_blank CHECK (btrim(location_text) <> ''),
    CONSTRAINT scans_coordinates_pair_check CHECK ((latitude IS NULL AND longitude IS NULL) OR (latitude IS NOT NULL AND longitude IS NOT NULL)),
    CONSTRAINT scans_latitude_check CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CONSTRAINT scans_longitude_check CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180),
    CONSTRAINT scans_radius_km_check CHECK (radius_km > 0),
    CONSTRAINT scans_failed_error_check CHECK (status <> 'FAILED' OR btrim(coalesce(error_code, '')) <> ''),
    CONSTRAINT scans_expiry_check CHECK (expires_at IS NULL OR expires_at >= created_at)
);

CREATE INDEX scans_status_updated_idx ON public.scans (status, updated_at DESC);
CREATE INDEX scans_profile_idx ON public.scans (profile_id, created_at DESC);
CREATE INDEX scans_expires_idx ON public.scans (expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE public.scan_matches (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scan_id uuid NOT NULL,
    job_id uuid NOT NULL,
    required_skills_points numeric(5, 2) NOT NULL,
    role_relevance_points numeric(5, 2) NOT NULL,
    experience_points numeric(5, 2) NOT NULL,
    seniority_points numeric(5, 2) NOT NULL,
    preferred_skills_domain_points numeric(5, 2) NOT NULL,
    match_percent numeric(5, 2) NOT NULL,
    distance_km numeric(8, 2),
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT scan_matches_scan_fk FOREIGN KEY (scan_id) REFERENCES public.scans (id) ON DELETE CASCADE,
    CONSTRAINT scan_matches_job_fk FOREIGN KEY (job_id) REFERENCES public.job_cache (id) ON DELETE RESTRICT,
    CONSTRAINT scan_matches_scan_job_unique UNIQUE (scan_id, job_id),
    CONSTRAINT scan_matches_required_skills_points_check CHECK (required_skills_points BETWEEN 0 AND 35),
    CONSTRAINT scan_matches_role_relevance_points_check CHECK (role_relevance_points BETWEEN 0 AND 25),
    CONSTRAINT scan_matches_experience_points_check CHECK (experience_points BETWEEN 0 AND 15),
    CONSTRAINT scan_matches_seniority_points_check CHECK (seniority_points BETWEEN 0 AND 15),
    CONSTRAINT scan_matches_preferred_skills_domain_points_check CHECK (preferred_skills_domain_points BETWEEN 0 AND 10),
    CONSTRAINT scan_matches_match_percent_check CHECK (match_percent BETWEEN 0 AND 100),
    CONSTRAINT scan_matches_score_sum_check CHECK (
        match_percent = required_skills_points
            + role_relevance_points
            + experience_points
            + seniority_points
            + preferred_skills_domain_points
    ),
    CONSTRAINT scan_matches_distance_check CHECK (distance_km IS NULL OR distance_km >= 0)
);

CREATE INDEX scan_matches_scan_score_idx ON public.scan_matches (scan_id, match_percent DESC, distance_km NULLS LAST);
CREATE INDEX scan_matches_job_idx ON public.scan_matches (job_id);

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

COMMENT ON TABLE public.job_sources IS 'Reviewed job-source registry. Future crawlers may use only ACTIVE rows.';
COMMENT ON TABLE public.source_crawl_runs IS 'Source-run health records used to distinguish missing jobs from source/parser failures.';
COMMENT ON TABLE public.job_cache IS 'Structured public job metadata and matching fields keyed by source identity and content hash.';
COMMENT ON TABLE public.structured_profiles IS 'Validated structured CV fields only; the original upload is outside this table.';
COMMENT ON TABLE public.scans IS 'Client scan requests and processing state; access control belongs to the future backend.';
COMMENT ON TABLE public.scan_matches IS 'Deterministic match scores and kilometer distance snapshots for a scan.';
COMMENT ON VIEW public.active_job_cache IS 'Public job-read contract: only ACTIVE jobs from ACTIVE approved sources are exposed.';

COMMENT ON COLUMN public.job_cache.salary_currency IS 'The source currency as received; no conversion is implied.';
COMMENT ON COLUMN public.job_cache.original_url IS 'The original employer/source URL shown to the client.';
COMMENT ON COLUMN public.job_cache.missing_healthy_cycles IS 'Consecutive missing observations from healthy crawl cycles, capped at the CLOSED threshold.';
COMMENT ON COLUMN public.scan_matches.match_percent IS 'CV Match percentage, not a probability of being hired.';
COMMENT ON COLUMN public.scan_matches.distance_km IS 'Distance from the scan location in kilometers.';

COMMIT;
