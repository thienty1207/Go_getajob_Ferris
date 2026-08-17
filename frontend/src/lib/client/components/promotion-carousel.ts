import type { PromotionSlide } from '../../shared/types/client';

export const MAX_PROMOTION_SLIDES = 3;

export function visiblePromotionSlides(slides: PromotionSlide[]): PromotionSlide[] {
	return slides.slice(0, MAX_PROMOTION_SLIDES);
}

export function nextPromotionIndex(index: number, count: number): number {
	if (count <= 0) return 0;
	return (index + 1) % count;
}

export function previousPromotionIndex(index: number, count: number): number {
	if (count <= 0) return 0;
	return (index - 1 + count) % count;
}

export function promotionTrackTransform(index: number, count: number): string {
	if (count <= 1) return 'translate3d(0%, 0, 0)';
	const boundedIndex = Math.min(Math.max(index, 0), count - 1);
	if (boundedIndex === 0) return 'translate3d(0%, 0, 0)';
	return `translate3d(-${(boundedIndex / count) * 100}%, 0, 0)`;
}

export function shouldAutoplay(count: number, paused: boolean, reducedMotion: boolean, pageHidden: boolean): boolean {
	return count > 1 && !paused && !reducedMotion && !pageHidden;
}
