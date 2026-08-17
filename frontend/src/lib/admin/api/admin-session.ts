import { ApiError } from '$lib/shared/api/api-errors';
import type { AdminAuthResponse } from './admin-api';

export const ADMIN_AUTH_CONTEXT = 'ferris.admin.auth';

export type AdminAuthStatus = 'checking' | 'authenticated' | 'anonymous' | 'error';

export interface AdminAuthState {
	status: AdminAuthStatus;
	user: AdminAuthResponse['admin'] | null;
	csrfToken: string;
	errorMessage: string;
}

export function isUnauthorized(error: unknown): boolean {
	return error instanceof ApiError && error.status === 401;
}
