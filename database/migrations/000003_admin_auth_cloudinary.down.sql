BEGIN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.promotion_slides
        WHERE storage_provider = 'CLOUDINARY'
    ) THEN
        RAISE EXCEPTION 'Cannot roll back Cloudinary promotion storage while Cloudinary-backed rows exist; remove or migrate them first';
    END IF;
END
$$;

ALTER TABLE public.promotion_slides
    DROP CONSTRAINT promotion_slides_cloudinary_metadata_check,
    DROP CONSTRAINT promotion_slides_storage_provider_check,
    DROP COLUMN cloudinary_asset_id,
    DROP COLUMN cloudinary_secure_url,
    DROP COLUMN cloudinary_public_id,
    DROP COLUMN storage_provider,
    ALTER COLUMN image_bytes SET NOT NULL,
    ADD CONSTRAINT promotion_slides_image_bytes_check CHECK (octet_length(image_bytes) > 0);

DROP TABLE IF EXISTS public.admin_audit_events;
DROP TABLE IF EXISTS public.admin_sessions;
DROP TABLE IF EXISTS public.admin_users;

COMMIT;
