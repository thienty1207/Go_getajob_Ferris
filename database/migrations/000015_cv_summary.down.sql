BEGIN;

ALTER TABLE public.structured_profiles
DROP CONSTRAINT IF EXISTS structured_profiles_summary_shape_check;

ALTER TABLE public.structured_profiles
DROP COLUMN IF EXISTS summary;

COMMIT;
