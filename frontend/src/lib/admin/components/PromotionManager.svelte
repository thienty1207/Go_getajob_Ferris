<script lang="ts">
	import { getContext } from 'svelte';
	import { ADMIN_AUTH_CONTEXT } from '../api/admin-session';
	import { deleteAdminPromotion, uploadAdminPromotion } from '../api/admin-api';
	import type { AdminAuthStore } from '../stores/admin-auth-store';
	import { ApiError } from '$lib/shared/api/api-errors';
	import type { PromotionSlide } from '$lib/shared/types/client';
	import PromotionSlotCard, { type PromotionFormValues } from './PromotionSlotCard.svelte';

	interface Props {
		initialPromotions: PromotionSlide[];
	}

	let { initialPromotions }: Props = $props();
	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let slides = $state<PromotionSlide[]>([]);
	let busySlot = $state<number | null>(null);
	let statusMessage = $state('');
	let errorMessage = $state('');

	$effect(() => {
		 slides = initialPromotions;
	});

	async function save(values: PromotionFormValues) {
		busySlot = values.slot;
		statusMessage = '';
		errorMessage = '';
		try {
			const promotion = await uploadAdminPromotion(values.slot, values, $auth.csrfToken);
			slides = [...slides.filter((item) => item.slot !== promotion.slot), promotion].sort((left, right) => left.slot - right.slot);
			statusMessage = `Slot ${values.slot} đã được cập nhật.`;
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể upload promotion lúc này.';
		} finally {
			busySlot = null;
		}
	}

	async function remove(slot: number) {
		busySlot = slot;
		statusMessage = '';
		errorMessage = '';
		try {
			await deleteAdminPromotion(slot, $auth.csrfToken);
			slides = slides.filter((item) => item.slot !== slot);
			statusMessage = `Slot ${slot} đã được xóa.`;
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể xóa promotion lúc này.';
		} finally {
			busySlot = null;
		}
	}
</script>

{#if statusMessage}<div class="admin-status success" role="status"><span class="admin-status-symbol">✓</span><span>{statusMessage}</span></div>{/if}
{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span></div>{/if}

<div class="promotion-admin-grid">
	{#each [1, 2, 3] as slot}
		<PromotionSlotCard slot={slot} promotion={slides.find((item) => item.slot === slot)} busy={busySlot === slot} onSave={save} onDelete={remove} />
	{/each}
</div>
