import { describe, expect, it } from 'vitest';
import { toLocationPayload } from './location';

describe('toLocationPayload', () => {
	it('giữ nguyên UUID string hợp lệ khi chọn location thật', () => {
		const uuid = '3f2e1c8a-6f4d-4b8e-9c1a-2b3c4d5e6f7a';
		expect(toLocationPayload(uuid)).toBe(uuid);
	});

	it('trả về null khi chọn "Chưa gán" (chuỗi rỗng)', () => {
		expect(toLocationPayload('')).toBeNull();
	});

	it('trả về null khi giá trị là null/undefined', () => {
		expect(toLocationPayload(null)).toBeNull();
		expect(toLocationPayload(undefined)).toBeNull();
	});

	it('không gửi chuỗi rỗng lên API, chỉ UUID hoặc null', () => {
		// Payload phải là string (UUID) hoặc null — không bao giờ là ''.
		const payload = toLocationPayload('');
		expect(payload === '' || payload === null).toBe(true);
		expect(payload).toBeNull();
	});
});
