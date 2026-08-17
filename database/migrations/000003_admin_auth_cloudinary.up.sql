BEGIN;

CREATE TABLE public.admin_users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    password_hash text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT admin_users_email_check CHECK (
        char_length(btrim(email)) BETWEEN 5 AND 320
        AND email ~* '^[^[:space:]@]+@[^[:space:]@]+\.[^[:space:]@]+$'
    ),
    CONSTRAINT admin_users_password_hash_check CHECK (char_length(password_hash) BETWEEN 20 AND 255)
);

CREATE UNIQUE INDEX admin_users_email_lower_uidx ON public.admin_users (lower(email));

CREATE TABLE public.admin_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id uuid NOT NULL,
    token_hash bytea NOT NULL,
    csrf_token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT admin_sessions_user_fk FOREIGN KEY (admin_user_id) REFERENCES public.admin_users (id) ON DELETE CASCADE,
    CONSTRAINT admin_sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT admin_sessions_csrf_hash_check CHECK (octet_length(csrf_token_hash) = 32),
    CONSTRAINT admin_sessions_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT admin_sessions_revoked_check CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE UNIQUE INDEX admin_sessions_token_hash_uidx ON public.admin_sessions (token_hash);
CREATE INDEX admin_sessions_active_expiry_idx ON public.admin_sessions (expires_at) WHERE revoked_at IS NULL;
CREATE INDEX admin_sessions_user_idx ON public.admin_sessions (admin_user_id, created_at DESC);

CREATE TABLE public.admin_audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id uuid,
    action text NOT NULL,
    result text NOT NULL,
    resource_type text,
    resource_key text,
    request_id text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT admin_audit_user_fk FOREIGN KEY (admin_user_id) REFERENCES public.admin_users (id) ON DELETE SET NULL,
    CONSTRAINT admin_audit_action_check CHECK (btrim(action) <> '' AND char_length(action) <= 80),
    CONSTRAINT admin_audit_result_check CHECK (result IN ('SUCCESS', 'FAILURE')),
    CONSTRAINT admin_audit_resource_type_check CHECK (resource_type IS NULL OR char_length(resource_type) <= 80),
    CONSTRAINT admin_audit_resource_key_check CHECK (resource_key IS NULL OR char_length(resource_key) <= 160),
    CONSTRAINT admin_audit_request_id_check CHECK (request_id IS NULL OR char_length(request_id) <= 120)
);

CREATE INDEX admin_audit_created_idx ON public.admin_audit_events (created_at DESC);
CREATE INDEX admin_audit_user_created_idx ON public.admin_audit_events (admin_user_id, created_at DESC);

ALTER TABLE public.promotion_slides
    ADD COLUMN storage_provider text NOT NULL DEFAULT 'DATABASE',
    ADD COLUMN cloudinary_public_id text,
    ADD COLUMN cloudinary_secure_url text,
    ADD COLUMN cloudinary_asset_id text;

ALTER TABLE public.promotion_slides
    ALTER COLUMN image_bytes DROP NOT NULL,
    DROP CONSTRAINT promotion_slides_image_bytes_check,
    ADD CONSTRAINT promotion_slides_storage_provider_check CHECK (storage_provider IN ('DATABASE', 'CLOUDINARY')),
    ADD CONSTRAINT promotion_slides_cloudinary_metadata_check CHECK (
        (storage_provider = 'DATABASE' AND image_bytes IS NOT NULL AND cloudinary_public_id IS NULL AND cloudinary_secure_url IS NULL AND cloudinary_asset_id IS NULL)
        OR
        (storage_provider = 'CLOUDINARY'
            AND image_bytes IS NULL
            AND cloudinary_public_id IS NOT NULL
            AND char_length(btrim(cloudinary_public_id)) BETWEEN 1 AND 255
            AND cloudinary_secure_url IS NOT NULL
            AND cloudinary_secure_url ~* '^https://[^[:space:]@]+$'
            AND cloudinary_asset_id IS NOT NULL
            AND char_length(btrim(cloudinary_asset_id)) BETWEEN 1 AND 255)
    );

COMMENT ON TABLE public.admin_users IS 'Admin identities. Passwords are bcrypt hashes and are provisioned through the backend CLI, never stored as plaintext.';
COMMENT ON TABLE public.admin_sessions IS 'Hashed, revocable admin sessions. Raw session and CSRF tokens never persist.';
COMMENT ON TABLE public.admin_audit_events IS 'Safe audit trail for admin actions; excludes credentials, cookies, raw CV, and full JD content.';
COMMENT ON COLUMN public.promotion_slides.image_bytes IS 'Legacy database storage retained only for reversible rollout; new writes must use Cloudinary.';
COMMENT ON COLUMN public.promotion_slides.cloudinary_secure_url IS 'Validated HTTPS delivery URL returned by the server-side Cloudinary upload.';

COMMIT;
