import { describe, expect, it } from 'vitest';
import { getClientCVHistory, deleteClientCV } from './client-cv-api';

function response(body: unknown, init: ResponseInit = {}) {
	return new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'content-type': 'application/json' },
		...init
	});
}

describe('client CV history API contract', () => {
	it('reads owner-scoped structured profile history with credentials', async () => {
		const result = await getClientCVHistory(1, 10, async (input, init) => {
			expect(String(input)).toContain('/api/v1/client/cv-history?page=1&page_size=10');
			expect(init?.credentials).toBe('include');
			return response({
				items: [{
					scan_id: 'scan-1',
					status: 'completed',
					location: 'Hồ Chí Minh',
					created_at: '2026-08-20T00:00:00Z',
					updated_at: '2026-08-20T00:01:00Z',
					match_count: 4,
					profile: { roles: ['Backend Developer'], skills: ['Go'], years_of_experience: 3, seniority: 'mid', domains: ['SaaS'], education: [], certifications: [] }
				}],
				page: 1,
				page_size: 10,
				total: 1
			});
		});

		expect(result.items[0]).toMatchObject({ scanId: 'scan-1', matchCount: 4, profile: { roles: ['Backend Developer'] } });
	});

	it('deletes only through the credentialed CSRF route', async () => {
		let request: Request | undefined;
		await deleteClientCV('scan-1', 'csrf-token', async (input, init) => {
			request = new Request(String(input), init);
			return response(undefined, { status: 204 });
		});

		expect(request?.method).toBe('DELETE');
		expect(request?.credentials).toBe('include');
		expect(request?.headers.get('X-CSRF-Token')).toBe('csrf-token');
	});
});
