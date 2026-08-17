BEGIN;

UPDATE public.locations
SET canonical_key = lower(regexp_replace(btrim(display_name), '[^[:alnum:]]+', '-', 'g'))
WHERE canonical_key IS DISTINCT FROM lower(regexp_replace(btrim(display_name), '[^[:alnum:]]+', '-', 'g'));

COMMIT;
