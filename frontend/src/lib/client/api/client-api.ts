import { ApiError } from '../../shared/api/api-errors';
import { resolveApiUrl } from '../../shared/api/api-url';
import { requestJson, type Fetcher } from '../../shared/api/http-client';
import type {
	ClientScanInput,
	ClientLocation,
	ScanAccepted,
	ScanCompleted,
	ScanFailed,
	ScanProcessing,
	ScanStatusResponse,
	PromotionSlide
} from '../../shared/types/client';
import type { JobMatch, WorkMode } from '../../shared/types/job';

const SCANS_PATH = '/api/v1/client/scans';
const LOCATIONS_PATH = '/api/v1/client/locations';
const MAX_MATCHES = 100;
const MAX_SCAN_ID_LENGTH = 128;
const MAX_TITLE_LENGTH = 240;
const MAX_COMPANY_LENGTH = 180;
const MAX_LOCATION_LENGTH = 180;
const MAX_EMPLOYMENT_TYPE_LENGTH = 80;
const MAX_SKILL_TAG_LENGTH = 64;
const MAX_SALARY_LENGTH = 160;
const MAX_URL_LENGTH = 2048;
const PROMOTIONS_PATH = '/api/v1/client/promotions';
const MAX_PROMOTIONS = 3;
const MAX_PROMOTION_ALT_LENGTH = 180;
const MAX_PROMOTION_COPY_LENGTH = 320;
const MAX_CONTENT_HASH_LENGTH = 128;

export async function startScan(input: ClientScanInput, csrfToken: string, fetcher: Fetcher = fetch): Promise<ScanAccepted> {
	if (!csrfToken.trim()) {
		throw new ApiError('Phiên đăng nhập không hợp lệ cho thao tác quét CV.', 403, 'client_csrf_invalid');
	}
	const form = new FormData();
	form.append('cv', input.file);
	form.append('location_id', input.locationId);

	const payload = await requestJson<unknown>(apiUrl(SCANS_PATH), {
		method: 'POST',
		body: form,
		credentials: 'include',
		headers: { 'X-CSRF-Token': csrfToken }
	}, fetcher);
	return parseAccepted(payload);
}

export async function getClientLocations(fetcher: Fetcher = fetch): Promise<ClientLocation[]> {
	const payload = asRecord(await requestJson<unknown>(apiUrl(LOCATIONS_PATH), {}, fetcher));
	if (!Array.isArray(payload.items)) {
		throw new ApiError('Location service trả về dữ liệu không hợp lệ.', 502, 'invalid_location_response');
	}
	return payload.items.map(parseClientLocation);
}

export async function getScanStatus(scanId: string, fetcher: Fetcher = fetch): Promise<ScanStatusResponse> {
	const payload = await requestJson<unknown>(apiUrl(`${SCANS_PATH}/${encodeURIComponent(scanId)}`), { credentials: 'include' }, fetcher);
	return parseStatus(payload);
}

export async function getPromotions(fetcher: Fetcher = fetch): Promise<PromotionSlide[]> {
	const payload = await requestJson<unknown>(apiUrl(PROMOTIONS_PATH), {}, fetcher);
	const record = asRecord(payload);
	if (!Array.isArray(record.promotions) || record.promotions.length > MAX_PROMOTIONS) {
		throw new ApiError('Promotion service trả về quá nhiều slide.', 502, 'invalid_promotion_response');
	}

	const seenSlots = new Set<number>();
	const promotions = record.promotions.map(parsePromotion).sort((left, right) => left.slot - right.slot);
	for (const promotion of promotions) {
		if (seenSlots.has(promotion.slot)) {
			throw new ApiError('Promotion service trả về slot bị trùng.', 502, 'invalid_promotion_response');
		}
		seenSlots.add(promotion.slot);
	}
	return promotions;
}

function apiUrl(path: string): string {
	return resolveApiUrl(path, import.meta.env.PUBLIC_API_BASE_URL, import.meta.env.PROD, 'Cấu hình matching service không hợp lệ.');
}

function parseAccepted(value: unknown): ScanAccepted {
	const record = asRecord(value);
	if (record.status !== 'processing') {
		throw new ApiError('Matching service trả về trạng thái scan không hợp lệ.', 502, 'invalid_scan_response');
	}

	return { scanId: stringField(record.scan_id, MAX_SCAN_ID_LENGTH), status: 'processing' };
}

function parseClientLocation(value: unknown): ClientLocation {
	const record = asRecord(value);
	const latitude = optionalNumber(record.latitude);
	const longitude = optionalNumber(record.longitude);
	return {
		id: stringField(record.id, 100),
		displayName: stringField(record.display_name, 200),
		province: stringField(record.province, 160),
		country: stringField(record.country, 120),
		...(latitude === undefined ? {} : { latitude }),
		...(longitude === undefined ? {} : { longitude })
	};
}

function parseStatus(value: unknown): ScanStatusResponse {
	const record = asRecord(value);
	if (typeof record.status !== 'string') {
		throw new ApiError('Matching service trả về trạng thái scan không hợp lệ.', 502, 'invalid_scan_response');
	}
	const scanId = stringField(record.scan_id, MAX_SCAN_ID_LENGTH);

	if (record.status === 'processing') {
		return { scanId, status: 'processing' } satisfies ScanProcessing;
	}

	if (record.status === 'failed') {
		return {
			scanId,
			status: 'failed',
			message: 'Matching service không thể hoàn tất việc quét CV. Vui lòng thử lại.'
		} satisfies ScanFailed;
	}

	if (record.status === 'completed' && Array.isArray(record.matches)) {
		if (record.matches.length > MAX_MATCHES) {
			throw new ApiError('Matching service trả về quá nhiều kết quả.', 502, 'invalid_job_response');
		}
		return {
			scanId,
			status: 'completed',
			matches: record.matches.map(parseJobMatch)
		} satisfies ScanCompleted;
	}

	throw new ApiError('Matching service trả về trạng thái scan không hợp lệ.', 502, 'invalid_scan_response');
}

function parseJobMatch(value: unknown): JobMatch {
	const record = asRecord(value);
	const matchPercent = numberField(record.match_percent);
	const title = stringField(record.title, MAX_TITLE_LENGTH);
	const company = stringField(record.company, MAX_COMPANY_LENGTH);
	const location = stringField(record.location, MAX_LOCATION_LENGTH);
	const employmentType = stringField(record.employment_type, MAX_EMPLOYMENT_TYPE_LENGTH);
	const workMode = stringField(record.work_mode).toLowerCase();
	const originalUrl = safeHttpUrl(record.original_url);
	const skillTags = Array.isArray(record.skill_tags)
		? record.skill_tags.slice(0, 3).map((tag) => stringField(tag, MAX_SKILL_TAG_LENGTH))
		: [];

	if (!['remote', 'hybrid', 'onsite'].includes(workMode)) {
		throw new ApiError('Job match có work mode không hợp lệ.', 502, 'invalid_job_response');
	}

	const distanceKm =
		workMode === 'remote' || record.distance_km === undefined || record.distance_km === null
			? undefined
			: nonNegativeNumber(record.distance_km, 'Khoảng cách job không hợp lệ.');
	const salary = record.salary === undefined || record.salary === null ? undefined : parseSalary(record.salary);

	if (matchPercent < 0 || matchPercent > 100) {
		throw new ApiError('CV Match % phải nằm trong khoảng từ 0 đến 100.', 502, 'invalid_job_response');
	}

	return {
		id: stringField(record.id, MAX_SCAN_ID_LENGTH),
		matchPercent,
		title,
		company,
		location,
		distanceKm,
		employmentType,
		workMode: workMode as WorkMode,
		salary,
		skillTags,
		originalUrl
	};
}

function parseSalary(value: unknown) {
	const record = asRecord(value);
	return {
		display: stringField(record.display, MAX_SALARY_LENGTH),
		...(typeof record.currency === 'string' ? { currency: stringField(record.currency, 16) } : {})
	};
}

function parsePromotion(value: unknown): PromotionSlide {
	const record = asRecord(value);
	const slot = record.slot;
	if (typeof slot !== 'number' || !Number.isInteger(slot) || slot < 1 || slot > MAX_PROMOTIONS) {
		throw new ApiError('Promotion service trả về vị trí slide không hợp lệ.', 502, 'invalid_promotion_response');
	}
	const imageUrl = safePromotionImageUrl(record.image_url);
	const contentHash = stringField(record.content_hash, MAX_CONTENT_HASH_LENGTH);
	if (!/^[a-f0-9]{64}$/.test(contentHash)) {
		throw new ApiError('Promotion service trả về hash ảnh không hợp lệ.', 502, 'invalid_promotion_response');
	}

	const eyebrow = optionalStringField(record.eyebrow, MAX_PROMOTION_COPY_LENGTH);
	const title = optionalStringField(record.title, MAX_PROMOTION_COPY_LENGTH);
	const body = optionalStringField(record.body, MAX_PROMOTION_COPY_LENGTH);
	return {
		slot,
		imageUrl,
		altText: stringField(record.alt_text, MAX_PROMOTION_ALT_LENGTH),
		...(eyebrow ? { eyebrow } : {}),
		...(title ? { title } : {}),
		...(body ? { body } : {}),
		...(record.target_url === undefined || record.target_url === null ? {} : { targetUrl: safeHttpUrl(record.target_url) }),
		contentHash
	};
}

function optionalStringField(value: unknown, maxLength: number): string | undefined {
	if (value === undefined || value === null || value === '') return undefined;
	return stringField(value, maxLength);
}

function safePromotionImageUrl(value: unknown): string {
	const rawUrl = stringField(value, MAX_URL_LENGTH);
	try {
		if (rawUrl.startsWith('/') && !rawUrl.startsWith('//')) {
			const relative = new URL(rawUrl, 'http://promotion.local');
			if (!relative.pathname.startsWith(`${PROMOTIONS_PATH}/`)) throw new Error('unexpected promotion path');
			return apiUrl(rawUrl);
		}
		throw new Error('promotion image URL must be a same-origin relative path');
	} catch {
		throw new ApiError('Promotion service trả về URL ảnh không hợp lệ.', 502, 'invalid_promotion_response');
	}
}

function asRecord(value: unknown): Record<string, unknown> {
	if (typeof value !== 'object' || value === null) {
		throw new ApiError('Matching service trả về dữ liệu không hợp lệ.', 502, 'invalid_response');
	}
	return value as Record<string, unknown>;
}

function stringField(value: unknown, maxLength = MAX_SALARY_LENGTH): string {
	if (typeof value !== 'string' || !value.trim() || value.length > maxLength) {
		throw new ApiError('Matching service trả về dữ liệu thiếu trường bắt buộc.', 502, 'invalid_response');
	}
	return value;
}

function numberField(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) {
		throw new ApiError('Matching service trả về dữ liệu số không hợp lệ.', 502, 'invalid_response');
	}
	return value;
}

function optionalNumber(value: unknown): number | undefined {
	if (value === undefined || value === null) return undefined;
	return numberField(value);
}

function nonNegativeNumber(value: unknown, message: string): number {
	const number = numberField(value);
	if (number < 0) {
		throw new ApiError(message, 502, 'invalid_job_response');
	}
	return number;
}

function safeHttpUrl(value: unknown): string {
	const rawUrl = stringField(value, MAX_URL_LENGTH);

	try {
		const url = new URL(rawUrl);
		if ((url.protocol !== 'http:' && url.protocol !== 'https:') || url.username || url.password) {
			throw new Error('unsupported protocol');
		}
		return url.toString();
	} catch {
		throw new ApiError('Original job URL không hợp lệ.', 502, 'invalid_job_response');
	}
}
