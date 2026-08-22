import { ApiError } from '../api/api-errors';

export type HomeSectionLayout = 'CONTENT_LEFT' | 'IMAGE_LEFT' | 'MEDIA_STRIP';

export interface HomeSectionMedia {
	id: string;
	sortOrder: number;
	isActive: boolean;
	imageAltText: string;
	imageUrl: string;
	imageContentHash: string;
	targetUrl?: string;
}

export interface HomeSection {
	id?: string;
	slot: number;
	layout: HomeSectionLayout;
	isActive: boolean;
	eyebrow?: string;
	title?: string;
	body?: string;
	imageAltText?: string;
	imageUrl?: string;
	imageContentHash?: string;
	targetUrl?: string;
	media: HomeSectionMedia[];
}

const HOME_SECTION_SLOTS = [1, 2, 3, 4] as const;

export function createEditableHomeSectionSlots(sections: HomeSection[]): HomeSection[] {
		return HOME_SECTION_SLOTS.map((slot) => sections.find((section) => section.slot === slot) ?? emptyEditableHomeSection(slot));
}

function emptyEditableHomeSection(slot: (typeof HOME_SECTION_SLOTS)[number]): HomeSection {
	return {
		slot,
		layout: slot === 2 ? 'IMAGE_LEFT' : slot === 4 ? 'MEDIA_STRIP' : 'CONTENT_LEFT',
		isActive: false,
		title: '',
		body: '',
		media: []
	};
}

export function parseHomeSectionsPayload(value: unknown): HomeSection[] {
	const record = asRecord(value);
	if (!Array.isArray(record.sections) || record.sections.length > 4) {
		throw new ApiError('Home service trả về danh sách section không hợp lệ.', 502, 'invalid_home_section_response');
	}
	const sections = record.sections.map(parseHomeSection);
	const seen = new Set<number>();
	for (const section of sections) {
		if (seen.has(section.slot)) throw new ApiError('Home service trả về slot bị trùng.', 502, 'invalid_home_section_response');
		seen.add(section.slot);
	}
	return sections.sort((left, right) => left.slot - right.slot);
}

export function parseHomeSection(value: unknown): HomeSection {
	const record = asRecord(value);
	const slot = integerField(record.slot, 1, 4);
	const layout = stringField(record.layout, 32) as HomeSectionLayout;
	const expectedLayout = slot === 2 ? 'IMAGE_LEFT' : slot === 4 ? 'MEDIA_STRIP' : 'CONTENT_LEFT';
	if (layout !== expectedLayout) throw new ApiError('Home service trả về layout không hợp lệ.', 502, 'invalid_home_section_response');
	const imageUrl = optionalImageUrl(record.image_url);
	const hash = optionalHash(record.image_content_hash);
	if (Boolean(imageUrl) !== Boolean(hash)) throw new ApiError('Home service trả về metadata ảnh thiếu.', 502, 'invalid_home_section_response');
	const mediaValue = record.media;
	if (!Array.isArray(mediaValue) || mediaValue.length > 12) throw new ApiError('Home service trả về quá nhiều ảnh section.', 502, 'invalid_home_section_response');
	return {
		...(optionalString(record.id, 100) ? { id: optionalString(record.id, 100) } : {}),
		slot,
		layout,
		isActive: booleanField(record.is_active),
		...(optionalString(record.eyebrow, 80) ? { eyebrow: optionalString(record.eyebrow, 80) } : {}),
		...(optionalString(record.title, 180) ? { title: optionalString(record.title, 180) } : {}),
		...(optionalString(record.body, 1200) ? { body: optionalString(record.body, 1200) } : {}),
		...(optionalString(record.image_alt_text, 180) ? { imageAltText: optionalString(record.image_alt_text, 180) } : {}),
		...(imageUrl ? { imageUrl } : {}),
		...(hash ? { imageContentHash: hash } : {}),
		...(optionalTargetUrl(record.target_url) ? { targetUrl: optionalTargetUrl(record.target_url) } : {}),
		media: mediaValue.map(parseHomeSectionMedia)
	};
}

export function parseHomeSectionMedia(value: unknown): HomeSectionMedia {
	const record = asRecord(value);
	const hash = stringField(record.image_content_hash, 64);
	if (!/^[a-f0-9]{64}$/.test(hash)) throw new ApiError('Home service trả về hash ảnh không hợp lệ.', 502, 'invalid_home_section_response');
	return {
		id: stringField(record.id, 100),
		sortOrder: integerField(record.sort_order, 0, 11),
		isActive: booleanField(record.is_active),
		imageAltText: stringField(record.image_alt_text, 180),
		imageUrl: safeImageUrl(record.image_url),
		imageContentHash: hash,
		...(optionalTargetUrl(record.target_url) ? { targetUrl: optionalTargetUrl(record.target_url) } : {})
	};
}

function asRecord(value: unknown): Record<string, unknown> {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ApiError('Home service trả về dữ liệu không hợp lệ.', 502, 'invalid_home_section_response');
	return value as Record<string, unknown>;
}

function stringField(value: unknown, maxLength: number): string {
	if (typeof value !== 'string' || !value.trim() || value.length > maxLength) throw new ApiError('Home service trả về dữ liệu thiếu.', 502, 'invalid_home_section_response');
	return value;
}

function optionalString(value: unknown, maxLength: number): string | undefined {
	if (value === undefined || value === null || value === '') return undefined;
	return stringField(value, maxLength);
}

function booleanField(value: unknown): boolean {
	if (typeof value !== 'boolean') throw new ApiError('Home service trả về trạng thái không hợp lệ.', 502, 'invalid_home_section_response');
	return value;
}

function integerField(value: unknown, minimum: number, maximum: number): number {
	if (typeof value !== 'number' || !Number.isInteger(value) || value < minimum || value > maximum) throw new ApiError('Home service trả về số không hợp lệ.', 502, 'invalid_home_section_response');
	return value;
}

function optionalImageUrl(value: unknown): string | undefined {
	if (value === undefined || value === null || value === '') return undefined;
	return safeImageUrl(value);
}

function safeImageUrl(value: unknown): string {
	const raw = stringField(value, 2048);
	try {
		const parsed = new URL(raw);
		if (parsed.protocol !== 'https:' || parsed.username || parsed.password) throw new Error('unsafe image URL');
		return parsed.toString();
	} catch {
		throw new ApiError('Home service trả về URL ảnh không hợp lệ.', 502, 'invalid_home_section_response');
	}
}

function optionalTargetUrl(value: unknown): string | undefined {
	if (value === undefined || value === null || value === '') return undefined;
	const raw = stringField(value, 2048);
	try {
		const parsed = new URL(raw);
		if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) throw new Error('unsafe target URL');
		return parsed.toString();
	} catch {
		throw new ApiError('Home service trả về link không hợp lệ.', 502, 'invalid_home_section_response');
	}
}

function optionalHash(value: unknown): string | undefined {
	if (value === undefined || value === null || value === '') return undefined;
	const hash = stringField(value, 64);
	if (!/^[a-f0-9]{64}$/.test(hash)) throw new ApiError('Home service trả về hash không hợp lệ.', 502, 'invalid_home_section_response');
	return hash;
}
