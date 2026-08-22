import { describe, expect, it } from 'vitest';
import { ApiError } from '../../shared/api/api-errors';
import { getClientLocations, getPromotions, getScanStatus, startScan } from './client-api';

function response(body: unknown, init: ResponseInit = {}) {
	return new Response(JSON.stringify(body), {
		status: 200,
		headers: { 'content-type': 'application/json' },
		...init
	});
}

describe('client API contract', () => {
	it('submits the CV as multipart form data without inventing a response', async () => {
		let request: Request | undefined;
		const fetcher: typeof fetch = async (input, init) => {
			const url = typeof input === 'string' && input.startsWith('/') ? `http://localhost${input}` : input;
			request = new Request(url, init);
			return response({ scan_id: 'scan-1', status: 'processing' }, { status: 202 });
		};

		const result = await startScan(
			{ file: new File(['cv'], 'resume.pdf'), locationId: 'location-hanoi' },
			'client-csrf-token',
			fetcher
		);

		expect(result).toEqual({ scanId: 'scan-1', status: 'processing' });
		expect(request?.method).toBe('POST');
		expect(request?.credentials).toBe('include');
		expect(request?.headers.get('x-csrf-token')).toBe('client-csrf-token');
		expect(request?.url).toContain('/api/v1/client/scans');
		expect(request?.headers.get('content-type')).toContain('multipart/form-data');
		const form = await request?.formData();
		expect(form?.get('location_id')).toBe('location-hanoi');
		expect(form?.get('radius_km')).toBeNull();
	});

	it('reads canonical active locations from the backend', async () => {
		const result = await getClientLocations(async (input) => {
			expect(String(input)).toContain('/api/v1/client/locations');
			return response({
				items: [{ id: 'location-1', display_name: 'Hồ Chí Minh', province: 'Hồ Chí Minh', country: 'Vietnam' }]
			});
		});

		expect(result).toEqual([{ id: 'location-1', displayName: 'Hồ Chí Minh', province: 'Hồ Chí Minh', country: 'Vietnam' }]);
	});

	it('parses a processing response from the real status endpoint', async () => {
		const result = await getScanStatus('scan-1', async (_input, init) => {
			expect(init?.credentials).toBe('include');
			return response({ scan_id: 'scan-1', status: 'processing' });
		});

		expect(result).toEqual({ scanId: 'scan-1', status: 'processing' });
	});

	it('turns a non-success response into an ApiError', async () => {
		await expect(
			getScanStatus('scan-1', async () => response({ message: 'service unavailable' }, { status: 503 }))
		).rejects.toBeInstanceOf(ApiError);
	});

	it('parses completed results, keeps source salary, caps tags, and omits remote distance', async () => {
		const result = await getScanStatus('scan-1', async () =>
			response({
				scan_id: 'scan-1',
				status: 'completed',
				matches: [
					{
						id: 'job-from-api',
						match_percent: 82,
						title: 'Role from API',
						company: 'Company from API',
						location: 'Remote',
						distance_km: 999,
						employment_type: 'Full-time',
						work_mode: 'remote',
						salary: { display: 'Theo nguồn', currency: 'VND' },
						skill_tags: ['Skill A', 'Skill B', 'Skill C', 'Skill D'],
						original_url: 'https://jobs.example.test/role-from-api'
					}
				]
			})
		);

		expect(result).toMatchObject({
			status: 'completed',
			matches: [
				{
					matchPercent: 82,
					distanceKm: undefined,
					salary: { display: 'Theo nguồn', currency: 'VND' },
					skillTags: ['Skill A', 'Skill B', 'Skill C']
				}
			]
		});
	});

	it('parses an honest empty result and a source-declared failure', async () => {
		await expect(
			getScanStatus('scan-empty', async () => response({ scan_id: 'scan-empty', status: 'completed', matches: [] }))
		).resolves.toEqual({ scanId: 'scan-empty', status: 'completed', matches: [] });

		await expect(
			getScanStatus('scan-failed', async () => response({ scan_id: 'scan-failed', status: 'failed', message: 'Source unavailable' }))
		).resolves.toEqual({
			scanId: 'scan-failed',
			status: 'failed',
			message: 'Matching service không thể hoàn tất việc quét CV. Vui lòng thử lại.'
		});
	});

	it('rejects unsafe URLs and impossible match percentages at the API boundary', async () => {
		const unsafeUrl = getScanStatus('scan-unsafe', async () =>
			response({
				scan_id: 'scan-unsafe',
				status: 'completed',
				matches: [{
					id: 'job-from-api',
					match_percent: 82,
					title: 'Role from API',
					company: 'Company from API',
					location: 'Remote',
					employment_type: 'Full-time',
					work_mode: 'remote',
					original_url: 'javascript:alert(1)'
				}]
			})
		);
		await expect(unsafeUrl).rejects.toMatchObject({ code: 'invalid_job_response' });

		const impossibleMatch = getScanStatus('scan-invalid-match', async () =>
			response({
				scan_id: 'scan-invalid-match',
				status: 'completed',
				matches: [{
					id: 'job-from-api',
					match_percent: 101,
					title: 'Role from API',
					company: 'Company from API',
					location: 'Hà Nội',
					employment_type: 'Full-time',
					work_mode: 'onsite',
					distance_km: 5,
					original_url: 'https://jobs.example.test/role-from-api'
				}]
			})
		);
		await expect(impossibleMatch).rejects.toMatchObject({ code: 'invalid_job_response' });
	});

	it('parses up to three backend promotion slides and resolves their image URLs', async () => {
		const result = await getPromotions(async (input) => {
			expect(String(input)).toContain('/api/v1/client/promotions');
			return response({
				promotions: [
					{
						slot: 2,
						image_url: '/api/v1/client/promotions/2/image?v=bbb',
						alt_text: 'Slide hai',
						title: 'Tìm việc phù hợp',
						content_hash: 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
					},
					{
						slot: 1,
						image_url: '/api/v1/client/promotions/1/image?v=aaa',
						alt_text: 'Slide một',
						content_hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
					}
				]
			});
		});

		expect(result.map((promotion) => promotion.slot)).toEqual([1, 2]);
		expect(result[0].imageUrl).toMatch(/\/api\/v1\/client\/promotions\/1\/image\?v=aaa$/);
		expect(result[1].title).toBe('Tìm việc phù hợp');
	});

	it('rejects promotion responses with more than three slides or unsafe image URLs', async () => {
		const tooMany = getPromotions(async () =>
			response({
				promotions: [1, 2, 3, 4].map((slot) => ({
					slot,
					image_url: `/api/v1/client/promotions/${slot}/image`,
					alt_text: 'Slide',
					content_hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
				}))
			})
		);
		await expect(tooMany).rejects.toMatchObject({ code: 'invalid_promotion_response' });

		const unsafe = getPromotions(async () =>
			response({
				promotions: [{
					slot: 1,
					image_url: 'javascript:alert(1)',
					alt_text: 'Slide',
					content_hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
				}]
			})
		);
		await expect(unsafe).rejects.toMatchObject({ code: 'invalid_promotion_response' });

		const external = getPromotions(async () =>
			response({
				promotions: [{
					slot: 1,
					image_url: '//external.example/api/v1/client/promotions/1/image',
					alt_text: 'Slide',
					content_hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
				}]
			})
		);
		await expect(external).rejects.toMatchObject({ code: 'invalid_promotion_response' });
	});
});
