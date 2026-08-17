<script lang="ts">
	import { onMount } from 'svelte';
	import { getContext } from 'svelte';
	import { getAdminJobs, getAdminPromotions, type AdminJobPage } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import { ApiError } from '$lib/shared/api/api-errors';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let loading = $state(true);
	let errorMessage = $state('');
	let promotionCount = $state(0);
	let jobPage = $state<AdminJobPage | null>(null);

	onMount(() => {
		void loadOverview();
	});

	async function loadOverview() {
		loading = true;
		errorMessage = '';
		try {
			const [promotions, jobs] = await Promise.all([getAdminPromotions(), getAdminJobs(1, 1)]);
			promotionCount = promotions.length;
			jobPage = jobs;
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể đọc trạng thái workspace.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Admin overview — Go get a job ferris</title>
	<meta name="description" content="Control room của Go get a job ferris." />
</svelte:head>

<section class="admin-page-heading">
	<div><h2>Control room.</h2></div>
</section>

{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadOverview()}>Thử lại</button></div>{/if}

{#if loading}
	<div class="admin-loading-card"><div class="admin-loading-orbit" aria-hidden="true"></div><p>Đang đọc dữ liệu thật từ API…</p></div>
{:else}
	<div class="admin-metric-grid" aria-label="Workspace metrics">
		<div class="admin-metric accent"><span class="admin-metric-label">Promotions live</span><strong class="admin-metric-value">{promotionCount}<small style="font-size: .9rem; color: var(--text-muted);"> / 3</small></strong><span class="admin-metric-note">Slot đang active trên client</span></div>
		<div class="admin-metric"><span class="admin-metric-label">Job cache</span><strong class="admin-metric-value">{jobPage?.total ?? 0}</strong><span class="admin-metric-note">Structured rows trong database</span></div>
		<div class="admin-metric"><span class="admin-metric-label">Public jobs</span><strong class="admin-metric-value">—</strong><span class="admin-metric-note">Chưa bật public feed trong tranche này</span></div>
	</div>

	<section class="admin-panel">
		<div class="admin-panel-header"><h3>Next actions</h3></div>
		<div class="admin-quick-list">
			<a class="admin-quick-link" href="/admin/promotions"><span><strong>Update promotions</strong></span><span class="admin-quick-arrow">→</span></a>
			<a class="admin-quick-link" href="/admin/jobs"><span><strong>Inspect job cache</strong></span><span class="admin-quick-arrow">→</span></a>
		</div>
	</section>
{/if}
