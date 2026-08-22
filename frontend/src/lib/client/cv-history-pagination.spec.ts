import { describe, expect, it } from 'vitest';
import { clampCVHistoryPage } from './cv-history-pagination';

describe('CV history pagination after mutation', () => {
	it('moves back when deleting the only row on the last page', () => {
		expect(clampCVHistoryPage(2, 10, 10)).toBe(1);
	});

	it('keeps the current page while it is still populated', () => {
		expect(clampCVHistoryPage(2, 11, 10)).toBe(2);
	});

	it('keeps the empty history on page one', () => {
		expect(clampCVHistoryPage(3, 0, 10)).toBe(1);
	});
});
