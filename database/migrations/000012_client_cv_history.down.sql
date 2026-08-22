BEGIN;

UPDATE public.scans
SET radius_km = 1
WHERE radius_km IS NULL;

ALTER TABLE public.scans
    ALTER COLUMN radius_km SET NOT NULL,
    ADD CONSTRAINT scans_radius_km_check CHECK (radius_km > 0);

DROP INDEX IF EXISTS public.scans_client_user_created_idx;

ALTER TABLE public.scans
    DROP CONSTRAINT IF EXISTS scans_client_user_fk,
    DROP COLUMN IF EXISTS client_user_id;

COMMIT;
