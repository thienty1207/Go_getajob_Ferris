BEGIN;

CREATE TABLE public.promotion_slides (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slot smallint NOT NULL UNIQUE,
    image_bytes bytea NOT NULL,
    mime_type text NOT NULL,
    content_hash text NOT NULL,
    alt_text text NOT NULL,
    eyebrow text,
    title text,
    body text,
    target_url text,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT promotion_slides_slot_check CHECK (slot BETWEEN 1 AND 3),
    CONSTRAINT promotion_slides_image_bytes_check CHECK (octet_length(image_bytes) > 0),
    CONSTRAINT promotion_slides_mime_type_check CHECK (mime_type IN ('image/png', 'image/jpeg', 'image/webp')),
    CONSTRAINT promotion_slides_content_hash_check CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT promotion_slides_alt_text_check CHECK (char_length(btrim(alt_text)) > 0 AND char_length(alt_text) <= 180),
    CONSTRAINT promotion_slides_eyebrow_length_check CHECK (eyebrow IS NULL OR char_length(eyebrow) <= 80),
    CONSTRAINT promotion_slides_title_length_check CHECK (title IS NULL OR char_length(title) <= 160),
    CONSTRAINT promotion_slides_body_length_check CHECK (body IS NULL OR char_length(body) <= 320),
    CONSTRAINT promotion_slides_target_url_check CHECK (
        target_url IS NULL
        OR target_url ~* '^https?://[^[:space:]/?#]+([/:?#]|$)'
    ),
    CONSTRAINT promotion_slides_target_url_no_credentials_check CHECK (
        target_url IS NULL
        OR target_url !~* '^https?://[^[:space:]]*@'
    )
);

CREATE INDEX promotion_slides_active_slot_idx
    ON public.promotion_slides (is_active, slot);

COMMENT ON TABLE public.promotion_slides IS 'Up to three admin-managed promotion images and bounded presentation metadata; starts empty and contains no seeded data.';
COMMENT ON COLUMN public.promotion_slides.image_bytes IS 'Validated PNG, JPEG, or WebP bytes uploaded through the admin boundary.';
COMMENT ON COLUMN public.promotion_slides.content_hash IS 'Lowercase SHA-256 hash used for cache validation and replacement detection.';

COMMIT;
