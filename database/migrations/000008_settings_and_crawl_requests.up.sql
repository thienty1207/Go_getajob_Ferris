BEGIN;

CREATE TABLE public.app_settings (
    setting_key text PRIMARY KEY,
    setting_group text NOT NULL,
    setting_value jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by text,
    CONSTRAINT app_settings_key_not_blank CHECK (btrim(setting_key) <> ''),
    CONSTRAINT app_settings_group_not_blank CHECK (btrim(setting_group) <> ''),
    CONSTRAINT app_settings_updated_by_not_blank CHECK (updated_by IS NULL OR btrim(updated_by) <> '')
);

CREATE INDEX app_settings_group_idx ON public.app_settings (setting_group, setting_key);

CREATE TABLE public.crawl_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id uuid NOT NULL,
    status text NOT NULL DEFAULT 'PENDING',
    requested_by text NOT NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    source_run_id uuid,
    error_code text,
    CONSTRAINT crawl_requests_source_fk FOREIGN KEY (source_id) REFERENCES public.job_sources (id) ON DELETE CASCADE,
    CONSTRAINT crawl_requests_source_run_fk FOREIGN KEY (source_run_id) REFERENCES public.source_crawl_runs (id) ON DELETE SET NULL,
    CONSTRAINT crawl_requests_status_check CHECK (status IN ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED')),
    CONSTRAINT crawl_requests_requested_by_not_blank CHECK (btrim(requested_by) <> ''),
    CONSTRAINT crawl_requests_finished_after_start_check CHECK (finished_at IS NULL OR (started_at IS NOT NULL AND finished_at >= started_at)),
    CONSTRAINT crawl_requests_error_code_check CHECK (status <> 'FAILED' OR btrim(coalesce(error_code, '')) <> '')
);

CREATE INDEX crawl_requests_status_requested_idx ON public.crawl_requests (status, requested_at, id);
CREATE INDEX crawl_requests_source_requested_idx ON public.crawl_requests (source_id, requested_at DESC, id DESC);
CREATE UNIQUE INDEX crawl_requests_active_source_uidx
    ON public.crawl_requests (source_id)
    WHERE status IN ('PENDING', 'RUNNING');

COMMIT;
