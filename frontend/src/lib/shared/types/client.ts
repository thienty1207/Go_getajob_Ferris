import type { JobMatch } from './job';

export interface ClientScanInput {
	file: File;
	locationId: string;
}

export interface ClientLocation {
	id: string;
	displayName: string;
	province: string;
	country: string;
	latitude?: number;
	longitude?: number;
}

export interface PromotionSlide {
	slot: number;
	imageUrl: string;
	altText: string;
	// Legacy metadata remains parseable for existing database rows, but the
	// admin workflow and public carousel treat the uploaded artwork as the
	// complete promotion surface.
	eyebrow?: string;
	title?: string;
	body?: string;
	targetUrl?: string;
	contentHash: string;
}

export interface ScanAccepted {
	scanId: string;
	status: 'processing';
}

export interface ScanProcessing {
	scanId: string;
	status: 'processing';
	phase: 'received' | 'parsing' | 'matching';
}

export interface ClientCVSummary {
	headline: string;
	overview: string;
	targetRoles: string[];
	strengths: string[];
	gaps: string[];
}

export interface ScanCompleted {
	scanId: string;
	status: 'completed';
	cvSummary?: ClientCVSummary;
	matches: JobMatch[];
}

export interface ScanFailed {
	scanId: string;
	status: 'failed';
	message: string;
}

export type ScanStatusResponse = ScanProcessing | ScanCompleted | ScanFailed;
