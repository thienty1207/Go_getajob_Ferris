import { describe, expect, it } from 'vitest';
import { reconcileCrawlFeedback, type CrawlFeedback } from './crawl-feedback';

describe('crawl feedback state', () => {
	it('keeps the button busy while the request is still pending or running', () => {
		const feedback: CrawlFeedback = {
			phase: 'PENDING',
			requestedAt: '2026-08-17T10:00:00Z',
			requestId: 'crawl-1'
		};

		const next = reconcileCrawlFeedback(feedback, {
			activeCrawlRequestStatus: 'RUNNING'
		});

		expect(next.phase).toBe('RUNNING');
		expect(next.status).toBe('RUNNING');
	});

	it('keeps the completed anomaly result visible with zero jobs', () => {
		const feedback: CrawlFeedback = {
			phase: 'RUNNING',
			requestedAt: '2026-08-17T10:00:00Z',
			requestId: 'crawl-1'
		};

		const next = reconcileCrawlFeedback(feedback, {
			lastCrawlAt: '2026-08-17T10:00:02Z',
			lastCrawlStatus: 'ANOMALY',
			lastCrawlPages: 1,
			lastCrawlJobs: 0,
			lastCrawlErrorCode: 'extraction_not_authoritative'
		});

		expect(next.phase).toBe('RESULT');
		expect(next.status).toBe('ANOMALY');
		expect(next.pages).toBe(1);
		expect(next.jobs).toBe(0);
		expect(next.errorCode).toBe('extraction_not_authoritative');
	});
});
