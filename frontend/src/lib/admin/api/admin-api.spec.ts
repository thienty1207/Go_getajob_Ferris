import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '$lib/shared/api/api-errors';
import { createAdminHomeMedia, createAdminJobLink, deleteAdminHomeMedia, deleteAdminJobLink, deleteAdminCVProfile, getAdminClientUsers, getAdminCVProfiles, getAdminHomeSections, getAdminJobLinks, getAdminJobs, getAdminSettings, requestAdminJobLinkCrawl, setAdminJobLinkStatus, updateAdminCrawlerSettings, updateAdminHomeMedia, updateAdminHomeSection, updateAdminJobLink, uploadAdminPromotion } from './admin-api';

function jsonResponse(body: unknown, status = 200) {
	return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('admin API contract', () => {
	it('sends credentials and CSRF header for a real promotion upload', async () => {
		let request: Request | undefined;
		vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
			request = new Request(input, init);
			return jsonResponse({
				slot: 1,
				image_url: '/api/v1/client/promotions/1/image?v=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
				alt_text: 'Ferris promotion',
				content_hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
			});
		});

		const result = await uploadAdminPromotion(1, { file: new File(['png'], 'promotion.png', { type: 'image/png' }), altText: 'Ferris promotion' }, 'csrf-token');
		const body = await request?.formData();
		 expect(result.slot).toBe(1);
		expect(request?.method).toBe('PUT');
		expect(request?.credentials).toBe('include');
		expect(request?.headers.get('x-csrf-token')).toBe('csrf-token');
		expect(body?.get('alt_text')).toBe('Ferris promotion');
		expect(body?.get('eyebrow')).toBeNull();
		expect(body?.get('title')).toBeNull();
		expect(body?.get('body')).toBeNull();
		expect(body?.get('target_url')).toBeNull();
		expect(body?.get('image')).toBeInstanceOf(File);
	});

	it('parses structured admin job rows and does not invent data on API failure', async () => {
		vi.stubGlobal('fetch', async () => jsonResponse({
			items: [{
				id: 'job-1',
				source_key: 'development-fixture',
				source_name: 'Development Fixture',
				source_approval_status: 'REVIEW',
				is_development_fixture: true,
				title: 'Software Engineer - Development Fixture',
				company: 'Development Fixture',
				location: 'Ho Chi Minh City',
				role: 'Software Engineer',
				required_skills: ['Go'],
				preferred_skills: [],
				seniority: 'MID',
				domains: ['software'],
				employment_type: 'FULL_TIME',
				work_mode: 'HYBRID',
				status: 'DISABLED',
				original_url: 'https://example.invalid/job/1',
				content_hash: 'hash',
				last_seen_at: '2026-08-16T00:00:00Z',
				updated_at: '2026-08-16T00:00:00Z'
			}],
			page: 1,
			page_size: 10,
			total: 1
		}));
		const result = await getAdminJobs();
		expect(result.total).toBe(1);
		expect(result.items[0].isDevelopmentFixture).toBe(true);

		vi.stubGlobal('fetch', async () => jsonResponse({ code: 'internal_error' }, 500));
		await expect(getAdminJobs()).rejects.toMatchObject({ status: 500 });
	});

	it('rejects an unsafe promotion image URL from the admin response', async () => {
		vi.stubGlobal('fetch', async () => jsonResponse({
			slot: 1,
			image_url: 'https://evil.example/image.png',
			alt_text: 'Bad',
			content_hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
		}));
		await expect(uploadAdminPromotion(1, { file: new File(['png'], 'promotion.png', { type: 'image/png' }), altText: 'Bad' }, 'csrf-token')).rejects.toBeInstanceOf(ApiError);
	});

	it('uses the real Job Link API contract with credentials and CSRF', async () => {
		const requests: Request[] = [];
		vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
			const request = new Request(input, init);
			requests.push(request);
			if (request.method === 'GET') {
				return jsonResponse({ items: [], page: 1, page_size: 10, total: 0 });
			}
			if (request.method === 'DELETE' || request.url.endsWith('/status')) return new Response(null, { status: 204 });
			return jsonResponse({
				id: 'source-1',
				url: 'https://jobs.example.com/careers/',
				source_key: 'source-source-1',
				display_name: 'jobs.example.com',
				approval_status: 'ACTIVE',
				approved_at: '2026-08-16T00:00:00Z',
				approved_by: 'admin@example.com',
				created_at: '2026-08-16T00:00:00Z',
				updated_at: '2026-08-16T00:00:00Z'
			});
		});

		const page = await getAdminJobLinks();
		const created = await createAdminJobLink('https://jobs.example.com/careers/', 'csrf-token');
		const updated = await updateAdminJobLink('source-1', 'https://jobs.example.com/jobs/', 'csrf-token');
		await setAdminJobLinkStatus('source-1', 'DISABLED', 'csrf-token');
		await deleteAdminJobLink('source-1', 'csrf-token');

		expect(page.total).toBe(0);
		expect(created.displayName).toBe('jobs.example.com');
		expect(updated.approvalStatus).toBe('ACTIVE');
		expect(requests.map((request) => request.method)).toEqual(['GET', 'POST', 'PATCH', 'PATCH', 'DELETE']);
		expect(requests[1].credentials).toBe('include');
		expect(requests[1].headers.get('x-csrf-token')).toBe('csrf-token');
		expect(requests[3].url).toContain('/job-links/source-1/status');
		expect(requests[3].headers.get('x-csrf-token')).toBe('csrf-token');
	});

	it('uses the database-backed settings and Crawl Now contracts', async () => {
		const requests: Request[] = [];
		vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
			const request = new Request(input, init);
			requests.push(request);
			if (request.url.endsWith('/settings') && request.method === 'GET') {
				return jsonResponse({ crawler: { interval_hours: 6, interval_minutes: 0, interval_seconds: 21600, min_interval_minutes: 15, max_interval_minutes: 10080 }, runtime: { status: 'IDLE', last_heartbeat_at: '2026-08-17T00:00:00Z', next_cycle_at: '2026-08-17T06:00:00Z' } });
			}
			if (request.url.endsWith('/settings/crawler')) {
				return jsonResponse({ crawler: { interval_hours: 2, interval_minutes: 30, interval_seconds: 9000, min_interval_minutes: 15, max_interval_minutes: 10080 }, runtime: { status: 'IDLE', last_heartbeat_at: '2026-08-17T00:00:00Z', next_cycle_at: '2026-08-17T02:30:00Z' } });
			}
			return jsonResponse({ id: 'crawl-1', source_id: 'source-1', status: 'PENDING', requested_by: 'admin@example.com', requested_at: '2026-08-17T00:00:00Z' }, 202);
		});

		const current = await getAdminSettings();
		const updated = await updateAdminCrawlerSettings(2, 30, 'csrf-token');
		const request = await requestAdminJobLinkCrawl('source-1', 'csrf-token');

		expect(current.crawler.intervalHours).toBe(6);
		expect(updated.crawler.intervalSeconds).toBe(9000);
		expect(request.status).toBe('PENDING');
		expect(requests.map((item) => item.method)).toEqual(['GET', 'PATCH', 'POST']);
		expect(requests[1].headers.get('x-csrf-token')).toBe('csrf-token');
		expect(requests[2].url).toContain('/job-links/source-1/crawl');
	});

	it('reads real client users and structured CV profiles and deletes CV with CSRF', async () => {
		const requests: Request[] = [];
		vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
			const request = new Request(input, init);
			requests.push(request);
			if (request.url.includes('/users?')) return jsonResponse({ items: [{ id: 'user-1', email: 'user@example.com', display_name: 'Example User', provider: 'google', created_at: '2026-08-20T00:00:00Z', last_login_at: '2026-08-20T00:01:00Z' }], page: 1, page_size: 10, total: 1 });
			if (request.method === 'DELETE') return new Response(null, { status: 204 });
			return jsonResponse({ items: [{ scan_id: 'scan-1', user_id: 'user-1', email: 'user@example.com', display_name: 'Example User', status: 'completed', location: 'Hồ Chí Minh', created_at: '2026-08-20T00:00:00Z', updated_at: '2026-08-20T00:01:00Z', match_count: 2, profile: { roles: ['Backend Developer'], skills: ['Go'], years_of_experience: 3, seniority: 'mid', domains: ['SaaS'], education: [], certifications: [] } }], page: 1, page_size: 10, total: 1 });
		});

		const users = await getAdminClientUsers();
		const cvs = await getAdminCVProfiles(1, 10, 'user@example.com', 'Backend');
		await deleteAdminCVProfile('scan-1', 'csrf-token');

		expect(users.items[0].email).toBe('user@example.com');
		expect(cvs.items[0].profile?.roles).toEqual(['Backend Developer']);
		expect(requests.at(-1)?.headers.get('x-csrf-token')).toBe('csrf-token');
	});

	it('uses the real Home section API for text/image and media-strip changes', async () => {
		const requests: Request[] = [];
		const image = 'https://res.cloudinary.com/example/image/upload/home.png';
		const hash = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
		const section = { slot: 1, layout: 'CONTENT_LEFT', is_active: true, title: 'Title', body: 'Body', image_alt_text: 'Alt', image_url: image, image_content_hash: hash, media: [] };
		const media = { id: 'media-1', sort_order: 0, is_active: true, image_alt_text: 'Strip alt', image_url: image, image_content_hash: hash };
		vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
			const request = new Request(input, init);
			requests.push(request);
			if (request.method === 'GET') return jsonResponse({ sections: [section] });
			if (request.method === 'POST') return jsonResponse(media);
			if (request.method === 'PATCH') return jsonResponse({ ...media, is_active: false });
			if (request.method === 'DELETE') return new Response(null, { status: 204 });
			return jsonResponse(section);
		});

		const sections = await getAdminHomeSections();
		const updatedSection = await updateAdminHomeSection(1, { isActive: true, title: 'Title', body: 'Body', file: new File(['png'], 'home.png', { type: 'image/png' }) }, 'csrf-token');
		const createdMedia = await createAdminHomeMedia(4, { sortOrder: 0, isActive: true, file: new File(['png'], 'strip.png', { type: 'image/png' }) }, 'csrf-token');
		const updatedMedia = await updateAdminHomeMedia('media-1', { isActive: false }, 'csrf-token');
		await deleteAdminHomeMedia('media-1', 'csrf-token');

		expect(sections[0].slot).toBe(1);
		expect(updatedSection.title).toBe('Title');
		expect(createdMedia.id).toBe('media-1');
		expect(updatedMedia.isActive).toBe(false);
		expect(requests.map((request) => request.method)).toEqual(['GET', 'PUT', 'POST', 'PATCH', 'DELETE']);
		expect(requests[1].headers.get('x-csrf-token')).toBe('csrf-token');
		const sectionForm = await requests[1].formData();
		expect(sectionForm.get('image')).toBeInstanceOf(File);
		expect(sectionForm.get('title')).toBe('Title');
		expect(sectionForm.get('body')).toBe('Body');
		expect(sectionForm.has('eyebrow')).toBe(false);
		expect(sectionForm.has('image_alt_text')).toBe(false);
		expect(sectionForm.has('target_url')).toBe(false);
		const mediaForm = await requests[2].formData();
		expect(mediaForm.get('image')).toBeInstanceOf(File);
		expect(mediaForm.has('image_alt_text')).toBe(false);
		expect(mediaForm.has('target_url')).toBe(false);
		expect(requests[4].url).toContain('/home-sections/4/items/media-1');
	});

	it('keeps the Home editor visible when the database has no section rows', async () => {
		vi.stubGlobal('fetch', async () => jsonResponse({ sections: [] }));

		const sections = await getAdminHomeSections();

		expect(sections.map((section) => section.slot)).toEqual([1, 2, 3, 4]);
		expect(sections.every((section) => section.isActive === false)).toBe(true);
		expect(sections.every((section) => section.media.length === 0)).toBe(true);
	});
});
