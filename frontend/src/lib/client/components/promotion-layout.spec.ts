import { describe, expect, it } from 'vitest';
import carouselStyles from './PromotionCarousel.svelte?raw';
import clientPageSource from '../../../routes/client/+page.svelte?raw';

function ruleBody(styles: string, selector: string) {
	const selectorStart = styles.indexOf(selector);
	const openingBrace = styles.indexOf('{', selectorStart);
	const closingBrace = styles.indexOf('}', openingBrace);

	return styles.slice(openingBrace + 1, closingBrace);
}

describe('promotion layout boundaries', () => {
	it('keeps the 16:9 carousel inside its grid column', () => {
		expect(ruleBody(carouselStyles, '.promotion-carousel')).toContain('width: 100%;');
		expect(ruleBody(carouselStyles, '.promotion-carousel')).toContain('max-width: 100%;');
		expect(ruleBody(carouselStyles, '.promotion-carousel')).toContain('min-width: 0;');
		expect(ruleBody(carouselStyles, '.promotion-carousel')).toContain('align-self: start;');
	});

	it('keeps the idle result state beside the upload flow on desktop', () => {
		expect(clientPageSource).toContain('class:has-promotions={promotions.length > 0}');
		expect(clientPageSource).toContain('class="hero-primary"');
		expect(clientPageSource).toContain('class="results-placeholder hero-results"');
	});
});
