import { ApiError } from './api-errors';

export type Fetcher = typeof fetch;
const REQUEST_TIMEOUT_MS = 30_000;
const MAX_RESPONSE_BYTES = 1_000_000;

export async function requestJson<T>(
	url: string,
	init: RequestInit,
	fetcher: Fetcher = fetch,
	timeoutMs = REQUEST_TIMEOUT_MS
): Promise<T> {
	let response: Response;
	const controller = new AbortController();
	const timeoutId = init.signal ? undefined : setTimeout(() => controller.abort(), timeoutMs);

	try {
		response = await fetcher(url, {
			...init,
			...(init.signal ? {} : { signal: controller.signal })
		});
	} catch (error) {
		if (isAbortError(error)) {
			throw new ApiError('Matching service phản hồi quá lâu. Vui lòng thử lại.', 408, 'request_timeout');
		}
		throw new ApiError('Không thể kết nối đến matching service.', 0, 'network_error');
	} finally {
		if (timeoutId !== undefined) clearTimeout(timeoutId);
	}

	const contentLength = Number(response.headers.get('content-length'));
	if (Number.isFinite(contentLength) && contentLength > MAX_RESPONSE_BYTES) {
		throw new ApiError('Matching service trả về dữ liệu quá lớn.', 502, 'response_too_large');
	}

	const rawBody = await response.text();
	if (new TextEncoder().encode(rawBody).byteLength > MAX_RESPONSE_BYTES) {
		throw new ApiError('Matching service trả về dữ liệu quá lớn.', 502, 'response_too_large');
	}
	let body: unknown = undefined;

	if (rawBody) {
		try {
			body = JSON.parse(rawBody);
		} catch {
			throw new ApiError('Matching service trả về dữ liệu không hợp lệ.', response.status, 'invalid_json');
		}
	}

	if (!response.ok) {
		const code = isRecord(body) && typeof body.code === 'string' ? body.code : undefined;
		throw new ApiError(clientErrorMessage(response.status), response.status, code);
	}

	return body as T;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

function isAbortError(value: unknown): boolean {
	return typeof value === 'object' && value !== null && 'name' in value && value.name === 'AbortError';
}

function clientErrorMessage(status: number): string {
	if (status === 408) return 'Matching service phản hồi quá lâu. Vui lòng thử lại.';
	if (status === 429) return 'Matching service đang nhận nhiều yêu cầu. Vui lòng thử lại sau ít phút.';
	return 'Matching service đang tạm thời không khả dụng.';
}
