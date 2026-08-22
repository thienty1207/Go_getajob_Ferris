<script lang="ts">
	import type { JobMatch } from '$lib/shared/types/job';
	import type { ClientCVSummary } from '$lib/shared/types/client';
	import MatchJobCard from './MatchJobCard.svelte';

	interface Props { matches: JobMatch[]; summary?: ClientCVSummary; }
	let { matches, summary }: Props = $props();
</script>

<section class="results-section" aria-labelledby="matches-title">
	{#if summary}
		<article class="cv-summary-card">
			<div class="cv-summary-heading"><span class="summary-label">CV của bạn</span><h2>{summary.headline}</h2></div>
			<p class="cv-summary-overview">{summary.overview}</p>
			<div class="cv-summary-grid">
				<div><h3>Vai trò phù hợp</h3><ul>{#each summary.targetRoles as item}<li>{item}</li>{/each}</ul></div>
				<div><h3>Điểm mạnh</h3><ul>{#each summary.strengths as item}<li>{item}</li>{/each}</ul></div>
				<div><h3>Cần bổ sung</h3><ul>{#each summary.gaps as item}<li>{item}</li>{/each}</ul></div>
			</div>
		</article>
	{/if}

	<div class="results-heading"><h2 id="matches-title">Việc làm phù hợp</h2></div>
	{#if matches.length > 0}
		<div class="job-list">{#each matches as job (job.id)}<MatchJobCard {job} />{/each}</div>
	{/if}
</section>
