<script lang="ts">
	interface Props {
		status: 'submitting' | 'polling';
	}

	let { status }: Props = $props();
</script>

<section class="state-card processing-card" aria-live="polite" aria-busy="true">
	<div class="processing-orbit" aria-hidden="true"><span></span></div>
	<div class="processing-copy"><span class="section-kicker">{status === 'submitting' ? 'UPLOADING CV' : 'MATCHING IN PROGRESS'}</span><h2>{status === 'submitting' ? 'Đang gửi CV an toàn…' : 'Đang tìm các job phù hợp…'}</h2><p>Go get a job ferris đang chờ matching service hoàn tất. Đừng đóng cửa sổ này.</p></div>
	<div class="processing-steps" aria-label="Tiến trình xử lý"><span class:active={status === 'submitting'} class:done={status === 'polling'}>Upload CV</span><span class:active={status === 'polling'}>Matching</span><span>Results</span></div>
</section>
