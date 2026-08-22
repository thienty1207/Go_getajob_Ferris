import { describe, expect, it } from 'vitest';
import { ApiError } from '../../shared/api/api-errors';
import { loadCurrentUser, logoutClient, googleLoginUrl } from './client-auth-api';

function jsonResponse(body: unknown, status = 200) {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

const sampleUser = {
	id: 'u-1',
	email: 'user@example.com',
	display_name: 'Example User',
	avatar_url: 'https://example.com/avatar.png',
	provider: 'google',
	created_at: '2026-01-01T00:00:00Z',
	last_login_at: '2026-01-01T00:00:00Z'
};

describe('client auth API', () => {
	it('loads the real session with credentials include and parses user + csrf', async () => {
		let sawCredentials = false;
		const fetcher: typeof fetch = (input, init) => {
			sawCredentials = sawCredentials || init?.credentials === 'include';
			expect(String(input)).toContain('/api/v1/client/auth/me');
			return Promise.resolve(jsonResponse({ user: sampleUser, csrf_token: 'csrf-abc' }));
		};

		const result = await loadCurrentUser(fetcher);
		expect(sawCredentials).toBe(true);
		expect(result?.csrfToken).toBe('csrf-abc');
		expect(result?.user.displayName).toBe('Example User');
		expect(result?.user.email).toBe('user@example.com');
	});

	it('returns null on an unauthenticated (401) response without fabricating a user', async () => {
		const result = await loadCurrentUser(() => Promise.resolve(jsonResponse({ code: 'client_unauthorized' }, 401)));
		expect(result).toBeNull();
	});

	it('propagates non-401 errors as ApiError', async () => {
		await expect(loadCurrentUser(() => Promise.resolve(jsonResponse({}, 500)))).rejects.toBeInstanceOf(ApiError);
	});

	it('sends the CSRF header on logout via a mutating POST', async () => {
		let request: Request | undefined;
		const fetcher: typeof fetch = async (input, init) => {
			request = new Request(String(input), init);
			return new Response(null, { status: 204 });
		};

		await logoutClient(fetcher, 'csrf-xyz');
		expect(request?.method).toBe('POST');
		expect(request?.headers.get('X-CSRF-Token')).toBe('csrf-xyz');
		expect(request?.credentials).toBe('include');
		expect(request?.url).toContain('/api/v1/client/auth/logout');
	});

	it('builds a google login URL to the backend start endpoint', () => {
		expect(googleLoginUrl()).toContain('/api/v1/client/auth/google');
	});
});
