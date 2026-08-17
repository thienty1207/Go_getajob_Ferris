import { describe, expect, it } from 'vitest';
import { isScanFormValid, validateScanForm } from './scan-form';

describe('validateScanForm', () => {
	it('requires a location and a kilometre radius', () => {
		expect(validateScanForm({ file: null, locationId: '', radiusKm: 0 })).toEqual({
			file: 'Vui lòng chọn file CV.',
			location: 'Vui lòng chọn tỉnh/thành phố.',
			radiusKm: 'Vui lòng chọn bán kính tìm kiếm.'
		});
	});

	it('accepts a valid file, Vietnamese location and positive radius', () => {
		const selectedFile = new File(['cv'], 'resume.pdf', { type: 'application/pdf' });

		expect(validateScanForm({ file: selectedFile, locationId: 'location-hcm', radiusKm: 25 })).toEqual({});
	});

	it('reports whether the submit action is ready without sending a request', () => {
		const selectedFile = new File(['cv'], 'resume.pdf', { type: 'application/pdf' });

		expect(isScanFormValid({ file: null, locationId: '', radiusKm: 25 })).toBe(false);
		expect(isScanFormValid({ file: selectedFile, locationId: 'location-danang', radiusKm: 25 })).toBe(true);
	});
});
