import { ApiError } from '$lib/shared/api/api-errors';
import { resolveApiUrl } from '$lib/shared/api/api-url';
import type { PromotionSlide } from '$lib/shared/types/client';

const ADMIN_PATH = '/api/v1/admin';
const MAX_RESPONSE_BYTES = 1_000_000;
const REQUEST_TIMEOUT_MS = 30_000;

export interface AdminUser {
	id: string;
	email: string;
	isActive: boolean;
	lastLoginAt?: string;
}

export interface AdminAuthResponse {
	admin: AdminUser;
	csrfToken: string;
}

export interface PromotionUploadInput {
	file: File;
	altText?: string;
}

export interface AdminJob {
	id: string;
	sourceKey: string;
	sourceName: string;
	sourceApprovalStatus: string;
	isDevelopmentFixture: boolean;
	title: string;
	company: string;
	location: string;
	locationId?: string;
	role: string;
	requiredSkills: string[];
	preferredSkills: string[];
	seniority: string;
	minimumExperienceYears?: number;
	domains: string[];
	employmentType: string;
	workMode: 'remote' | 'hybrid' | 'onsite' | string;
	status: 'active' | 'verifying' | 'closed' | 'expired' | 'disabled' | string;
	originalUrl: string;
	contentHash: string;
	lastSeenAt: string;
	updatedAt: string;
}

export interface AdminJobPage {
	items: AdminJob[];
	page: number;
	pageSize: number;
	total: number;
}

export interface AdminJobFilter {
	search?: string;
	locationId?: string;
	unresolvedLocation?: boolean;
}

export interface AdminJobLink {
	id: string;
	url: string;
	sourceKey: string;
	displayName: string;
	approvalStatus: string;
	approvedAt?: string;
	approvedBy?: string;
	createdAt: string;
	updatedAt: string;
	lastCrawlStatus?: string;
	lastCrawlAt?: string;
	activeCrawlRequestId?: string;
	activeCrawlRequestStatus?: string;
	lastCrawlPages?: number;
	lastCrawlJobs?: number;
	lastCrawlCreated?: number;
	lastCrawlUpdated?: number;
	lastCrawlMissing?: number;
	lastCrawlErrorCode?: string;
}

export interface AdminJobLinkPage {
	items: AdminJobLink[];
	page: number;
	pageSize: number;
	total: number;
}

export interface AdminLocation {
	id: string;
	displayName: string;
	province: string;
	country: string;
	canonicalKey: string;
	latitude?: number;
	longitude?: number;
	isActive: boolean;
	jobCount: number;
	createdAt: string;
	updatedAt: string;
}

export interface AdminLocationPage {
	items: AdminLocation[];
	page: number;
	pageSize: number;
	total: number;
}

export interface AdminLocationOption {
	id: string;
	displayName: string;
	isActive: boolean;
}

export interface AdminCrawlerSettings {
	intervalHours: number;
	intervalMinutes: number;
	intervalSeconds: number;
	minIntervalMinutes: number;
	maxIntervalMinutes: number;
}

export type AdminCrawlerRuntimeStatus = 'OFFLINE' | 'IDLE' | 'RUNNING' | 'ERROR' | string;

export interface AdminCrawlerRuntime {
	status: AdminCrawlerRuntimeStatus;
	lastHeartbeatAt?: string;
	lastCycleStartedAt?: string;
	lastCycleFinishedAt?: string;
	nextCycleAt?: string;
	currentSourceKey?: string;
	lastErrorCode?: string;
}

export interface AdminSettings {
	crawler: AdminCrawlerSettings;
	runtime: AdminCrawlerRuntime;
}

export interface AdminCrawlRequest {
	id: string;
	sourceId: string;
	status: string;
	requestedBy: string;
	requestedAt: string;
	startedAt?: string;
	finishedAt?: string;
	sourceRunId?: string;
	errorCode?: string;
}

export async function adminLogin(email: string, password: string): Promise<AdminAuthResponse> {
	const response = await adminRequest<unknown>('/auth/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ email, password })
	});
	return parseAuthResponse(response);
}

export async function getAdminMe(): Promise<AdminAuthResponse> {
	return parseAuthResponse(await adminRequest<unknown>('/auth/me', { method: 'GET' }));
}

export async function adminLogout(csrfToken: string): Promise<void> {
	await adminRequest<void>('/auth/logout', {
		method: 'POST',
		headers: { 'X-CSRF-Token': csrfToken }
	});
}

export async function getAdminPromotions(): Promise<PromotionSlide[]> {
	const payload = asRecord(await adminRequest<unknown>('/promotions', { method: 'GET' }));
	if (!Array.isArray(payload.promotions) || payload.promotions.length > 3) {
		throw new ApiError('Promotion service trả về dữ liệu không hợp lệ.', 502, 'invalid_promotion_response');
	}
	return payload.promotions.map(parsePromotion).sort((left, right) => left.slot - right.slot);
}

export async function uploadAdminPromotion(slot: number, input: PromotionUploadInput, csrfToken: string): Promise<PromotionSlide> {
	if (!Number.isInteger(slot) || slot < 1 || slot > 3) {
		throw new ApiError('Vị trí promotion không hợp lệ.', 400, 'invalid_promotion');
	}
	const formData = new FormData();
	formData.set('image', input.file);
	if (input.altText?.trim()) formData.set('alt_text', input.altText.trim());
	return parsePromotion(
		await adminRequest<unknown>(`/promotions/${slot}`, {
			method: 'PUT',
			headers: { 'X-CSRF-Token': csrfToken },
			body: formData
		})
	);
}

export async function deleteAdminPromotion(slot: number, csrfToken: string): Promise<void> {
	await adminRequest<void>(`/promotions/${slot}`, {
		method: 'DELETE',
		headers: { 'X-CSRF-Token': csrfToken }
	});
}

export async function getAdminJobs(page = 1, pageSize = 10, filter: AdminJobFilter = {}): Promise<AdminJobPage> {
	const query = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
	if (filter.search?.trim()) query.set('q', filter.search.trim());
	if (filter.locationId) query.set('location_id', filter.locationId);
	if (filter.unresolvedLocation) query.set('unresolved', 'true');
	const payload = asRecord(await adminRequest<unknown>(`/jobs?${query.toString()}`, { method: 'GET' }));
	if (!Array.isArray(payload.items)) {
		throw new ApiError('Job service trả về danh sách không hợp lệ.', 502, 'invalid_job_response');
	}
	const parsedPage = integerField(payload.page, 1, 100_000);
	const parsedPageSize = integerField(payload.page_size, 1, 10);
	const total = integerField(payload.total, 0, Number.MAX_SAFE_INTEGER);
	return {
		items: payload.items.map(parseJob),
		page: parsedPage,
		pageSize: parsedPageSize,
		total
	};
}

export async function getAdminJobLinks(page = 1, pageSize = 10): Promise<AdminJobLinkPage> {
	const payload = asRecord(await adminRequest<unknown>(`/job-links?page=${page}&page_size=${pageSize}`, { method: 'GET' }));
	if (!Array.isArray(payload.items)) {
		throw new ApiError('Job Link service trả về danh sách không hợp lệ.', 502, 'invalid_job_link_response');
	}
	return {
		items: payload.items.map(parseJobLink),
		page: integerField(payload.page, 1, 100_000),
		pageSize: integerField(payload.page_size, 1, 10),
		total: integerField(payload.total, 0, Number.MAX_SAFE_INTEGER)
	};
}

export async function getAdminLocationPage(page = 1, pageSize = 10): Promise<AdminLocationPage> {
	const payload = asRecord(await adminRequest<unknown>(`/locations?page=${page}&page_size=${pageSize}`, { method: 'GET' }));
	if (!Array.isArray(payload.items)) throw new ApiError('Location service trả về danh sách không hợp lệ.', 502, 'invalid_location_response');
	return {
		items: payload.items.map(parseLocation),
		page: integerField(payload.page, 1, 100_000),
		pageSize: integerField(payload.page_size, 1, 10),
		total: integerField(payload.total, 0, Number.MAX_SAFE_INTEGER)
	};
}

export async function getAdminLocationOptions(): Promise<AdminLocationOption[]> {
	const payload = asRecord(await adminRequest<unknown>('/locations/options', { method: 'GET' }));
	if (!Array.isArray(payload.items)) throw new ApiError('Location options trả về dữ liệu không hợp lệ.', 502, 'invalid_location_response');
	return payload.items.map(parseLocationOption);
}

export async function createAdminLocation(input: Pick<AdminLocation, 'displayName' | 'province' | 'country' | 'isActive'>, csrfToken: string): Promise<AdminLocation> {
	return parseLocation(await adminRequest<unknown>('/locations', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
		body: JSON.stringify({ display_name: input.displayName, province: input.province, country: input.country, is_active: input.isActive })
	}));
}

export async function updateAdminLocation(id: string, input: Pick<AdminLocation, 'displayName' | 'province' | 'country' | 'isActive'>, csrfToken: string): Promise<AdminLocation> {
	return parseLocation(await adminRequest<unknown>(`/locations/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
		body: JSON.stringify({ display_name: input.displayName, province: input.province, country: input.country, is_active: input.isActive })
	}));
}

export async function assignAdminJobLocation(jobId: string, locationId: string | null, csrfToken: string): Promise<void> {
	await adminRequest<void>(`/jobs/${encodeURIComponent(jobId)}/location`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
		body: JSON.stringify({ location_id: locationId })
	});
}

export async function createAdminJobLink(url: string, csrfToken: string): Promise<AdminJobLink> {
	return parseJobLink(await adminRequest<unknown>('/job-links', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
		body: JSON.stringify({ url })
	}));
}

export async function updateAdminJobLink(id: string, url: string, csrfToken: string): Promise<AdminJobLink> {
	return parseJobLink(await adminRequest<unknown>(`/job-links/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
		body: JSON.stringify({ url })
	}));
}

export async function deleteAdminJobLink(id: string, csrfToken: string): Promise<void> {
	await adminRequest<void>(`/job-links/${encodeURIComponent(id)}`, {
		method: 'DELETE',
		headers: { 'X-CSRF-Token': csrfToken }
	});
}

export async function setAdminJobLinkStatus(id: string, approvalStatus: 'ACTIVE' | 'DISABLED', csrfToken: string): Promise<void> {
	await adminRequest<void>(`/job-links/${encodeURIComponent(id)}/status`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
		body: JSON.stringify({ approval_status: approvalStatus })
	});
}

export async function requestAdminJobLinkCrawl(id: string, csrfToken: string): Promise<AdminCrawlRequest> {
	return parseCrawlRequest(await adminRequest<unknown>(`/job-links/${encodeURIComponent(id)}/crawl`, {
		method: 'POST',
		headers: { 'X-CSRF-Token': csrfToken }
	}));
}

export async function getAdminSettings(): Promise<AdminSettings> {
	const payload = asRecord(await adminRequest<unknown>('/settings', { method: 'GET' }));
	return { crawler: parseCrawlerSettings(payload.crawler), runtime: parseCrawlerRuntime(payload.runtime) };
}

export async function updateAdminCrawlerSettings(intervalHours: number, intervalMinutes: number, csrfToken: string): Promise<AdminSettings> {
	const payload = asRecord(await adminRequest<unknown>('/settings/crawler', {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrfToken },
		body: JSON.stringify({ interval_hours: intervalHours, interval_minutes: intervalMinutes })
	}));
	return {
		crawler: parseCrawlerSettings(payload.crawler),
		runtime: parseCrawlerRuntime(payload.runtime)
	};
}

async function adminRequest<T>(path: string, init: RequestInit): Promise<T> {
	const controller = new AbortController();
	const timeoutId = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
	let response: Response;
	try {
		response = await fetch(apiUrl(`${ADMIN_PATH}${path}`), {
			...init,
			credentials: 'include',
			signal: controller.signal
		});
	} catch (error) {
		if (isAbortError(error)) throw new ApiError('Admin service phản hồi quá lâu. Vui lòng thử lại.', 408, 'request_timeout');
		throw new ApiError('Không thể kết nối đến admin service.', 0, 'network_error');
	} finally {
		clearTimeout(timeoutId);
	}

	const contentLength = Number(response.headers.get('content-length'));
	if (Number.isFinite(contentLength) && contentLength > MAX_RESPONSE_BYTES) {
		throw new ApiError('Admin service trả về dữ liệu quá lớn.', 502, 'response_too_large');
	}
	const rawBody = await response.text();
	if (new TextEncoder().encode(rawBody).byteLength > MAX_RESPONSE_BYTES) {
		throw new ApiError('Admin service trả về dữ liệu quá lớn.', 502, 'response_too_large');
	}
	if (response.status === 204) return undefined as T;

	let body: unknown;
	try {
		body = rawBody ? JSON.parse(rawBody) : undefined;
	} catch {
		throw new ApiError('Admin service trả về JSON không hợp lệ.', response.status, 'invalid_json');
	}
	if (!response.ok) {
		throw new ApiError(adminErrorMessage(response.status, body), response.status, errorCode(body));
	}
	return body as T;
}

function parseAuthResponse(value: unknown): AdminAuthResponse {
	const record = asRecord(value);
	const admin = asRecord(record.admin);
	return {
		admin: {
			id: stringField(admin.id, 100),
			email: stringField(admin.email, 320),
			isActive: booleanField(admin.is_active),
			...(admin.last_login_at === undefined || admin.last_login_at === null ? {} : { lastLoginAt: stringField(admin.last_login_at, 80) })
		},
		csrfToken: stringField(record.csrf_token, 200)
	};
}

function parsePromotion(value: unknown): PromotionSlide {
	const record = asRecord(value);
	const slot = integerField(record.slot, 1, 3);
	const imageUrl = stringField(record.image_url, 1000);
	if (!imageUrl.startsWith('/') || imageUrl.startsWith('//')) {
		throw new ApiError('Admin service trả về URL ảnh không hợp lệ.', 502, 'invalid_promotion_response');
	}
	return {
		slot,
		imageUrl: apiUrl(imageUrl),
		altText: stringField(record.alt_text, 180),
		...(optionalString(record.eyebrow, 80) ? { eyebrow: optionalString(record.eyebrow, 80) } : {}),
		...(optionalString(record.title, 160) ? { title: optionalString(record.title, 160) } : {}),
		...(optionalString(record.body, 320) ? { body: optionalString(record.body, 320) } : {}),
		...(record.target_url === undefined || record.target_url === null ? {} : { targetUrl: safeHttpUrl(record.target_url) }),
		contentHash: stringMatching(record.content_hash, /^[a-f0-9]{64}$/, 64)
	};
}

function parseJob(value: unknown): AdminJob {
	const record = asRecord(value);
	return {
		id: stringField(record.id, 100),
		sourceKey: stringField(record.source_key, 160),
		sourceName: stringField(record.source_name, 200),
		sourceApprovalStatus: stringField(record.source_approval_status, 32),
		isDevelopmentFixture: booleanField(record.is_development_fixture),
		title: stringField(record.title, 320),
		company: stringField(record.company, 240),
		location: stringField(record.location, 240),
		...(record.location_id === undefined || record.location_id === null ? {} : { locationId: stringField(record.location_id, 100) }),
		role: stringField(record.role, 160),
		requiredSkills: stringArray(record.required_skills, 30, 80),
		preferredSkills: stringArray(record.preferred_skills, 30, 80),
		seniority: stringField(record.seniority, 80),
		...(record.minimum_experience_years === undefined || record.minimum_experience_years === null ? {} : { minimumExperienceYears: numberField(record.minimum_experience_years) }),
		domains: stringArray(record.domains, 30, 80),
		employmentType: stringField(record.employment_type, 80),
		workMode: stringField(record.work_mode, 32).toLowerCase(),
		status: stringField(record.status, 32).toLowerCase(),
		originalUrl: safeHttpUrl(record.original_url),
		contentHash: stringField(record.content_hash, 200),
		lastSeenAt: stringField(record.last_seen_at, 80),
		updatedAt: stringField(record.updated_at, 80)
	};
}

function parseLocation(value: unknown): AdminLocation {
	const record = asRecord(value);
	const latitude = optionalNumber(record.latitude);
	const longitude = optionalNumber(record.longitude);
	return {
		id: stringField(record.id, 100),
		displayName: stringField(record.display_name, 200),
		province: stringField(record.province, 160),
		country: stringField(record.country, 120),
		canonicalKey: stringField(record.canonical_key, 240),
		...(latitude === undefined ? {} : { latitude }),
		...(longitude === undefined ? {} : { longitude }),
		isActive: booleanField(record.is_active),
		jobCount: integerField(record.job_count, 0, Number.MAX_SAFE_INTEGER),
		createdAt: stringField(record.created_at, 80),
		updatedAt: stringField(record.updated_at, 80)
	};
}

function parseLocationOption(value: unknown): AdminLocationOption {
	const record = asRecord(value);
	return {
		id: stringField(record.id, 100),
		displayName: stringField(record.display_name, 200),
		isActive: booleanField(record.is_active)
	};
}

function parseJobLink(value: unknown): AdminJobLink {
	const record = asRecord(value);
	const approvedAt = optionalString(record.approved_at, 80);
	const approvedBy = optionalString(record.approved_by, 320);
	return {
		id: stringField(record.id, 100),
		url: safeHttpUrl(record.url),
		sourceKey: stringField(record.source_key, 200),
		displayName: stringField(record.display_name, 240),
		approvalStatus: stringField(record.approval_status, 32),
		...(approvedAt ? { approvedAt } : {}),
		...(approvedBy ? { approvedBy } : {}),
		createdAt: stringField(record.created_at, 80),
		updatedAt: stringField(record.updated_at, 80),
		...(optionalString(record.last_crawl_status, 32) ? { lastCrawlStatus: optionalString(record.last_crawl_status, 32) } : {}),
		...(optionalString(record.last_crawl_at, 80) ? { lastCrawlAt: optionalString(record.last_crawl_at, 80) } : {}),
		...(optionalString(record.active_crawl_request_id, 100) ? { activeCrawlRequestId: optionalString(record.active_crawl_request_id, 100) } : {}),
		...(optionalString(record.active_crawl_request_status, 32) ? { activeCrawlRequestStatus: optionalString(record.active_crawl_request_status, 32) } : {}),
		...(record.last_crawl_pages === undefined ? {} : { lastCrawlPages: integerField(record.last_crawl_pages, 0, Number.MAX_SAFE_INTEGER) }),
		...(record.last_crawl_jobs === undefined ? {} : { lastCrawlJobs: integerField(record.last_crawl_jobs, 0, Number.MAX_SAFE_INTEGER) }),
		...(record.last_crawl_created === undefined ? {} : { lastCrawlCreated: integerField(record.last_crawl_created, 0, Number.MAX_SAFE_INTEGER) }),
		...(record.last_crawl_updated === undefined ? {} : { lastCrawlUpdated: integerField(record.last_crawl_updated, 0, Number.MAX_SAFE_INTEGER) }),
		...(record.last_crawl_missing === undefined ? {} : { lastCrawlMissing: integerField(record.last_crawl_missing, 0, Number.MAX_SAFE_INTEGER) }),
		...(optionalString(record.last_crawl_error_code, 160) ? { lastCrawlErrorCode: optionalString(record.last_crawl_error_code, 160) } : {})
	};
}

function parseCrawlerSettings(value: unknown): AdminCrawlerSettings {
	const record = asRecord(value);
	return {
		intervalHours: integerField(record.interval_hours, 0, 10080),
		intervalMinutes: integerField(record.interval_minutes, 0, 59),
		intervalSeconds: integerField(record.interval_seconds, 900, 604800),
		minIntervalMinutes: integerField(record.min_interval_minutes, 1, 604800),
		maxIntervalMinutes: integerField(record.max_interval_minutes, 1, 604800)
	};
}

function parseCrawlerRuntime(value: unknown): AdminCrawlerRuntime {
	const record = asRecord(value);
	return {
		status: stringField(record.status, 32),
		...(optionalString(record.last_heartbeat_at, 80) ? { lastHeartbeatAt: optionalString(record.last_heartbeat_at, 80) } : {}),
		...(optionalString(record.last_cycle_started_at, 80) ? { lastCycleStartedAt: optionalString(record.last_cycle_started_at, 80) } : {}),
		...(optionalString(record.last_cycle_finished_at, 80) ? { lastCycleFinishedAt: optionalString(record.last_cycle_finished_at, 80) } : {}),
		...(optionalString(record.next_cycle_at, 80) ? { nextCycleAt: optionalString(record.next_cycle_at, 80) } : {}),
		...(optionalString(record.current_source_key, 200) ? { currentSourceKey: optionalString(record.current_source_key, 200) } : {}),
		...(optionalString(record.last_error_code, 160) ? { lastErrorCode: optionalString(record.last_error_code, 160) } : {})
	};
}

function parseCrawlRequest(value: unknown): AdminCrawlRequest {
	const record = asRecord(value);
	return {
		id: stringField(record.id, 100),
		sourceId: stringField(record.source_id, 100),
		status: stringField(record.status, 32),
		requestedBy: stringField(record.requested_by, 320),
		requestedAt: stringField(record.requested_at, 80),
		...(optionalString(record.started_at, 80) ? { startedAt: optionalString(record.started_at, 80) } : {}),
		...(optionalString(record.finished_at, 80) ? { finishedAt: optionalString(record.finished_at, 80) } : {}),
		...(optionalString(record.source_run_id, 100) ? { sourceRunId: optionalString(record.source_run_id, 100) } : {}),
		...(optionalString(record.error_code, 160) ? { errorCode: optionalString(record.error_code, 160) } : {})
	};
}

function apiUrl(path: string): string {
	return resolveApiUrl(path, import.meta.env.PUBLIC_API_BASE_URL, import.meta.env.PROD, 'Cấu hình admin service không hợp lệ.');
}

function asRecord(value: unknown): Record<string, unknown> {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ApiError('Admin service trả về dữ liệu không hợp lệ.', 502, 'invalid_response');
	return value as Record<string, unknown>;
}

function stringField(value: unknown, maxLength: number): string {
	if (typeof value !== 'string' || !value.trim() || value.length > maxLength) throw new ApiError('Admin service trả về dữ liệu thiếu trường bắt buộc.', 502, 'invalid_response');
	return value;
}

function optionalString(value: unknown, maxLength: number): string | undefined {
	if (value === undefined || value === null || value === '') return undefined;
	return stringField(value, maxLength);
}

function stringMatching(value: unknown, expression: RegExp, maxLength: number): string {
	const parsed = stringField(value, maxLength);
	if (!expression.test(parsed)) throw new ApiError('Admin service trả về hash không hợp lệ.', 502, 'invalid_response');
	return parsed;
}

function booleanField(value: unknown): boolean {
	if (typeof value !== 'boolean') throw new ApiError('Admin service trả về cờ dữ liệu không hợp lệ.', 502, 'invalid_response');
	return value;
}

function numberField(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) throw new ApiError('Admin service trả về số không hợp lệ.', 502, 'invalid_response');
	return value;
}

function optionalNumber(value: unknown): number | undefined {
	if (value === undefined || value === null) return undefined;
	return numberField(value);
}

function integerField(value: unknown, minimum: number, maximum: number): number {
	const number = numberField(value);
	if (!Number.isInteger(number) || number < minimum || number > maximum) throw new ApiError('Admin service trả về phân trang không hợp lệ.', 502, 'invalid_response');
	return number;
}

function stringArray(value: unknown, maxItems: number, maxLength: number): string[] {
	if (!Array.isArray(value) || value.length > maxItems) throw new ApiError('Admin service trả về danh sách không hợp lệ.', 502, 'invalid_response');
	return value.map((item) => stringField(item, maxLength));
}

function safeHttpUrl(value: unknown): string {
	const raw = stringField(value, 1000);
	try {
		const parsed = new URL(raw);
		if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) throw new Error('unsafe URL');
		return parsed.toString();
	} catch {
		throw new ApiError('Admin service trả về URL không hợp lệ.', 502, 'invalid_response');
	}
}

function errorCode(value: unknown): string | undefined {
	if (typeof value === 'object' && value !== null && !Array.isArray(value) && typeof (value as Record<string, unknown>).code === 'string') return (value as Record<string, string>).code;
	return undefined;
}

function adminErrorMessage(status: number, value: unknown): string {
	const code = errorCode(value);
	if (status === 401 && code === 'admin_invalid_credentials') return 'Email hoặc mật khẩu không đúng.';
	if (status === 401) return 'Phiên quản trị không còn hợp lệ. Vui lòng đăng nhập lại.';
	if (status === 403) return 'Thao tác quản trị chưa được xác thực đầy đủ.';
	if (status === 413) return 'Ảnh vượt quá dung lượng cho phép.';
	if (status === 409 && code === 'job_link_exists') return 'Job Link này đã được thêm trước đó.';
	if (status === 429) return 'Admin service đang nhận nhiều yêu cầu. Vui lòng thử lại sau.';
	return 'Admin service tạm thời không khả dụng.';
}

function isAbortError(value: unknown): boolean {
	return typeof value === 'object' && value !== null && 'name' in value && value.name === 'AbortError';
}
