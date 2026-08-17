<script lang="ts">
	interface Props { message: string; onRetry: () => void; }
	let { message, onRetry }: Props = $props();
</script>

<section class="state-card error-card" aria-live="assertive" role="alert"><div class="state-symbol" aria-hidden="true">!</div><div><span class="section-kicker">SERVICE UNAVAILABLE</span><h2>Chưa thể hoàn tất lần quét này.</h2><p>{message}</p></div><button class="secondary-button" type="button" onclick={onRetry}>Thử lại</button></section>
