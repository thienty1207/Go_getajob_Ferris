import { ApiError } from '../../shared/api/api-errors';
import { resolveApiUrl } from '../../shared/api/api-url';
import { requestJson, type Fetcher } from '../../shared/api/http-client';
import type {
	ClientCertificationRecord,
	ClientCVHistoryItem,
	ClientCVHistoryPage,
	ClientEducationRecord,
	ClientStructuredProfile
} from '../../shared/types/client-cv';

const HISTORY_PATH = '/api/v1/client/cv-history';
const MAX_PAGE_SIZE = 10;
const MAX_ITEMS = 10;
const MAX_STRING = 240;

export async function getClientCVHistory(page = 1, pageSize = MAX_PAGE_SIZE, fetcher: Fetcher = fetch): Promise<ClientCVHistoryPage> {
	if (!Number.isInteger(page) || page < 1 || !Number.isInteger(pageSize) || pageSize < 1 || pageSize > MAX_PAGE_SIZE) {
		throw new ApiError('Thông số lịch sử CV không hợp lệ.', 400, 'invalid_cv_paging');
	}
	const query = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
	const payload = asRecord(await requestJson<unknown>(apiUrl(`${HISTORY_PATH}?${query}`), { credentials: 'include' }, fetcher));
	if (!Array.isArray(payload.items) || payload.items.length > MAX_ITEMS) {
		throw new ApiError('Lịch sử CV trả về quá nhiều dòng.', 502, 'invalid_cv_response');
	}
	return {
		items: payload.items.map(parseHistoryItem),
		page: integerField(payload.page, 'Trang lịch sử CV không hợp lệ.'),
		pageSize: integerField(payload.page_size, 'Kích thước lịch sử CV không hợp lệ.'),
		total: nonNegativeInteger(payload.total, 'Tổng lịch sử CV không hợp lệ.')
	};
}

export async function deleteClientCV(scanId: string, csrfToken: string, fetcher: Fetcher = fetch): Promise<void> {
	if (!scanId.trim() || !csrfToken.trim()) {
		throw new ApiError('Phiên thao tác CV không hợp lệ.', 400, 'invalid_cv_request');
	}
	await requestJson<void>(apiUrl(`${HISTORY_PATH}/${encodeURIComponent(scanId)}`), {
		method: 'DELETE',
		credentials: 'include',
		headers: { 'X-CSRF-Token': csrfToken }
	}, fetcher);
}

function apiUrl(path: string): string {
	return resolveApiUrl(path, import.meta.env.PUBLIC_API_BASE_URL, import.meta.env.PROD, 'Cấu hình matching service không hợp lệ.');
}

function parseHistoryItem(value: unknown): ClientCVHistoryItem {
	const record = asRecord(value);
	const status = stringField(record.status, MAX_STRING);
	if (!['received', 'parsing', 'matching', 'completed', 'failed'].includes(status)) {
		throw new ApiError('Lịch sử CV có trạng thái không hợp lệ.', 502, 'invalid_cv_response');
	}
	const matchCount = nonNegativeInteger(record.match_count, 'Số job match không hợp lệ.');
	return {
		scanId: stringField(record.scan_id, MAX_STRING),
		status: status as ClientCVHistoryItem['status'],
		location: stringField(record.location, MAX_STRING),
		createdAt: stringField(record.created_at, MAX_STRING),
		updatedAt: stringField(record.updated_at, MAX_STRING),
		matchCount,
		...(record.profile === undefined || record.profile === null ? {} : { profile: parseProfile(record.profile) })
	};
}

function parseProfile(value: unknown): ClientStructuredProfile {
	const record = asRecord(value);
	return {
		roles: stringArray(record.roles),
		skills: stringArray(record.skills),
		yearsOfExperience: finiteNumber(record.years_of_experience),
		seniority: stringField(record.seniority, 80),
		domains: stringArray(record.domains),
		education: recordArray(record.education, parseEducation),
		certifications: recordArray(record.certifications, parseCertification)
	};
}

function parseEducation(value: Record<string, unknown>): ClientEducationRecord {
	return {
		...(optionalString(value.institution) ? { institution: optionalString(value.institution) } : {}),
		...(optionalString(value.degree) ? { degree: optionalString(value.degree) } : {}),
		...(optionalString(value.field_of_study) ? { fieldOfStudy: optionalString(value.field_of_study) } : {}),
		...(optionalInteger(value.start_year) === undefined ? {} : { startYear: optionalInteger(value.start_year) }),
		...(optionalInteger(value.end_year) === undefined ? {} : { endYear: optionalInteger(value.end_year) }),
		...(optionalString(value.grade) ? { grade: optionalString(value.grade) } : {})
	};
}

function parseCertification(value: Record<string, unknown>): ClientCertificationRecord {
	return {
		...(optionalString(value.certificate_name) ? { certificateName: optionalString(value.certificate_name) } : {}),
		...(optionalString(value.issuer) ? { issuer: optionalString(value.issuer) } : {}),
		...(optionalInteger(value.issued_year) === undefined ? {} : { issuedYear: optionalInteger(value.issued_year) }),
		...(optionalInteger(value.expires_year) === undefined ? {} : { expiresYear: optionalInteger(value.expires_year) })
	};
}

function recordArray<T>(value: unknown, parser: (record: Record<string, unknown>) => T): T[] {
	if (!Array.isArray(value) || value.length > 20) throw new ApiError('Lịch sử CV có danh sách không hợp lệ.', 502, 'invalid_cv_response');
	return value.map((entry) => parser(asRecord(entry)));
}

function stringArray(value: unknown): string[] {
	if (!Array.isArray(value) || value.length > 100) throw new ApiError('Lịch sử CV có danh sách không hợp lệ.', 502, 'invalid_cv_response');
	return value.map((entry) => stringField(entry, MAX_STRING));
}

function optionalString(value: unknown): string | undefined {
	if (value === undefined || value === null || value === '') return undefined;
	return stringField(value, MAX_STRING);
}

function optionalInteger(value: unknown): number | undefined {
	if (value === undefined || value === null) return undefined;
	if (!Number.isInteger(value)) throw new ApiError('Lịch sử CV có năm không hợp lệ.', 502, 'invalid_cv_response');
	return value as number;
}

function asRecord(value: unknown): Record<string, unknown> {
	if (typeof value !== 'object' || value === null) throw new ApiError('Lịch sử CV trả về dữ liệu không hợp lệ.', 502, 'invalid_cv_response');
	return value as Record<string, unknown>;
}

function stringField(value: unknown, maxLength: number): string {
	if (typeof value !== 'string' || !value.trim() || value.length > maxLength) throw new ApiError('Lịch sử CV thiếu dữ liệu bắt buộc.', 502, 'invalid_cv_response');
	return value;
}

function integerField(value: unknown, message: string): number {
	if (!Number.isInteger(value) || (value as number) < 1) throw new ApiError(message, 502, 'invalid_cv_response');
	return value as number;
}

function nonNegativeInteger(value: unknown, message: string): number {
	if (!Number.isInteger(value) || (value as number) < 0) throw new ApiError(message, 502, 'invalid_cv_response');
	return value as number;
}

function finiteNumber(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value) || value < 0 || value > 100) throw new ApiError('Kinh nghiệm trong CV không hợp lệ.', 502, 'invalid_cv_response');
	return value;
}
