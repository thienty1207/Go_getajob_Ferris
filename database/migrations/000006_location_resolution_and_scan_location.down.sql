BEGIN;

DROP INDEX IF EXISTS public.scans_location_idx;
ALTER TABLE public.scans DROP CONSTRAINT IF EXISTS scans_location_fk;
ALTER TABLE public.scans DROP COLUMN IF EXISTS location_id;

DROP INDEX IF EXISTS public.location_aliases_location_idx;
DROP INDEX IF EXISTS public.location_aliases_normalized_uidx;
DROP TABLE IF EXISTS public.location_aliases;

ALTER TABLE public.locations
    DROP CONSTRAINT IF EXISTS locations_longitude_check,
    DROP CONSTRAINT IF EXISTS locations_latitude_check,
    DROP CONSTRAINT IF EXISTS locations_coordinates_pair_check,
    DROP CONSTRAINT IF EXISTS locations_canonical_key_not_blank;
DROP INDEX IF EXISTS public.locations_lookup_key_uidx;
ALTER TABLE public.locations
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude,
    DROP COLUMN IF EXISTS canonical_key;

COMMIT;
