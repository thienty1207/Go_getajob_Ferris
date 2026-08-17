import { validateCvFile } from './cv-file';

export interface ScanFormInput {
	file: File | null;
	locationId: string;
	radiusKm: number | string;
}

export type ScanFieldErrors = Partial<Record<'file' | 'location' | 'radiusKm', string>>;

export function validateScanForm(input: ScanFormInput): ScanFieldErrors {
	const errors: ScanFieldErrors = {};
	const fileValidation = validateCvFile(input.file);

	if (!fileValidation.valid) {
		errors.file = fileValidation.message;
	}

	if (!input.locationId.trim()) {
		errors.location = 'Vui lòng chọn tỉnh/thành phố.';
	}

	const radius = typeof input.radiusKm === 'string' ? Number(input.radiusKm) : input.radiusKm;
	if (!Number.isFinite(radius) || radius <= 0) {
		errors.radiusKm = 'Vui lòng chọn bán kính tìm kiếm.';
	}

	return errors;
}

export function isScanFormValid(input: ScanFormInput): boolean {
	return Object.keys(validateScanForm(input)).length === 0;
}
