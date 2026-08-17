BEGIN;

CREATE TABLE public.crawler_runtime (
    runtime_key text PRIMARY KEY,
    status text NOT NULL DEFAULT 'OFFLINE',
    last_heartbeat_at timestamptz,
    last_cycle_started_at timestamptz,
    last_cycle_finished_at timestamptz,
    next_cycle_at timestamptz,
    current_source_key text,
    last_error_code text,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT crawler_runtime_key_check CHECK (runtime_key = 'default'),
    CONSTRAINT crawler_runtime_status_check CHECK (status IN ('OFFLINE', 'IDLE', 'RUNNING', 'ERROR')),
    CONSTRAINT crawler_runtime_source_key_check CHECK (current_source_key IS NULL OR btrim(current_source_key) <> ''),
    CONSTRAINT crawler_runtime_error_code_check CHECK (last_error_code IS NULL OR btrim(last_error_code) <> '')
);

INSERT INTO public.crawler_runtime (runtime_key, status)
VALUES ('default', 'OFFLINE');

COMMIT;
