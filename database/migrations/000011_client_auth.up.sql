BEGIN;

-- Client identity and session domain (Google login).
--
-- Kept fully separate from the admin domain (admin_users / admin_sessions):
-- a client session cookie must never be accepted as an admin session and vice
-- versa. Identity is anchored to the Google `sub` claim, not email, because an
-- email address can change; email is stored normalized (lowercase) for display
-- and lookup only. No password, access token, refresh token, or raw OAuth
-- response is persisted.

CREATE TABLE public.client_users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    display_name text NOT NULL,
    avatar_url text,
    provider text NOT NULL DEFAULT 'google',
    created_at timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_users_email_check CHECK (
        btrim(email) <> '' AND char_length(email) <= 254
    ),
    CONSTRAINT client_users_display_name_check CHECK (
        btrim(display_name) <> '' AND char_length(display_name) <= 200
    ),
    CONSTRAINT client_users_avatar_url_check CHECK (
        avatar_url IS NULL OR (btrim(avatar_url) <> '' AND char_length(avatar_url) <= 2048)
    ),
    CONSTRAINT client_users_provider_check CHECK (provider IN ('google'))
);

-- Email is normalized and stored for display/lookup only; it is deliberately
-- NOT a unique key because the identity anchor is the Google `sub`.

CREATE TABLE public.client_google_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_user_id uuid NOT NULL,
    google_sub text NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    avatar_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_google_identities_user_fk
        FOREIGN KEY (client_user_id) REFERENCES public.client_users (id) ON DELETE CASCADE,
    CONSTRAINT client_google_identities_google_sub_not_blank CHECK (
        btrim(google_sub) <> '' AND char_length(google_sub) <= 255
    ),
    CONSTRAINT client_google_identities_email_check CHECK (
        btrim(email) <> '' AND char_length(email) <= 254
    ),
    CONSTRAINT client_google_identities_display_name_check CHECK (
        btrim(display_name) <> '' AND char_length(display_name) <= 200
    ),
    CONSTRAINT client_google_identities_avatar_url_check CHECK (
        avatar_url IS NULL OR (btrim(avatar_url) <> '' AND char_length(avatar_url) <= 2048)
    )
);

-- Stable identity: one Google subject maps to exactly one client user, and
-- one client user maps back to exactly one identity.
CREATE UNIQUE INDEX client_google_identities_sub_uidx ON public.client_google_identities (google_sub);
CREATE UNIQUE INDEX client_google_identities_user_uidx ON public.client_google_identities (client_user_id);
CREATE INDEX client_google_identities_user_idx ON public.client_google_identities (client_user_id);

CREATE TABLE public.client_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_user_id uuid NOT NULL,
    token_hash bytea NOT NULL,
    csrf_token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_sessions_user_fk
        FOREIGN KEY (client_user_id) REFERENCES public.client_users (id) ON DELETE CASCADE,
    CONSTRAINT client_sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT client_sessions_csrf_hash_check CHECK (octet_length(csrf_token_hash) = 32),
    CONSTRAINT client_sessions_expiry_check CHECK (expires_at > created_at),
    CONSTRAINT client_sessions_revoked_check CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX client_sessions_user_idx ON public.client_sessions (client_user_id);

COMMENT ON TABLE public.client_users IS
'Client identity for Google login. Deliberately separate from admin_users; a client session never authorizes admin routes.';
COMMENT ON TABLE public.client_google_identities IS
'Stable Google identity (sub) attached to a client user. The Google subject is the identity anchor, not email.';
COMMENT ON TABLE public.client_sessions IS
'Client browser session keyed by a random raw token whose SHA-256 is stored. Cookie is HttpOnly; mutations require the CSRF token.';

COMMIT;
