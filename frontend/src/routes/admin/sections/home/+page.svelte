<script lang="ts">
	import { getContext, onDestroy, onMount } from 'svelte';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import { createAdminHomeMedia, deleteAdminHomeMedia, getAdminHomeSections, updateAdminHomeMedia, updateAdminHomeSection, type AdminHomeMediaInput, type AdminHomeSectionInput } from '$lib/admin/api/admin-api';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import type { HomeSection, HomeSectionMedia } from '$lib/shared/types/home-section';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let sections = $state<HomeSection[]>([]);
	let loading = $state(true);
	let errorMessage = $state('');
	let successMessage = $state('');
	let savingSlot = $state<number | null>(null);
	let savingMedia = $state(false);
	let selectedFiles = $state<Record<number, File | undefined>>({});
	let selectedPreviewUrls = $state<Record<number, string>>({});
	let mediaFile = $state<File | null>(null);
	let mediaActive = $state(true);
	let mediaStrip = $derived(sections.find((section) => section.slot === 4));

	onMount(() => { void loadSections(); });
	onDestroy(() => {
		Object.values(selectedPreviewUrls).forEach((url) => URL.revokeObjectURL(url));
	});

	async function loadSections() {
		loading = true;
		errorMessage = '';
		try {
			sections = await getAdminHomeSections();
		} catch (error) {
			handleError(error, 'Không thể đọc section Home từ API.');
		} finally {
			loading = false;
		}
	}

	function chooseSectionFile(slot: number, event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (selectedPreviewUrls[slot]) URL.revokeObjectURL(selectedPreviewUrls[slot]);
		selectedFiles[slot] = file;
		selectedPreviewUrls[slot] = file ? URL.createObjectURL(file) : '';
	}

	function chooseMediaFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		mediaFile = input.files?.[0] ?? null;
	}

	async function saveSection(section: HomeSection) {
		if (!$auth.csrfToken || savingSlot !== null) return;
		savingSlot = section.slot;
		errorMessage = '';
		successMessage = '';
		const input: AdminHomeSectionInput = {
			isActive: section.isActive,
			title: section.title ?? '',
			body: section.body ?? '',
			...(selectedFiles[section.slot] ? { file: selectedFiles[section.slot] } : {})
		};
		try {
			const saved = await updateAdminHomeSection(section.slot, input, $auth.csrfToken);
			sections = sections.map((item) => item.slot === saved.slot ? saved : item);
			selectedFiles[section.slot] = undefined;
			successMessage = `Đã lưu section ${section.slot}.`;
		} catch (error) {
			handleError(error, 'Không thể lưu section Home.');
		} finally {
			savingSlot = null;
		}
	}

	async function addMedia() {
		const section = sections.find((item) => item.slot === 4);
		if (!$auth.csrfToken || savingMedia || !mediaFile || !section) return;
		const usedSortOrders = new Set(section.media.map((item) => item.sortOrder));
		const sortOrder = Array.from({ length: 10 }, (_, index) => index).find((index) => !usedSortOrders.has(index));
		if (sortOrder === undefined) {
			errorMessage = 'Section ảnh đã đạt tối đa 10 ảnh.';
			return;
		}
		savingMedia = true;
		errorMessage = '';
		successMessage = '';
		const input: AdminHomeMediaInput = { sortOrder, isActive: mediaActive, file: mediaFile };
		try {
			const media = await createAdminHomeMedia(4, input, $auth.csrfToken);
			sections = sections.map((item) => item.slot === 4 ? { ...item, isActive: true, media: [...item.media, media].sort((left, right) => left.sortOrder - right.sortOrder) } : item);
			mediaFile = null;
			successMessage = 'Đã thêm ảnh vào dải Home.';
		} catch (error) {
			handleError(error, 'Không thể upload ảnh section Home.');
		} finally {
			savingMedia = false;
		}
	}

	async function toggleMedia(media: HomeSectionMedia) {
		if (!$auth.csrfToken || savingMedia) return;
		savingMedia = true;
		try {
			const updated = await updateAdminHomeMedia(media.id, { isActive: !media.isActive }, $auth.csrfToken);
			updateMediaInState(updated);
		} catch (error) {
			handleError(error, 'Không thể cập nhật ảnh section.');
		} finally { savingMedia = false; }
	}

	async function removeMedia(media: HomeSectionMedia) {
		if (!$auth.csrfToken || savingMedia || !window.confirm('Xóa ảnh này khỏi dải Home?')) return;
		savingMedia = true;
		try {
			await deleteAdminHomeMedia(media.id, $auth.csrfToken);
			sections = sections.map((item) => item.slot === 4 ? { ...item, media: item.media.filter((current) => current.id !== media.id) } : item);
			successMessage = 'Đã xóa ảnh section.';
		} catch (error) {
			handleError(error, 'Không thể xóa ảnh section.');
		} finally { savingMedia = false; }
	}

	function updateMediaInState(updated: HomeSectionMedia) {
		sections = sections.map((item) => item.slot === 4 ? { ...item, media: item.media.map((media) => media.id === updated.id ? updated : media) } : item);
	}

	function handleError(error: unknown, fallback: string) {
		if (error instanceof ApiError && error.status === 401) auth.expire();
		errorMessage = error instanceof ApiError ? error.message : fallback;
	}
</script>

<svelte:head>
	<title>Quản lý section Home — Admin Sugoi-oniichan</title>
</svelte:head>

<section class="admin-page-heading"><div><h2>Quản lý section.</h2></div></section>

{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadSections()}>Thử lại</button></div>{/if}
{#if successMessage}<div class="admin-status success" role="status"><span class="admin-status-symbol">✓</span><span>{successMessage}</span></div>{/if}

{#if loading}
	<section class="admin-panel admin-empty-state"><span class="admin-loading-orbit" aria-hidden="true"></span><strong>Đang đọc section Home</strong></section>
{:else}
	<div class="home-section-admin-list">
		{#each sections.filter((section) => section.slot < 4) as section (section.slot)}
			<section class="admin-panel home-section-editor">
				<div class="admin-panel-header"><div><h3>Section {section.slot}</h3><span class="admin-panel-meta">{section.layout === 'IMAGE_LEFT' ? 'Ảnh trái · nội dung phải' : 'Nội dung trái · ảnh phải'}</span></div><label class="admin-checkbox-row"><input type="checkbox" bind:checked={section.isActive} /><span>Hiển thị</span></label></div>
				<div class="home-section-editor-grid">
					<div class="home-section-editor-copy">
						<div class="admin-field"><label for={`section-${section.slot}-title`}>Tiêu đề</label><input id={`section-${section.slot}-title`} bind:value={section.title} maxlength="180" /></div>
						<div class="admin-field"><label for={`section-${section.slot}-body`}>Nội dung</label><textarea id={`section-${section.slot}-body`} bind:value={section.body} maxlength="1200" rows="5"></textarea></div>
					</div>
					<div class="home-section-editor-media">
						{#if selectedPreviewUrls[section.slot] || section.imageUrl}<img src={selectedPreviewUrls[section.slot] || section.imageUrl} alt={section.title ?? 'Ảnh section Home'} />{:else}<div class="home-section-empty-media">Chưa có ảnh</div>{/if}
						<div class="admin-field"><label for={`section-${section.slot}-file`}>Ảnh section</label><input id={`section-${section.slot}-file`} type="file" accept="image/png,image/jpeg,image/webp" onchange={(event) => chooseSectionFile(section.slot, event)} /></div>
					</div>
				</div>
				<div class="admin-form-actions"><button class="admin-primary-button" type="button" onclick={() => void saveSection(section)} disabled={savingSlot !== null}>{savingSlot === section.slot ? 'Đang lưu…' : 'Lưu section'}</button></div>
			</section>
		{/each}

		{#if mediaStrip}
			<section class="admin-panel home-section-editor">
				<div class="admin-panel-header"><div><h3>Section 4 · Dải ảnh</h3><span class="admin-panel-meta">Ảnh chạy ngang liên tục trên Home · tối đa 10 ảnh</span></div><span class="admin-status-chip">{mediaStrip.media.length} / 10</span></div>
				<div class="home-media-admin-grid">
					<div class="home-media-upload-form">
						<div class="admin-field"><label for="home-media-file">Ảnh mới</label><input id="home-media-file" type="file" accept="image/png,image/jpeg,image/webp" onchange={chooseMediaFile} /></div>
						<label class="admin-checkbox-row"><input type="checkbox" bind:checked={mediaActive} /><span>Hiển thị trên Home</span></label>
						<button class="admin-primary-button" type="button" onclick={() => void addMedia()} disabled={savingMedia || !mediaFile}>{savingMedia ? 'Đang upload…' : 'Thêm ảnh'}</button>
					</div>
					<div class="home-media-admin-list">
						{#if mediaStrip.media.length === 0}<div class="admin-empty-state"><strong>Chưa có ảnh nào</strong><span>Slot này đang trống, không có nội dung mẫu.</span></div>{:else}{#each mediaStrip.media as media (media.id)}<article class:inactive={!media.isActive} class="home-media-admin-row"><img src={media.imageUrl} alt={`Ảnh dải Home ${media.sortOrder + 1}`} /><div><strong>Ảnh {media.sortOrder + 1}</strong><span>{media.isActive ? 'Đang hiển thị' : 'Đang ẩn'}</span></div><div class="home-media-admin-actions"><button class="admin-secondary-button" type="button" onclick={() => void toggleMedia(media)} disabled={savingMedia}>{media.isActive ? 'Ẩn' : 'Hiện'}</button><button class="admin-danger-button" type="button" onclick={() => void removeMedia(media)} disabled={savingMedia}>Xóa</button></div></article>{/each}{/if}
					</div>
				</div>
			</section>
		{/if}
	</div>
{/if}
