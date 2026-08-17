import { ApiError } from './api-errors';

/**
 * Resolves an API path while keeping local browser and API hosts aligned.
 *
 * The admin session cookie is intentionally host-only. During local
 * development, `localhost` and `127.0.0.1` are different cookie hosts even
 * though they point at the same machine, so a configured API base must follow
 * the hostname the browser is currently using.
 */
export function resolveApiUrl(
	path: string,
	configuredBase: string | undefined,
	production: boolean,
	invalidMessage: string,
	currentHostname = browserHostname()
): string {
	const base = (configuredBase ?? '').trim().replace(/\/$/, '');
	if (!base) return path;

	let parsed: URL;
	try {
		parsed = new URL(base);
		const validProtocol = production ? parsed.protocol === 'https:' : parsed.protocol === 'http:' || parsed.protocol === 'https:';
		if (!validProtocol) throw new Error('invalid API protocol');
	} catch {
		throw new ApiError(invalidMessage, 0, 'invalid_api_config');
	}

	if (isLocalHostAlias(parsed.hostname, currentHostname)) {
		parsed.hostname = currentHostname;
	}

	return `${parsed.origin}${path}`;
}

function browserHostname(): string | undefined {
	return typeof window === 'undefined' ? undefined : window.location.hostname;
}

function isLocalHostAlias(apiHostname: string, browserHostname: string | undefined): browserHostname is string {
	if (!browserHostname) return false;
	const localHosts = new Set(['localhost', '127.0.0.1']);
	return localHosts.has(apiHostname) && localHosts.has(browserHostname) && apiHostname !== browserHostname;
}
