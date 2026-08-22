import { describe, expect, it } from 'vitest';
import { workModeLabel, employmentLabels, sourceDisplayLabel } from './job-labels';

describe('workModeLabel', () => {
	it('bản đồ ONSITE/onsite sang Tại văn phòng', () => {
		expect(workModeLabel('ONSITE')).toBe('Tại văn phòng');
		expect(workModeLabel('onsite')).toBe('Tại văn phòng');
	});
	it('bản đồ HYBRID/hybrid sang Hybrid', () => {
		expect(workModeLabel('HYBRID')).toBe('Hybrid');
		expect(workModeLabel('hybrid')).toBe('Hybrid');
	});
	it('bản đồ REMOTE/remote sang Remote', () => {
		expect(workModeLabel('REMOTE')).toBe('Remote');
		expect(workModeLabel('remote')).toBe('Remote');
	});
	it('giá trị lạ tách gạch dưới, in hoa, không để raw enum', () => {
		expect(workModeLabel('FIELD_WORK')).toBe('FIELD WORK');
	});
	it('chuỗi rỗng/undefined trả về placeholder an toàn', () => {
		expect(workModeLabel('')).toBe('—');
	});
});

describe('employmentLabels', () => {
	it('bản đồ single token sang tiếng Việt', () => {
		expect(employmentLabels('FULL_TIME')).toEqual(['Toàn thời gian']);
		expect(employmentLabels('PART_TIME')).toEqual(['Bán thời gian']);
		expect(employmentLabels('CONTRACTOR')).toEqual(['Hợp đồng']);
		expect(employmentLabels('INTERN')).toEqual(['Thực tập']);
		expect(employmentLabels('VOLUNTEER')).toEqual(['Tình nguyện']);
		expect(employmentLabels('PER_DIEM')).toEqual(['Theo ngày']);
		expect(employmentLabels('OTHER')).toEqual(['Khác']);
	});
	it('tách nhiều token gạch dưới thành danh sách', () => {
		expect(employmentLabels('FULL_TIME_PART_TIME_CONTRACTOR_INTERN_VOLUNTEER_PER_DIEM_OTHER')).toEqual([
			'Toàn thời gian',
			'Bán thời gian',
			'Hợp đồng',
			'Thực tập',
			'Tình nguyện',
			'Theo ngày',
			'Khác'
		]);
	});
	it('tách đúng các dạng separator (dấu chấm, phẩy, gạch chéo, space) không sinh badge dấu câu', () => {
		expect(employmentLabels('FULL_TIME.PART_TIME.CONTRACTOR')).toEqual(['Toàn thời gian', 'Bán thời gian', 'Hợp đồng']);
		expect(employmentLabels('FULL_TIME, PART_TIME, CONTRACTOR')).toEqual(['Toàn thời gian', 'Bán thời gian', 'Hợp đồng']);
		expect(employmentLabels('FULL_TIME/PART_TIME')).toEqual(['Toàn thời gian', 'Bán thời gian']);
		expect(employmentLabels('FULL TIME / PART TIME')).toEqual(['Toàn thời gian', 'Bán thời gian']);
		expect(employmentLabels('FULL TIME PART TIME')).toEqual(['Toàn thời gian', 'Bán thời gian']);
	});
	it('không bao giờ output token chỉ chứa punctuation (.,/;_-) hoặc chuỗi rỗng', () => {
		for (const input of ['FULL_TIME', 'FULL_TIME.PART_TIME', ', ', '.', 'FULL_TIME.. PART_TIME', 'FULL_TIME,;PART_TIME', '/']) {
			const labels = employmentLabels(input);
			for (const label of labels) {
				expect(label.trim()).not.toBe('');
				expect(/^[.,;:/_\-|()\s]+$/.test(label)).toBe(false);
			}
		}
	});
	it('token lạ giữ nguyên dạng in hoa chữ đầu', () => {
		expect(employmentLabels('FREELANCE_TEMP')).toEqual(['FREELANCE', 'TEMP']);
	});
	it('bỏ token rỗng và chuỗi rỗng trả về danh sách rỗng', () => {
		expect(employmentLabels('')).toEqual([]);
		expect(employmentLabels('FULL_TIME__INTERN')).toEqual(['Toàn thời gian', 'Thực tập']);
	});
	it('không bao giờ tách đôi enum thành các token lẻ (FULL/TIME/PART/PER/DIEM)', () => {
		for (const input of ['FULL_TIME', 'PART_TIME', 'PER_DIEM', 'FULL_TIME_PART_TIME', 'FULL_TIME_CONTRACTOR', 'PER_DIEM_INTERN']) {
			const labels = employmentLabels(input);
			const badSet = new Set(['FULL', 'TIME', 'PART', 'PER', 'DIEM']);
			for (const label of labels) {
				expect(badSet.has(label.toUpperCase())).toBe(false);
			}
		}
	});
	it('mỗi enum đầy đủ nằm trong đúng một phần tử (không tách cụm nhiều từ)', () => {
		expect(employmentLabels('FULL_TIME')).toHaveLength(1);
		expect(employmentLabels('PART_TIME')).toHaveLength(1);
		expect(employmentLabels('PER_DIEM')).toHaveLength(1);
		expect(employmentLabels('FULL_TIME_PART_TIME')).toHaveLength(2);
		expect(employmentLabels('FULL_TIME_PER_DIEM_CONTRACTOR')).toEqual(['Toàn thời gian', 'Theo ngày', 'Hợp đồng']);
	});
});

describe('sourceDisplayLabel', () => {
	it('bỏ prefix scheme/www và trailing slash', () => {
		expect(sourceDisplayLabel('https://tuyendung.viettel.vn/')).toBe('tuyendung.viettel.vn');
		expect(sourceDisplayLabel('https://www.fptjobs.com')).toBe('fptjobs.com');
	});
	it('giữ chuỗi không có scheme', () => {
		expect(sourceDisplayLabel('tuyendung.viettel.vn')).toBe('tuyendung.viettel.vn');
	});
	it('chuỗi rỗng trả về —', () => {
		expect(sourceDisplayLabel('')).toBe('—');
	});
});
