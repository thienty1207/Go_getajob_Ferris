<script lang="ts">
	interface Props {
		phase: 'received' | 'parsing' | 'matching';
	}

	let { phase }: Props = $props();

	const phases = [
		{ id: 'received', label: 'Nhận CV' },
		{ id: 'parsing', label: 'Phân tích' },
		{ id: 'matching', label: 'Đối chiếu' }
	] as const;

	const phaseIndex = $derived(phases.findIndex((item) => item.id === phase));
	const phaseTitle = $derived(
		phase === 'received' ? 'Đang nhận CV…' : phase === 'parsing' ? 'Đang phân tích CV…' : 'Đang tìm việc phù hợp…'
	);
	const progress = $derived(`${Math.max(18, Math.min(88, (phaseIndex + 1) * 30))}%`);
</script>

<div class="scan-loading-backdrop" role="dialog" aria-modal="true" aria-live="polite" aria-busy="true">
	<section class="scan-loading-modal">
		<div class="scan-loading-icon" aria-hidden="true"><span></span></div>
		<h1>{phaseTitle}</h1>
		<div class="scan-loading-track" aria-hidden="true"><span style={`width: ${progress}`}></span></div>
		<div class="scan-loading-steps" aria-label="Tiến trình quét CV">
			{#each phases as item, index}
				<span class:current={index === phaseIndex} class:complete={index < phaseIndex}>{item.label}</span>
			{/each}
		</div>
	</section>
</div>
