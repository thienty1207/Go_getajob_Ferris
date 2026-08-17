import { describe, expect, it } from 'vitest';
import { countdownSeconds, formatCountdown } from './crawler-runtime';

describe('crawler runtime presentation', () => {
	it('counts down from the server-provided next cycle', () => {
		const now = Date.parse('2026-08-17T12:00:00Z');
		expect(countdownSeconds('2026-08-17T12:03:07Z', now)).toBe(187);
		expect(formatCountdown(187)).toBe('00:03:07');
	});

	it('does not invent a countdown when the schedule is absent or passed', () => {
		const now = Date.parse('2026-08-17T12:00:00Z');
		expect(countdownSeconds(undefined, now)).toBeNull();
		expect(countdownSeconds('2026-08-17T11:59:59Z', now)).toBe(0);
	});
});
