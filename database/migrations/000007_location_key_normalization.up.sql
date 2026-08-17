BEGIN;

CREATE EXTENSION IF NOT EXISTS unaccent;

UPDATE public.locations
SET canonical_key = btrim(
    lower(
        regexp_replace(unaccent(display_name), '[^A-Za-z0-9]+', ' ', 'g')
    )
)
WHERE canonical_key IS DISTINCT FROM btrim(
    lower(
        regexp_replace(unaccent(display_name), '[^A-Za-z0-9]+', ' ', 'g')
    )
);

COMMENT ON EXTENSION unaccent IS 'Used to keep PostgreSQL location keys equivalent to the Go and Rust accent-insensitive normalizers.';

COMMIT;
