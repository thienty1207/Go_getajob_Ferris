import { describe, expect, it } from 'vitest';
import { isScanFormValid, validateScanForm } from './scan-form';

describe('validateScanForm', () => {
	it('requires a CV file and canonical location', () => {
		expect(validateScanForm({ file: null, locationId: '' })).toEqual({
			file: 'Vui lòng chọn file CV.',
			location: 'Vui lòng chọn tỉnh/thành phố.'
		});
	});

	it('accepts a valid file and canonical location', () => {
		const selectedFile = new File(['cv'], 'resume.pdf', { type: 'application/pdf' });

		expect(validateScanForm({ file: selectedFile, locationId: 'location-hcm' })).toEqual({});
	});

	it('accepts a valid file and canonical location without a radius field', () => {
		const selectedFile = new File(['cv'], 'resume.pdf', { type: 'application/pdf' });

		expect(validateScanForm({ file: selectedFile, locationId: 'location-hcm' })).toEqual({});
	});

	it('reports whether the submit action is ready without sending a request', () => {
		const selectedFile = new File(['cv'], 'resume.pdf', { type: 'application/pdf' });

		expect(isScanFormValid({ file: null, locationId: '' })).toBe(false);
		expect(isScanFormValid({ file: selectedFile, locationId: 'location-danang' })).toBe(true);
	});
});
