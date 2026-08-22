import { writable } from 'svelte/store';
import type { ScanFieldErrors } from '$lib/client/validation/scan-form';
import type { JobMatch } from '$lib/shared/types/job';

export type ScanUiStatus = 'idle' | 'submitting' | 'polling' | 'success' | 'empty' | 'error';

export interface ScanState {
	status: ScanUiStatus;
	selectedFile: File | null;
	locationId: string;
	errors: ScanFieldErrors;
	errorMessage: string;
	matches: JobMatch[];
	scanId: string;
}

function initialState(): ScanState {
	return {
		status: 'idle',
		selectedFile: null,
		locationId: '',
		errors: {},
		errorMessage: '',
		matches: [],
		scanId: ''
	};
}

export function createScanStore() {
	const store = writable<ScanState>(initialState());

	return {
		subscribe: store.subscribe,
		patch(updates: Partial<ScanState>) {
			store.update((state) => ({ ...state, ...updates }));
		}
	};
}
