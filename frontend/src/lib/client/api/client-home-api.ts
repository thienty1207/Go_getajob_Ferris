import { requestJson, type Fetcher } from '../../shared/api/http-client';
import { resolveApiUrl } from '../../shared/api/api-url';
import { parseHomeSectionsPayload, type HomeSection } from '../../shared/types/home-section';

const HOME_SECTIONS_PATH = '/api/v1/client/home-sections';

export async function getClientHomeSections(fetcher: Fetcher = fetch): Promise<HomeSection[]> {
	const payload = await requestJson<unknown>(apiUrl(HOME_SECTIONS_PATH), {}, fetcher);
	return parseHomeSectionsPayload(payload).filter((section) => section.isActive);
}

function apiUrl(path: string): string {
	return resolveApiUrl(path, import.meta.env.PUBLIC_API_BASE_URL, import.meta.env.PROD, 'Cấu hình matching service không hợp lệ.');
}
