import { describe, expect, it } from 'vitest';
import type { PromotionSlide } from '../../shared/types/client';
import { MAX_PROMOTION_SLIDES, nextPromotionIndex, previousPromotionIndex, promotionTrackTransform, shouldAutoplay, visiblePromotionSlides } from './promotion-carousel';

const slide = (slot: number): PromotionSlide => ({
	slot,
	imageUrl: `/api/v1/client/promotions/${slot}/image`,
	altText: `Slide ${slot}`,
	contentHash: 'a'.repeat(64)
});

describe('promotion carousel behavior', () => {
	it('never exposes more than the three supported slots', () => {
		expect(MAX_PROMOTION_SLIDES).toBe(3);
		expect(visiblePromotionSlides([slide(1), slide(2), slide(3), slide(4)])).toHaveLength(3);
	});

	it('wraps next and previous navigation', () => {
		expect(nextPromotionIndex(2, 3)).toBe(0);
		expect(previousPromotionIndex(0, 3)).toBe(2);
		expect(nextPromotionIndex(0, 0)).toBe(0);
		expect(previousPromotionIndex(0, 0)).toBe(0);
	});

	it('moves the full-width track by one viewport per active slide', () => {
		expect(promotionTrackTransform(0, 3)).toBe('translate3d(0%, 0, 0)');
		expect(promotionTrackTransform(1, 3)).toBe('translate3d(-33.33333333333333%, 0, 0)');
		expect(promotionTrackTransform(2, 3)).toBe('translate3d(-66.66666666666666%, 0, 0)');
		expect(promotionTrackTransform(1, 1)).toBe('translate3d(0%, 0, 0)');
	});

	it('pauses autoplay for focus, reduced motion, hidden tabs, and one-slide states', () => {
		expect(shouldAutoplay(3, false, false, false)).toBe(true);
		expect(shouldAutoplay(3, true, false, false)).toBe(false);
		expect(shouldAutoplay(3, false, true, false)).toBe(false);
		expect(shouldAutoplay(3, false, false, true)).toBe(false);
		expect(shouldAutoplay(1, false, false, false)).toBe(false);
	});
});
