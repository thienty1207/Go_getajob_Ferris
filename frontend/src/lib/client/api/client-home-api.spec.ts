import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '$lib/shared/api/api-errors';
import { getClientHomeSections } from './client-home-api';

function jsonResponse(body: unknown, status = 200) {
	return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

const image = 'https://res.cloudinary.com/example/image/upload/home-section.png';
const hash = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';

afterEach(() => vi.unstubAllGlobals());

describe('client Home sections API', () => {
	it('requests the public contract and returns only active sections', async () => {
		let request: Request | undefined;
		vi.stubGlobal('fetch', async (input: RequestInfo | URL, init?: RequestInit) => {
			request = new Request(input, init);
			return jsonResponse({
				sections: [
					{ slot: 1, layout: 'CONTENT_LEFT', is_active: true, title: 'Section thật', body: 'Nội dung thật', image_alt_text: 'Ảnh thật', image_url: image, image_content_hash: hash, media: [] },
					{ slot: 2, layout: 'IMAGE_LEFT', is_active: false, media: [] }
				]
			});
		});

		const sections = await getClientHomeSections();

		expect(request?.url).toContain('/api/v1/client/home-sections');
		expect(request?.method).toBe('GET');
		expect(sections).toHaveLength(1);
		expect(sections[0].title).toBe('Section thật');
	});

	it('rejects a public response that exposes a non-HTTPS image URL', async () => {
		vi.stubGlobal('fetch', async () => jsonResponse({
			sections: [{ slot: 1, layout: 'CONTENT_LEFT', is_active: true, title: 'Không an toàn', body: 'Nội dung', image_alt_text: 'Ảnh', image_url: 'http://example.com/image.png', image_content_hash: hash, media: [] }]
		}));

		await expect(getClientHomeSections()).rejects.toBeInstanceOf(ApiError);
	});
});
