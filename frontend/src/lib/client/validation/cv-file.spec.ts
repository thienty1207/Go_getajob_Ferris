import { describe, expect, it } from 'vitest';
import { MAX_CV_BYTES, validateCvFile } from './cv-file';

function file(name: string, size: number) {
	return new File([new Uint8Array(size)], name);
}

describe('validateCvFile', () => {
	it('accepts PDF, DOCX and TXT files up to 10 MB', () => {
		expect(validateCvFile(file('resume.pdf', 1024))).toEqual({ valid: true });
		expect(validateCvFile(file('resume.DOCX', 1024))).toEqual({ valid: true });
		expect(validateCvFile(file('resume.txt', MAX_CV_BYTES))).toEqual({ valid: true });
	});

	it('rejects an empty selection', () => {
		expect(validateCvFile(null)).toEqual({ valid: false, message: 'Vui lòng chọn file CV.' });
	});

	it('rejects unsupported extensions', () => {
		expect(validateCvFile(file('resume.zip', 1024))).toEqual({
			valid: false,
			message: 'CV chỉ hỗ trợ định dạng PDF, DOCX hoặc TXT.'
		});
	});

	it('rejects files larger than 10 MB', () => {
		expect(validateCvFile(file('resume.pdf', MAX_CV_BYTES + 1))).toEqual({
			valid: false,
			message: 'CV phải có dung lượng tối đa 10 MB.'
		});
	});
});
