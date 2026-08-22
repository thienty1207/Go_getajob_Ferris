BEGIN;

-- Location ownership marker on job_cache.
--
-- The crawler resolves a source location_text to a canonical public.locations
-- row at ingest time. Before this migration every upsert overwrote
-- job_cache.location_id with the crawler's (possibly NULL) resolution, which
-- erased locations that an admin assigned directly in the Job Cache console.
--
-- This column records whether location_id is owned by automatic resolution
-- (AUTO) or by an explicit admin assignment (ADMIN). The crawler only owns the
-- value for AUTO rows; ADMIN rows keep the canonical location the admin chose
-- across re-crawls, restarts, and lifecycle transitions.
--
-- Values are constrained to the single source of truth; there is no third
-- "unset" state deliberately, so the default AUTO is the correct value for
-- every existing row (their location_id, if any, was written by the crawler).

ALTER TABLE public.job_cache
ADD COLUMN location_assignment_source text NOT NULL DEFAULT 'AUTO';

ALTER TABLE public.job_cache
ADD CONSTRAINT job_cache_location_assignment_source_check
CHECK (location_assignment_source IN ('AUTO', 'ADMIN'));

COMMENT ON COLUMN public.job_cache.location_assignment_source IS
'Ownership of location_id: AUTO = determined by crawler resolution, ADMIN = set by an operator. The crawler preserves ADMIN locations and never lets an unresolved location NULL overwrite an existing value.';

COMMIT;
