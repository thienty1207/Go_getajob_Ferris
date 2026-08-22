BEGIN;

DROP INDEX IF EXISTS public.client_sessions_user_idx;
DROP INDEX IF EXISTS public.client_google_identities_user_idx;
DROP INDEX IF EXISTS public.client_google_identities_user_uidx;
DROP INDEX IF EXISTS public.client_google_identities_sub_uidx;

DROP TABLE IF EXISTS public.client_sessions;
DROP TABLE IF EXISTS public.client_google_identities;
DROP TABLE IF EXISTS public.client_users;

COMMIT;
