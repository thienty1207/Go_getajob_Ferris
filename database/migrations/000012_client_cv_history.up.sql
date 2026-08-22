BEGIN;

ALTER TABLE public.scans
    ADD COLUMN client_user_id uuid;

ALTER TABLE public.scans
    ADD CONSTRAINT scans_client_user_fk
        FOREIGN KEY (client_user_id) REFERENCES public.client_users (id) ON DELETE CASCADE;

CREATE INDEX scans_client_user_created_idx
ON public.scans (client_user_id, created_at DESC)
WHERE client_user_id IS NOT NULL;

ALTER TABLE public.scans
    DROP CONSTRAINT scans_radius_km_check,
    ALTER COLUMN radius_km DROP NOT NULL;

COMMENT ON COLUMN public.scans.client_user_id IS 'Authenticated client owner for new scans; NULL is retained only for legacy anonymous rows.';
COMMENT ON COLUMN public.scans.radius_km IS 'Deprecated compatibility column. Active client scans do not accept or apply a radius.';

COMMIT;
