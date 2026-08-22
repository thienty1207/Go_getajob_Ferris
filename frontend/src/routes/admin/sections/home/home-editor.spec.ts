import { describe, expect, it } from 'vitest';
import editorSource from './+page.svelte?raw';
import mediaSource from '../../../../lib/client/components/HomeMediaMarquee.svelte?raw';

describe('non-technical Home section editor', () => {
	it('exposes only title, content, and image authoring fields', () => {
		expect(editorSource).toContain('>Tiêu đề</label>');
		expect(editorSource).toContain('>Nội dung</label>');
		expect(editorSource).toContain('>Ảnh section</label>');
		expect(editorSource).not.toMatch(/>Eyebrow<|>Alt text<|>Link \(không bắt buộc\)<|imageAltText|targetUrl|mediaAltText|mediaTargetUrl/);
	});

	it('keeps the section four upload boundary and mobile two-row layout explicit', () => {
		expect(editorSource).toContain('tối đa 10 ảnh');
		expect(editorSource).toContain('length: 10');
		expect(mediaSource).toContain('home-media-row');
		expect(mediaSource).toContain('home-media-row-reverse');
		expect(mediaSource).toContain('prefers-reduced-motion');
	});
});
