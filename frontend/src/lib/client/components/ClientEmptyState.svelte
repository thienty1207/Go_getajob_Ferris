<script lang="ts">
	interface Props { onRetry: () => void; }
	let { onRetry }: Props = $props();
</script>

<section class="state-card empty-card" aria-live="polite"><div class="state-symbol" aria-hidden="true">∅</div><div><span class="section-kicker">NO ACTIVE MATCHES</span><h2>Chưa tìm thấy job phù hợp.</h2><p>Thử mở rộng bán kính hoặc chọn một khu vực khác. Go get a job ferris chỉ hiển thị những job đang active từ nguồn được duyệt.</p></div><button class="secondary-button" type="button" onclick={onRetry}>Thử lại</button></section>
