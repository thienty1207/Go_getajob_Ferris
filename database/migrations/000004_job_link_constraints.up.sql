BEGIN;

CREATE UNIQUE INDEX job_sources_active_base_url_uidx
    ON public.job_sources (base_url)
    WHERE approval_status <> 'DISABLED';

COMMENT ON INDEX public.job_sources_active_base_url_uidx IS 'One enabled Job Link per normalized URL; disabled history may be re-approved later.';

COMMIT;
