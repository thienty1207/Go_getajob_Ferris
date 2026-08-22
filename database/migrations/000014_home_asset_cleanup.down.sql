BEGIN;

-- Rollback removes only retry metadata. It does not delete provider assets or
-- mutate the Home rows that may still reference them.
DROP TABLE IF EXISTS public.home_asset_cleanup_queue;

COMMIT;
