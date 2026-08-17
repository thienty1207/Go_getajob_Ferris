import { describe, expect, it } from 'vitest';
import { requestJson } from './http-client';

describe('requestJson', () => {
	it('maps a network failure to a recoverable ApiError', async () => {
		await expect(
			requestJson('/api/test', {}, async () => {
				throw new Error('offline');
			})
		).rejects.toMatchObject({ code: 'network_error' });
	});

	it('maps a slow request abort to a timeout ApiError', async () => {
		const fetcher: typeof fetch = async (_input, init) =>
			new Promise((_resolve, reject) => {
				init?.signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')));
			});

		await expect(requestJson('/api/test', {}, fetcher, 1)).rejects.toMatchObject({
			status: 408,
			code: 'request_timeout'
		});
	});

	it('rejects a successful response that is not JSON', async () => {
		await expect(requestJson('/api/test', {}, async () => new Response('not-json'))).rejects.toMatchObject({
			code: 'invalid_json'
		});
	});

	it('rejects an oversized response before it reaches the UI', async () => {
		const oversizedBody = 'x'.repeat(1_000_001);
		await expect(requestJson('/api/test', {}, async () => new Response(oversizedBody))).rejects.toMatchObject({
			code: 'response_too_large'
		});
	});
});
