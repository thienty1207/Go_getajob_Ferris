import { requestJson, type Fetcher } from '../../shared/api/http-client';
import { ApiError } from '../../shared/api/api-errors';
import { resolveApiUrl } from '../../shared/api/api-url';
import type { ClientUser } from '../../shared/types/client-auth';

const AUTH_PATH = '/api/v1/client/auth';
const GOOGLE_PATH = `${AUTH_PATH}/google`;

interface CredentialsInit extends RequestInit {
	credentials: 'include';
}

export interface ClientMeResponse {
	user: ClientUser;
	csrfToken: string;
}

function apiUrl(path: string): string {
	return resolveApiUrl(path, import.meta.env.PUBLIC_API_BASE_URL, import.meta.env.PROD, 'Cấu hình matching service không hợp lệ.');
}

function withCredentials(init: RequestInit = {}): CredentialsInit {
	return { ...init, credentials: 'include' };
}

export function googleLoginUrl(): string {
	return apiUrl(GOOGLE_PATH);
}

// loadCurrentUser returns the real session or null when unauthenticated. The
// snake_case backend payload is parsed into the typed client shape; a missing
// session is never fabricated by the frontend.
export async function loadCurrentUser(fetcher: Fetcher = fetch): Promise<ClientMeResponse | null> {
	try {
		const payload = await requestJson<unknown>(apiUrl(`${AUTH_PATH}/me`), withCredentials(), fetcher);
		return parseMe(payload);
	} catch (error) {
		if (error instanceof ApiError && error.status === 401) {
			return null;
		}
		throw error;
	}
}

export async function logoutClient(fetcher: Fetcher = fetch, csrfToken: string): Promise<void> {
	await requestJson<void>(apiUrl(`${AUTH_PATH}/logout`), withCredentials({ method: 'POST', headers: { 'X-CSRF-Token': csrfToken } }), fetcher);
}

function parseMe(value: unknown): ClientMeResponse {
	const record = asRecord(value);
	const user = asRecord(record.user);
	const csrfToken = typeof record.csrf_token === 'string' ? record.csrf_token : '';
	return {
		user: {
			id: stringField(user.id),
			email: stringField(user.email),
			displayName: stringField(user.display_name),
			...(typeof user.avatar_url === 'string' && user.avatar_url ? { avatarUrl: user.avatar_url } : {}),
			provider: 'google',
			createdAt: stringField(user.created_at),
			lastLoginAt: stringField(user.last_login_at)
		},
		csrfToken
	};
}

function asRecord(value: unknown): Record<string, unknown> {
	if (typeof value !== 'object' || value === null) {
		throw new ApiError('Matching service trả về dữ liệu không hợp lệ.', 502, 'invalid_response');
	}
	return value as Record<string, unknown>;
}

function stringField(value: unknown): string {
	if (typeof value !== 'string' || !value.trim()) {
		throw new ApiError('Matching service trả về dữ liệu thiếu trường bắt buộc.', 502, 'invalid_response');
	}
	return value;
}
