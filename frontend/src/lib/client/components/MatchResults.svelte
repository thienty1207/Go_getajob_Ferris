<script lang="ts">
	import type { JobMatch } from '$lib/shared/types/job';
	import MatchJobCard from './MatchJobCard.svelte';

	interface Props { matches: JobMatch[]; }
	let { matches }: Props = $props();
</script>

<section class="results-section" aria-labelledby="matches-title"><div class="results-heading"><div><span class="section-kicker">MATCH RESULTS</span><h2 id="matches-title">Các cơ hội phù hợp với hồ sơ của bạn.</h2></div><span class="results-note">Xếp hạng deterministic · từ cao xuống thấp</span></div><div class="job-list">{#each matches as job (job.id)}<MatchJobCard {job} />{/each}</div></section>
