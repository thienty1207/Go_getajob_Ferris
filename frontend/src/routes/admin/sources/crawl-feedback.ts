import type { AdminJobLink } from '$lib/admin/api/admin-api';

export type CrawlFeedbackPhase = 'REQUESTING' | 'PENDING' | 'RUNNING' | 'RESULT';

export type CrawlFeedback = {
	phase: CrawlFeedbackPhase;
	requestedAt: string;
	requestId?: string;
	status?: string;
	pages?: number;
	jobs?: number;
	errorCode?: string;
};

type CrawlResultFields = Pick<
	AdminJobLink,
	| 'activeCrawlRequestStatus'
	| 'lastCrawlAt'
	| 'lastCrawlStatus'
	| 'lastCrawlPages'
	| 'lastCrawlJobs'
	| 'lastCrawlErrorCode'
>;

export function reconcileCrawlFeedback(feedback: CrawlFeedback, link: CrawlResultFields): CrawlFeedback {
	if (link.activeCrawlRequestStatus === 'PENDING' || link.activeCrawlRequestStatus === 'RUNNING') {
		return {
			...feedback,
			phase: link.activeCrawlRequestStatus,
			status: link.activeCrawlRequestStatus
		};
	}

	const requestedAt = Date.parse(feedback.requestedAt);
	const finishedAt = link.lastCrawlAt ? Date.parse(link.lastCrawlAt) : Number.NaN;
	if (link.lastCrawlAt && Number.isFinite(requestedAt) && Number.isFinite(finishedAt) && finishedAt >= requestedAt) {
		return {
			...feedback,
			phase: 'RESULT',
			status: link.lastCrawlStatus ?? 'COMPLETED',
			pages: link.lastCrawlPages,
			jobs: link.lastCrawlJobs,
			errorCode: link.lastCrawlErrorCode
		};
	}

	return feedback;
}
