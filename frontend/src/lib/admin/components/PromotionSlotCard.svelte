<script lang="ts">
	import { onDestroy, untrack } from 'svelte';
	import type { PromotionSlide } from '$lib/shared/types/client';

	export interface PromotionFormValues {
		slot: number;
		file: File;
		altText: string;
	}

	interface Props {
		slot: number;
		promotion?: PromotionSlide;
		busy: boolean;
		onSave: (values: PromotionFormValues) => Promise<void>;
		onDelete: (slot: number) => Promise<void>;
	}

	let { slot, promotion, busy, onSave, onDelete }: Props = $props();
	let file = $state<File | null>(null);
	let fileError = $state('');
	let altText = $state('');
	let previewUrl = $state<string | null>(null);
	let previewImageUrl = $derived(previewUrl ?? promotion?.imageUrl ?? '');
	let generatedAltText = $derived(`Ảnh quảng bá Go get a job ferris - Slot ${String(slot).padStart(2, '0')}`);

	$effect(() => {
		const currentPromotionImage = promotion?.imageUrl;
		void currentPromotionImage;
		altText = generatedAltText;
		clearPreview();
		file = null;
		fileError = '';
	});

	onDestroy(clearPreview);

	function clearPreview() {
		const currentPreviewUrl = untrack(() => previewUrl);
		if (currentPreviewUrl) URL.revokeObjectURL(currentPreviewUrl);
		previewUrl = null;
	}

	function chooseFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const candidate = input.files?.[0] ?? null;
		fileError = '';
		if (!candidate) {
			clearPreview();
			file = null;
			return;
		}
		if (!['image/png', 'image/jpeg', 'image/webp'].includes(candidate.type)) {
			clearPreview();
			file = null;
			fileError = 'Chỉ nhận PNG, JPEG hoặc WebP.';
			input.value = '';
			return;
		}
		if (candidate.size > 5 * 1024 * 1024) {
			clearPreview();
			file = null;
			fileError = 'Ảnh không được vượt quá 5 MB.';
			input.value = '';
			return;
		}
		clearPreview();
		file = candidate;
		previewUrl = URL.createObjectURL(candidate);
	}

	async function save() {
		if (!file) {
			fileError = 'Chọn ảnh mới trước khi upload.';
			return;
		}
		fileError = '';
		await onSave({ slot, file, altText: altText.trim() || generatedAltText });
	}

	async function remove() {
		if (!promotion || !window.confirm(`Xóa promotion slot ${slot}?`)) return;
		await onDelete(slot);
	}
</script>

<article class="promotion-slot-card">
	<header class="promotion-slot-header"><strong>Slot {String(slot).padStart(2, '0')}</strong><span class:live={promotion} class="promotion-slot-status">{promotion ? 'Live' : 'Empty'}</span></header>
	<div class="promotion-preview">
		{#if previewImageUrl}
			<img src={previewImageUrl} alt={altText || generatedAltText} />
			<span class="promotion-preview-status">{file ? 'Ảnh đã chọn' : 'Đang public'}</span>
		{:else}
			<div class="promotion-empty"><span class="promotion-empty-mark" aria-hidden="true">+</span><strong>Chưa có ảnh</strong><span>Slot này chưa được public.</span></div>
		{/if}
	</div>
	<div class="promotion-form">
		<div class="admin-field"><label for={`promotion-file-${slot}`}>Ảnh quảng bá · 16:9 · 1920×1080</label><input id={`promotion-file-${slot}`} class="admin-file" type="file" accept="image/png,image/jpeg,image/webp" onchange={chooseFile} disabled={busy} /></div>
		{#if file}<span class="admin-help">Sẵn sàng: {file.name} · {(file.size / 1024 / 1024).toFixed(2)} MB</span>{/if}
		{#if fileError}<span class="field-error" role="alert">{fileError}</span>{/if}
		<div class="promotion-actions"><button class="admin-primary-button" type="button" onclick={save} disabled={busy}>{busy ? 'Đang upload…' : promotion ? 'Thay ảnh' : 'Upload ảnh'}</button>{#if promotion}<button class="admin-danger-button" type="button" onclick={remove} disabled={busy}>Xóa</button>{/if}</div>
	</div>
</article>
