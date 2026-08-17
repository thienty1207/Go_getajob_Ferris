import { afterEach, describe, expect, it, vi } from 'vitest';
import type { AdminAuthState } from '../api/admin-session';
import { createAdminAuthStore } from './admin-auth-store';

function jsonResponse(body: unknown, status = 200) {
	return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('admin auth store', () => {
	it('keeps session state in memory and bootstraps from the real me endpoint', async () => {
		vi.stubGlobal('fetch', async (input: RequestInfo | URL) => {
			if (String(input).endsWith('/auth/me')) return jsonResponse({ admin: { id: '1', email: 'admin@example.com', is_active: true }, csrf_token: 'csrf' });
			return new Response(null, { status: 204 });
		});
		const store = createAdminAuthStore();
		await expect(store.bootstrap()).resolves.toBe(true);
		let state: AdminAuthState | undefined;
		const unsubscribe = store.subscribe((value) => (state = value));
		unsubscribe();
		expect(state?.status).toBe('authenticated');
		expect(state?.user?.email).toBe('admin@example.com');
		expect(state?.csrfToken).toBe('csrf');
	});

	it('clears state when the server returns an unauthorized session', async () => {
		vi.stubGlobal('fetch', async () => jsonResponse({ code: 'admin_auth_required' }, 401));
		const store = createAdminAuthStore();
		await expect(store.bootstrap()).resolves.toBe(false);
		let state: AdminAuthState | undefined;
		const unsubscribe = store.subscribe((value) => (state = value));
		unsubscribe();
		expect(state?.status).toBe('anonymous');
		expect(state?.csrfToken).toBe('');
	});

	it('does not let a stale bootstrap response erase a completed login', async () => {
		let releaseBootstrap!: (response: Response) => void;
		const bootstrapResponse = new Promise<Response>((resolve) => {
			releaseBootstrap = resolve;
		});
		vi.stubGlobal('fetch', async (input: RequestInfo | URL) => {
			if (String(input).endsWith('/auth/me')) return bootstrapResponse;
			return jsonResponse({
				admin: { id: '1', email: 'admin@example.com', is_active: true },
				csrf_token: 'csrf-after-login'
			});
		});

		const store = createAdminAuthStore();
		const bootstrapPromise = store.bootstrap();
		await Promise.resolve();
		await store.login('admin@example.com', 'a-valid-password');
		releaseBootstrap(jsonResponse({ code: 'admin_auth_required' }, 401));
		await bootstrapPromise;

		let state: AdminAuthState | undefined;
		const unsubscribe = store.subscribe((value) => (state = value));
		unsubscribe();
		expect(state?.status).toBe('authenticated');
		expect(state?.csrfToken).toBe('csrf-after-login');
	});
});
