import { describe, expect, it } from 'vitest';
// The spec runs in Vitest's Node project; the application bundle never imports these built-ins.
// @ts-expect-error The frontend intentionally does not ship @types/node.
import { readFileSync } from 'node:fs';
// @ts-expect-error The frontend intentionally does not ship @types/node.
import { resolve } from 'node:path';

declare const process: { cwd(): string };
import clientHeaderSource from './ClientHeader.svelte?raw';
import clientLoginSource from '../../../routes/client/login/+page.svelte?raw';
import clientCVSource from '../../../routes/client/cv/+page.svelte?raw';
import homeContentSource from './HomeContentSection.svelte?raw';
import homeMediaSource from './HomeMediaMarquee.svelte?raw';

const sharedStylesSource = readFileSync(resolve(process.cwd(), 'src/lib/shared/styles/global.css'), 'utf8');

describe('client entry layout', () => {
	it('keeps the login screen brand in the shared header only', () => {
		expect(clientLoginSource).not.toContain('class="login-brand"');
		expect(clientLoginSource).toContain('<ClientHeader hideLoginAction />');
		expect(clientHeaderSource).toContain('hideLoginAction');
	});

	it('keeps one scan CTA on the CV history screen', () => {
		expect(clientCVSource).toContain('Quét CV mới');
		expect(clientCVSource).not.toContain('Bắt đầu quét');
		expect(clientCVSource.split('href="/client"').length - 1).toBe(1);
	});

	it('gives content sections an accessible label when the CMS title is empty', () => {
		expect(homeContentSource).toContain('aria-label={sectionLabel()}');
		expect(homeContentSource).toContain('function sectionLabel()');
		expect(homeMediaSource).toContain('aria-label={sectionLabel()}');
		expect(homeMediaSource).toContain('function sectionLabel()');
	});

	it('keeps public Home content to title, body, and image presentation', () => {
		expect(homeContentSource).not.toContain('section.eyebrow');
		expect(homeContentSource).not.toContain('section.targetUrl');
		expect(homeMediaSource).not.toContain('media.targetUrl');
	});

	it('keeps the Home page vertically scrollable while clipping only decorative overflow', () => {
		expect(sharedStylesSource).toContain('.client-page {');
		expect(sharedStylesSource).toContain('overflow-x: clip');
		expect(sharedStylesSource).toContain('overflow-y: visible');
		expect(sharedStylesSource).not.toMatch(/\.client-page \{[^}]*overflow:\s*hidden/);
	});

	it('keeps ambient decoration out of the document scroll height', () => {
		expect(sharedStylesSource).toContain('.ambient { position: fixed;');
	});
});
