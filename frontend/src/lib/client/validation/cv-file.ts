export const MAX_CV_BYTES = 10 * 1024 * 1024;
export const ACCEPTED_CV_EXTENSIONS = ['pdf', 'docx', 'txt'] as const;

export interface ValidationResult {
	valid: boolean;
	message?: string;
}

export function validateCvFile(file: File | null): ValidationResult {
	if (!file) {
		return { valid: false, message: 'Vui lòng chọn file CV.' };
	}

	const extension = file.name.split('.').pop()?.toLowerCase();
	if (!extension || !ACCEPTED_CV_EXTENSIONS.includes(extension as (typeof ACCEPTED_CV_EXTENSIONS)[number])) {
		return { valid: false, message: 'CV chỉ hỗ trợ định dạng PDF, DOCX hoặc TXT.' };
	}

	if (file.size > MAX_CV_BYTES) {
		return { valid: false, message: 'CV phải có dung lượng tối đa 10 MB.' };
	}

	return { valid: true };
}
