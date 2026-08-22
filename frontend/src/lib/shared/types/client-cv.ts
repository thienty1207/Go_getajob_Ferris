export interface ClientEducationRecord {
	institution?: string;
	degree?: string;
	fieldOfStudy?: string;
	startYear?: number;
	endYear?: number;
	grade?: string;
}

export interface ClientCertificationRecord {
	certificateName?: string;
	issuer?: string;
	issuedYear?: number;
	expiresYear?: number;
}

export interface ClientStructuredProfile {
	roles: string[];
	skills: string[];
	yearsOfExperience: number;
	seniority: string;
	domains: string[];
	education: ClientEducationRecord[];
	certifications: ClientCertificationRecord[];
}

export interface ClientCVHistoryItem {
	scanId: string;
	status: 'received' | 'parsing' | 'matching' | 'completed' | 'failed';
	location: string;
	createdAt: string;
	updatedAt: string;
	matchCount: number;
	profile?: ClientStructuredProfile;
}

export interface ClientCVHistoryPage {
	items: ClientCVHistoryItem[];
	page: number;
	pageSize: number;
	total: number;
}
