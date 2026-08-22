import { writable, get, type Readable } from 'svelte/store';
import type { ClientUser } from '../../shared/types/client-auth';
import { loadCurrentUser, logoutClient } from '../api/client-auth-api';
import type { Fetcher } from '../../shared/api/http-client';

export type ClientAuthStatus = 'unknown' | 'loading' | 'authenticated' | 'unauthenticated' | 'error';

export interface ClientAuthState {
	user: ClientUser | null;
	status: ClientAuthStatus;
	csrfToken: string;
	error: boolean;
}

export interface ClientAuthStore extends Readable<ClientAuthState> {
	loadCurrentUser: (fetcher?: Fetcher) => Promise<void>;
	logout: (fetcher?: Fetcher) => Promise<void>;
}

function initialState(): ClientAuthState {
	return { user: null, status: 'unknown', csrfToken: '', error: false };
}

function createClientAuthStore(): ClientAuthStore {
	const { subscribe, set, update } = writable<ClientAuthState>(initialState());
	let bootstrapInFlight: Promise<void> | null = null;

	const store: ClientAuthStore = {
		subscribe,
		loadCurrentUser(fetcher) {
			// Header and page mounts can bootstrap together in one tab. Sharing the
			// in-flight request avoids duplicate /me work and one caller's state race.
			if (bootstrapInFlight) return bootstrapInFlight;
			update((state) => ({ ...state, status: 'loading', error: false }));
			bootstrapInFlight = (async () => {
				try {
					const response = await loadCurrentUser(fetcher);
					if (response) {
						set({ user: response.user, status: 'authenticated', csrfToken: response.csrfToken, error: false });
					} else {
						set({ user: null, status: 'unauthenticated', csrfToken: '', error: false });
					}
				} catch {
					set({ user: null, status: 'error', csrfToken: '', error: true });
				}
			})().finally(() => {
				bootstrapInFlight = null;
			});
			return bootstrapInFlight;
		},
		async logout(fetcher) {
			const csrfToken = get(store).csrfToken;
			await logoutClient(fetcher, csrfToken);
			set(initialState());
		}
	};

	return store;
}

export const clientAuth = createClientAuthStore();
