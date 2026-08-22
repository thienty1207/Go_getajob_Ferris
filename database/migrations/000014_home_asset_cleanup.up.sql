BEGIN;

-- Provider deletion is deliberately separated from Home metadata writes.
-- Replacements/deletes enqueue only a Cloudinary public ID in the same
-- transaction; no delivery URL, credential URL, or provider error is stored.
CREATE TABLE public.home_asset_cleanup_queue (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cloudinary_public_id text NOT NULL UNIQUE,
    attempt_count integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT home_asset_cleanup_public_id_check CHECK (
        char_length(btrim(cloudinary_public_id)) BETWEEN 1 AND 255
    ),
    CONSTRAINT home_asset_cleanup_attempt_count_check CHECK (attempt_count >= 0)
);

CREATE INDEX home_asset_cleanup_due_idx
    ON public.home_asset_cleanup_queue (next_attempt_at, created_at);

COMMENT ON TABLE public.home_asset_cleanup_queue IS
'Durable retry queue for unreferenced Home Cloudinary public IDs. Provider errors and credential-bearing URLs are never retained.';

COMMIT;
