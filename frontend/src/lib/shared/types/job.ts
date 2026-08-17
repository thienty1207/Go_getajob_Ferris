export type WorkMode = 'remote' | 'hybrid' | 'onsite';

export interface JobSalary {
	display: string;
	currency?: string;
}

export interface JobMatch {
	id: string;
	matchPercent: number;
	title: string;
	company: string;
	location: string;
	distanceKm?: number;
	employmentType: string;
	workMode: WorkMode;
	salary?: JobSalary;
	skillTags: string[];
	originalUrl: string;
}
