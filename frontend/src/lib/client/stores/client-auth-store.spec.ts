import { describe, expect, it, vi, beforeEach } from 'vitest';
import { clientAuth, type ClientAuthState } from './client-auth-store';
import { loadCurrentUser, logoutClient } from '../api/client-auth-api';

vi.mock('../api/client-auth-api', () => ({
	loadCurrentUser: vi.fn(),
	logoutClient: vi.fn()
}));

const mockedLoad = vi.mocked(loadCurrentUser);
const mockedLogout = vi.mocked(logoutClient);

const sampleUser = {
	id: 'u-1',
	email: 'user@example.com',
	displayName: 'Example User',
	provider: 'google' as const,
	createdAt: '2026-01-01T00:00:00Z',
	lastLoginAt: '2026-01-01T00:00:00Z'
};

function readState(): ClientAuthState {
	let value!: ClientAuthState;
	clientAuth.subscribe((state) => {
		value = state;
	})();
	return value;
}

describe('client auth store', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('marks the session authenticated with the real user on a valid /me', async () => {
		mockedLoad.mockResolvedValue({ user: sampleUser, csrfToken: 'csrf-1' });
		await clientAuth.loadCurrentUser();
		const state = readState();
		expect(state.status).toBe('authenticated');
		expect(state.user?.email).toBe('user@example.com');
		expect(state.csrfToken).toBe('csrf-1');
	});

	it('coalesces concurrent same-tab bootstraps into one /me request', async () => {
		let releaseLoad!: (value: { user: typeof sampleUser; csrfToken: string }) => void;
		mockedLoad.mockReturnValue(
			new Promise((resolve) => {
				releaseLoad = resolve;
			})
		);

		const firstBootstrap = clientAuth.loadCurrentUser();
		const secondBootstrap = clientAuth.loadCurrentUser();
		releaseLoad({ user: sampleUser, csrfToken: 'stable-csrf' });
		await Promise.all([firstBootstrap, secondBootstrap]);
		expect(mockedLoad).toHaveBeenCalledTimes(1);
		expect(readState()).toMatchObject({ status: 'authenticated', csrfToken: 'stable-csrf' });
	});

	it('marks the session unauthenticated and clears the user when /me is 401', async () => {
		mockedLoad.mockResolvedValue(null);
		await clientAuth.loadCurrentUser();
		const state = readState();
		expect(state.status).toBe('unauthenticated');
		expect(state.user).toBeNull();
		expect(state.csrfToken).toBe('');
	});

	it('records an error state and never fabricates a user on network failure', async () => {
		mockedLoad.mockRejectedValue(new Error('network'));
		await clientAuth.loadCurrentUser();
		const state = readState();
		expect(state.status).toBe('error');
		expect(state.user).toBeNull();
	});

	it('clears the store after logout and calls the logout API with the csrf token', async () => {
		mockedLoad.mockResolvedValue({ user: sampleUser, csrfToken: 'csrf-3' });
		await clientAuth.loadCurrentUser();
		mockedLogout.mockResolvedValue();
		await clientAuth.logout();

		expect(mockedLogout).toHaveBeenCalledWith(undefined, 'csrf-3');
		const state = readState();
		expect(state.status).toBe('unknown');
		expect(state.user).toBeNull();
	});
});
