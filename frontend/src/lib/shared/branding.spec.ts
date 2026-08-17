import { describe, expect, it } from 'vitest';
import adminLoginSource from '../../routes/admin/login/+page.svelte?raw';
import adminShellSource from '../admin/components/AdminShell.svelte?raw';
import clientHeaderSource from '../client/components/ClientHeader.svelte?raw';
import clientPageSource from '../../routes/client/+page.svelte?raw';

const brandLogoPath = '/brand/sugoi-oniichan-logo.png';
const websiteName = 'Go get a job ferris';

describe('product branding boundaries', () => {

	it('uses the Sugoi-oniichan logo instead of the placeholder F mark', () => {
		for (const source of [clientHeaderSource, adminShellSource, adminLoginSource]) {
			expect(source).toContain(`src="${brandLogoPath}"`);
			expect(source).toContain('alt="Sugoi-oniichan"');
			expect(source).not.toMatch(/brand(?:-mark)?[^>]*>F<\/span>/);
		}
	});

	it('keeps the website name separate from the brand name in visible entry copy', () => {
		for (const source of [clientHeaderSource, adminShellSource, adminLoginSource, clientPageSource]) {
			expect(source).toContain(websiteName);
		}
	});
});
