BEGIN;

ALTER TABLE public.structured_profiles
ADD COLUMN summary jsonb;

ALTER TABLE public.structured_profiles
ADD CONSTRAINT structured_profiles_summary_shape_check CHECK (
    summary IS NULL
    OR (
        jsonb_typeof(summary) = 'object'
        AND summary ? 'headline'
        AND summary ? 'overview'
        AND summary ? 'target_roles'
        AND summary ? 'strengths'
        AND summary ? 'gaps'
        AND jsonb_typeof(summary->'headline') = 'string'
        AND jsonb_typeof(summary->'overview') = 'string'
        AND jsonb_typeof(summary->'target_roles') = 'array'
        AND jsonb_typeof(summary->'strengths') = 'array'
        AND jsonb_typeof(summary->'gaps') = 'array'
    )
);

COMMENT ON COLUMN public.structured_profiles.summary IS 'Bounded AI-generated CV summary; raw CV text and direct identity fields are excluded.';

COMMIT;
