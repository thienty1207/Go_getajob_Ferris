import { describe, expect, it } from 'vitest';
import { resolveApiUrl } from './api-url';

describe('resolveApiUrl', () => {
	it('uses the browser hostname for local API aliases so host-only cookies match', () => {
		expect(resolveApiUrl('/api/v1/admin/auth/me', 'http://127.0.0.1:8080', false, 'Invalid API config', 'localhost')).toBe(
			'http://localhost:8080/api/v1/admin/auth/me'
		);
		expect(resolveApiUrl('/api/v1/admin/auth/me', 'http://localhost:8080', false, 'Invalid API config', '127.0.0.1')).toBe(
			'http://127.0.0.1:8080/api/v1/admin/auth/me'
		);
	});

	it('does not rewrite non-local API hosts', () => {
		expect(resolveApiUrl('/api/v1/client/promotions', 'https://api.example.com', true, 'Invalid API config', 'localhost')).toBe(
			'https://api.example.com/api/v1/client/promotions'
		);
	});
});
