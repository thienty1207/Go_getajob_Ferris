BEGIN;

ALTER TABLE public.locations
    ADD COLUMN canonical_key text,
    ADD COLUMN latitude numeric(9, 6),
    ADD COLUMN longitude numeric(9, 6);

UPDATE public.locations
SET canonical_key = lower(regexp_replace(btrim(display_name), '[^[:alnum:]]+', '-', 'g'))
WHERE canonical_key IS NULL;

ALTER TABLE public.locations
    ALTER COLUMN canonical_key SET NOT NULL;

ALTER TABLE public.locations
    ADD CONSTRAINT locations_canonical_key_not_blank CHECK (btrim(canonical_key) <> ''),
    ADD CONSTRAINT locations_coordinates_pair_check CHECK (
        (latitude IS NULL AND longitude IS NULL)
        OR (latitude IS NOT NULL AND longitude IS NOT NULL)
    ),
    ADD CONSTRAINT locations_latitude_check CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    ADD CONSTRAINT locations_longitude_check CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180);

CREATE UNIQUE INDEX locations_lookup_key_uidx
ON public.locations (canonical_key);

CREATE TABLE public.location_aliases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    location_id uuid NOT NULL,
    normalized_text text NOT NULL,
    alias_text text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT location_aliases_location_fk
        FOREIGN KEY (location_id) REFERENCES public.locations (id) ON DELETE CASCADE,
    CONSTRAINT location_aliases_normalized_not_blank CHECK (btrim(normalized_text) <> ''),
    CONSTRAINT location_aliases_alias_not_blank CHECK (btrim(alias_text) <> '')
);

CREATE UNIQUE INDEX location_aliases_normalized_uidx
ON public.location_aliases (normalized_text);

CREATE INDEX location_aliases_location_idx
ON public.location_aliases (location_id);

ALTER TABLE public.scans
    ADD COLUMN location_id uuid;

ALTER TABLE public.scans
    ADD CONSTRAINT scans_location_fk
        FOREIGN KEY (location_id) REFERENCES public.locations (id) ON DELETE RESTRICT;

CREATE INDEX scans_location_idx
ON public.scans (location_id, created_at DESC);

COMMENT ON COLUMN public.locations.canonical_key IS 'Stable lookup key generated from the canonical display name.';
COMMENT ON COLUMN public.locations.latitude IS 'Canonical location latitude used for kilometer filtering.';
COMMENT ON COLUMN public.locations.longitude IS 'Canonical location longitude used for kilometer filtering.';
COMMENT ON TABLE public.location_aliases IS 'Deterministic source-location variants mapped to an admin-approved canonical location.';
COMMENT ON COLUMN public.scans.location_id IS 'Canonical location selected by the client scan request; location_text remains the request snapshot.';

COMMIT;
