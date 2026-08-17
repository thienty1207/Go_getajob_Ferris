import { get } from 'svelte/store';
import { describe, expect, it } from 'vitest';
import { createScanStore } from './scan-store';

describe('client scan store', () => {
	it('owns the scan lifecycle state without owning job records', () => {
		const scanStore = createScanStore();

		expect(get(scanStore)).toMatchObject({
			status: 'idle',
			selectedFile: null,
			locationId: '',
			radiusKm: 25,
			errors: {},
			errorMessage: '',
			matches: [],
			scanId: ''
		});

		scanStore.patch({ status: 'polling', locationId: 'location-hanoi', scanId: 'scan-from-api' });
		expect(get(scanStore)).toMatchObject({ status: 'polling', locationId: 'location-hanoi', scanId: 'scan-from-api' });
	});
});
