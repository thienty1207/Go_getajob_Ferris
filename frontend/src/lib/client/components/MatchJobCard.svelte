<script lang="ts">
	import type { JobMatch } from '$lib/shared/types/job';

	interface Props { job: JobMatch; }
	let { job }: Props = $props();
	const workModeLabels = { remote: 'Remote', hybrid: 'Hybrid', onsite: 'Onsite' } as const;
</script>

<article class="job-card"><div class="match-ring" style={`--match: ${Math.max(0, Math.min(job.matchPercent, 100))}%`} aria-label={`${job.matchPercent}% CV Match`}><strong>{job.matchPercent}%</strong><span>Match</span></div><div class="job-main"><div class="job-title-row"><h3>{job.title}</h3><span class="source-dot" title="Nguồn tuyển dụng đã phản hồi"></span></div><p class="job-company">{job.company}</p><div class="job-meta"><span>{job.location}</span>{#if job.distanceKm !== undefined}<span>{job.distanceKm} km</span>{/if}<span>{job.employmentType}</span><span>{workModeLabels[job.workMode]}</span></div>{#if job.skillTags.length > 0}<div class="job-tags">{#each job.skillTags.slice(0, 3) as tag}<span>{tag}</span>{/each}</div>{/if}</div><div class="job-side"><div class="salary">{job.salary?.display ?? 'Lương chưa công bố'}</div><a class="job-link" href={job.originalUrl} target="_blank" rel="noopener noreferrer">Xem job <span aria-hidden="true">↗</span></a></div></article>
