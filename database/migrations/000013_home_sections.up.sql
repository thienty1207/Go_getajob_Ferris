BEGIN;

-- Home content is a fixed four-slot contract. The database stores plain text
-- and Cloudinary metadata only; it never stores arbitrary HTML or image bytes.
CREATE TABLE public.home_sections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slot smallint NOT NULL UNIQUE,
    layout text NOT NULL,
    is_active boolean NOT NULL DEFAULT false,
    eyebrow text,
    title text,
    body text,
    image_alt_text text,
    image_content_hash text,
    storage_provider text,
    cloudinary_public_id text,
    cloudinary_secure_url text,
    cloudinary_asset_id text,
    target_url text,
    updated_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT home_sections_id_slot_unique UNIQUE (id, slot),
    CONSTRAINT home_sections_slot_check CHECK (slot BETWEEN 1 AND 4),
    CONSTRAINT home_sections_layout_check CHECK (
        (slot IN (1, 3) AND layout = 'CONTENT_LEFT')
        OR (slot = 2 AND layout = 'IMAGE_LEFT')
        OR (slot = 4 AND layout = 'MEDIA_STRIP')
    ),
    CONSTRAINT home_sections_eyebrow_check CHECK (eyebrow IS NULL OR char_length(eyebrow) <= 80),
    CONSTRAINT home_sections_title_check CHECK (title IS NULL OR char_length(title) <= 180),
    CONSTRAINT home_sections_body_check CHECK (body IS NULL OR char_length(body) <= 1200),
    CONSTRAINT home_sections_alt_check CHECK (image_alt_text IS NULL OR char_length(btrim(image_alt_text)) BETWEEN 1 AND 180),
    CONSTRAINT home_sections_hash_check CHECK (image_content_hash IS NULL OR image_content_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT home_sections_storage_check CHECK (storage_provider IS NULL OR storage_provider = 'CLOUDINARY'),
    CONSTRAINT home_sections_cloudinary_check CHECK (
        (storage_provider IS NULL AND image_content_hash IS NULL AND cloudinary_public_id IS NULL AND cloudinary_secure_url IS NULL AND cloudinary_asset_id IS NULL)
        OR
        (storage_provider = 'CLOUDINARY'
            AND image_content_hash IS NOT NULL
            AND cloudinary_public_id IS NOT NULL
            AND cloudinary_secure_url IS NOT NULL
            AND cloudinary_secure_url ~* '^https://[^[:space:]@]+$'
            AND cloudinary_asset_id IS NOT NULL)
    ),
    CONSTRAINT home_sections_target_url_check CHECK (
        target_url IS NULL OR target_url ~* '^https?://[^[:space:]/?#]+([/:?#]|$)'
    ),
    CONSTRAINT home_sections_target_url_no_credentials_check CHECK (
        target_url IS NULL OR target_url !~* '^https?://[^[:space:]]*@'
    ),
    CONSTRAINT home_sections_updated_by_check CHECK (updated_by IS NULL OR char_length(btrim(updated_by)) BETWEEN 1 AND 320)
);

CREATE TABLE public.home_section_media (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    home_section_id uuid NOT NULL,
    section_slot smallint NOT NULL DEFAULT 4,
    sort_order smallint NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    image_alt_text text NOT NULL,
    image_content_hash text NOT NULL,
    storage_provider text NOT NULL DEFAULT 'CLOUDINARY',
    cloudinary_public_id text NOT NULL,
    cloudinary_secure_url text NOT NULL,
    cloudinary_asset_id text NOT NULL,
    target_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT home_section_media_section_fk
        FOREIGN KEY (home_section_id, section_slot)
        REFERENCES public.home_sections (id, slot) ON DELETE CASCADE,
    CONSTRAINT home_section_media_slot_check CHECK (section_slot = 4),
    CONSTRAINT home_section_media_order_check CHECK (sort_order BETWEEN 0 AND 11),
    CONSTRAINT home_section_media_alt_check CHECK (char_length(btrim(image_alt_text)) BETWEEN 1 AND 180),
    CONSTRAINT home_section_media_hash_check CHECK (image_content_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT home_section_media_storage_check CHECK (storage_provider = 'CLOUDINARY'),
    CONSTRAINT home_section_media_public_id_check CHECK (char_length(btrim(cloudinary_public_id)) BETWEEN 1 AND 255),
    CONSTRAINT home_section_media_url_check CHECK (cloudinary_secure_url ~* '^https://[^[:space:]@]+$'),
    CONSTRAINT home_section_media_asset_id_check CHECK (char_length(btrim(cloudinary_asset_id)) BETWEEN 1 AND 255),
    CONSTRAINT home_section_media_target_url_check CHECK (
        target_url IS NULL OR target_url ~* '^https?://[^[:space:]/?#]+([/:?#]|$)'
    ),
    CONSTRAINT home_section_media_target_url_no_credentials_check CHECK (
        target_url IS NULL OR target_url !~* '^https?://[^[:space:]]*@'
    )
);

CREATE UNIQUE INDEX home_section_media_order_uidx
    ON public.home_section_media (home_section_id, sort_order);
CREATE INDEX home_section_media_public_idx
    ON public.home_section_media (home_section_id, is_active, sort_order);

COMMENT ON TABLE public.home_sections IS
'Four fixed Home content slots. Slots 1-3 are alternating text/image blocks; slot 4 owns the horizontal media strip.';
COMMENT ON TABLE public.home_section_media IS
'Ordered Cloudinary metadata for Home slot 4. Raw image bytes and arbitrary HTML are never stored.';

COMMIT;
