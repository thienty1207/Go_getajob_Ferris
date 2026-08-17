<script lang="ts">
	import { onMount } from 'svelte';
	import { getContext } from 'svelte';
	import PromotionManager from '$lib/admin/components/PromotionManager.svelte';
	import { getAdminPromotions } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import type { PromotionSlide } from '$lib/shared/types/client';
	import { ApiError } from '$lib/shared/api/api-errors';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let promotions = $state<PromotionSlide[]>([]);
	let loading = $state(true);
	let errorMessage = $state('');

	onMount(() => {
		void loadPromotions();
	});

	async function loadPromotions() {
		loading = true;
		errorMessage = '';
		try {
			promotions = await getAdminPromotions();
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể đọc promotion từ API.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Promotions — Admin Go get a job ferris</title>
	<meta name="description" content="Quản lý tối đa ba promotion của Go get a job ferris." />
</svelte:head>

<section class="admin-page-heading">
	<div><h2>Ảnh quảng bá.</h2></div>
</section>

{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadPromotions()}>Thử lại</button></div>{/if}
{#if loading}
	<div class="admin-loading-card"><div class="admin-loading-orbit" aria-hidden="true"></div><p>Đang đọc promotion metadata…</p></div>
{:else}
	<PromotionManager initialPromotions={promotions} />
{/if}
