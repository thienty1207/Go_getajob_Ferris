/**
 * Helpers thuần cho phần hiển thị Job Cache — chỉ thay đổi presentation,
 * không đụng API/database. Bản đồ enum raw sang nhãn tiếng Việt dễ đọc.
 */

const WORK_MODE_LABELS: Record<string, string> = {
	ONSITE: 'Tại văn phòng',
	HYBRID: 'Hybrid',
	REMOTE: 'Remote'
};

const EMPLOYMENT_LABELS: Record<string, string> = {
	FULL_TIME: 'Toàn thời gian',
	PART_TIME: 'Bán thời gian',
	CONTRACTOR: 'Hợp đồng',
	INTERN: 'Thực tập',
	VOLUNTEER: 'Tình nguyện',
	PER_DIEM: 'Theo ngày',
	OTHER: 'Khác'
};

function toTitleCase(value: string): string {
	return value
		.split('_')
		.filter(Boolean)
		.map((part) => part.toUpperCase())
		.join(' ');
}

export function workModeLabel(value: string): string {
	const normalized = value.trim().toUpperCase();
	if (!normalized) return '—';
	return WORK_MODE_LABELS[normalized] ?? toTitleCase(value);
}

// Các employment type đầy đủ, theo thứ tự token dài nhất trước để nhận diện chính xác
// các giá trị nhiều từ (FULL_TIME, PART_TIME, PER_DIEM) mà không tách đôi chúng.
const EMPLOYMENT_TOKENS: string[] = ['FULL_TIME', 'PART_TIME', 'PER_DIEM', 'CONTRACTOR', 'INTERN', 'VOLUNTEER', 'OTHER'];

// Mỗi enum khớp nguyên cụm; giữa các từ bên trong enum cho phép `_`, `-` hoặc space
// (FULL_TIME == FULL TIME == FULL-TIME). Dựng regex case-insensitive từ danh sách đầy đủ.
let employmentRegex: RegExp | null = null;
function getEmploymentRegex(): RegExp {
	if (!employmentRegex) {
		const alt = EMPLOYMENT_TOKENS.map((key) => key.split('_').join('[ _\\-]+')).join('|');
		employmentRegex = new RegExp(`(${alt})`, 'gi');
	}
	return employmentRegex;
}

// Chuẩn hoá một chuỗi enum đã khớp (có thể chứa space/dấu `-` bên trong) về key có `_`.
function toEmploymentKey(matched: string): string {
	return matched.split(/[ _\-]+/).map((w) => w.toUpperCase()).join('_');
}

// Tách phần chưa khớp enum (gap) thành các token chữ-số, bỏ hoàn toàn separator/punctuation.
function pushUnknownTokens(gap: string, out: string[]): void {
	for (const token of gap.toUpperCase().split(/[^A-Z0-9]+/)) {
		if (token) out.push(token);
	}
}

export function employmentLabels(value: string): string[] {
	const normalized = value.toUpperCase();
	if (!normalized.trim()) return [];

	const result: string[] = [];
	let lastEnd = 0;
	for (const match of normalized.matchAll(getEmploymentRegex())) {
		const index = match.index ?? 0;
		if (index > lastEnd) pushUnknownTokens(normalized.slice(lastEnd, index), result);
		lastEnd = index + match[0].length;
		result.push(EMPLOYMENT_LABELS[toEmploymentKey(match[0])]);
	}
	if (lastEnd < normalized.length) pushUnknownTokens(normalized.slice(lastEnd), result);
	return result;
}

export function sourceDisplayLabel(value: string): string {
	if (!value) return '—';
	return value
		.replace(/^https?:\/\//i, '')
		.replace(/^www\./i, '')
		.replace(/\/+$/, '')
		.trim();
}

export const ALL_WORK_MODES = Object.keys(WORK_MODE_LABELS);
export const ALL_EMPLOYMENT_TYPES = Object.keys(EMPLOYMENT_LABELS);
