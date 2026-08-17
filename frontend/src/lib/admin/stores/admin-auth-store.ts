import { writable } from 'svelte/store';
import { ApiError } from '$lib/shared/api/api-errors';
import { adminLogin, adminLogout, getAdminMe } from '../api/admin-api';
import type { AdminAuthState } from '../api/admin-session';

const initialState: AdminAuthState = {
	status: 'checking',
	user: null,
	csrfToken: '',
	errorMessage: ''
};

export function createAdminAuthStore() {
	const store = writable<AdminAuthState>(initialState);
	let operationVersion = 0;

	return {
		subscribe: store.subscribe,
		async bootstrap() {
			const operation = ++operationVersion;
			try {
				const response = await getAdminMe();
				if (operation !== operationVersion) return false;
				store.set({ status: 'authenticated', user: response.admin, csrfToken: response.csrfToken, errorMessage: '' });
				return true;
			} catch (error) {
				if (operation !== operationVersion) return false;
				if (error instanceof ApiError && error.status === 401) {
					store.set({ status: 'anonymous', user: null, csrfToken: '', errorMessage: '' });
					return false;
				}
				store.update((state) => ({ ...state, status: 'error', errorMessage: errorMessage(error) }));
				return false;
			}
		},
		async login(email: string, password: string) {
			const operation = ++operationVersion;
			store.update((state) => ({ ...state, status: 'checking', errorMessage: '' }));
			try {
				const response = await adminLogin(email, password);
				if (operation !== operationVersion) return;
				store.set({ status: 'authenticated', user: response.admin, csrfToken: response.csrfToken, errorMessage: '' });
			} catch (error) {
				if (operation !== operationVersion) return;
				store.update((state) => ({ ...state, status: 'anonymous', errorMessage: errorMessage(error) }));
				throw error;
			}
		},
		async logout() {
			const operation = ++operationVersion;
			let csrfToken = '';
			const unsubscribe = store.subscribe((state) => (csrfToken = state.csrfToken));
			unsubscribe();
			try {
				if (csrfToken) await adminLogout(csrfToken);
			} finally {
				if (operation === operationVersion) {
					store.set({ status: 'anonymous', user: null, csrfToken: '', errorMessage: '' });
				}
			}
		},
		expire() {
			operationVersion += 1;
			store.set({ status: 'anonymous', user: null, csrfToken: '', errorMessage: 'Phiên quản trị đã hết hạn. Vui lòng đăng nhập lại.' });
		}
	};
}

export type AdminAuthStore = ReturnType<typeof createAdminAuthStore>;

function errorMessage(error: unknown): string {
	if (error instanceof ApiError) return error.message;
	return 'Không thể hoàn tất thao tác quản trị. Vui lòng thử lại.';
}
