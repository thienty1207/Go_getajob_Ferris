<script lang="ts">
	import { onMount, getContext } from 'svelte';
	import { createAdminLocation, getAdminLocationPage, updateAdminLocation, type AdminLocation, type AdminLocationPage } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import { ApiError } from '$lib/shared/api/api-errors';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	const pageSize = 10;
	let data = $state<AdminLocationPage | null>(null);
	let currentPage = $state(1);
	let displayName = $state('');
	let province = $state('');
	let country = $state('Vietnam');
	let locationActive = $state(true);
	let editingId = $state<string | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let errorMessage = $state('');
	let successMessage = $state('');
	let isEditing = $derived(editingId !== null);
	let pageCount = $derived(Math.max(1, Math.ceil((data?.total ?? 0) / pageSize)));

	onMount(() => {
		void loadData();
	});

	async function loadData(page = currentPage) {
		loading = true;
		errorMessage = '';
		try {
			const result = await getAdminLocationPage(page, pageSize);
			if (result.items.length === 0 && result.total > 0 && page > 1) {
				currentPage = Math.min(page, Math.ceil(result.total / pageSize));
				data = await getAdminLocationPage(currentPage, pageSize);
			} else {
				currentPage = result.page;
				data = result;
			}
		} catch (error) {
			handleError(error, 'Không thể đọc dữ liệu Job Location từ API.');
		} finally {
			loading = false;
		}
	}

	async function saveLocation() {
		if (!$auth.csrfToken || saving || !displayName.trim() || !province.trim() || !country.trim()) return;
		saving = true;
		errorMessage = '';
		successMessage = '';
		const input = { displayName: displayName.trim(), province: province.trim(), country: country.trim(), isActive: locationActive };
		const wasEditing = editingId !== null;
		try {
			if (editingId) {
				await updateAdminLocation(editingId, input, $auth.csrfToken);
				successMessage = 'Đã cập nhật Job Location.';
			} else {
				await createAdminLocation(input, $auth.csrfToken);
				successMessage = 'Đã thêm Job Location.';
			}
			resetForm();
			currentPage = wasEditing ? currentPage : 1;
			await loadData(currentPage);
		} catch (error) {
			handleError(error, 'Không thể lưu Job Location.');
		} finally {
			saving = false;
		}
	}

	function editLocation(location: AdminLocation) {
		editingId = location.id;
		displayName = location.displayName;
		province = location.province;
		country = location.country;
		locationActive = location.isActive;
		errorMessage = '';
		successMessage = '';
	}

	function resetForm() {
		editingId = null;
		displayName = '';
		province = '';
		country = 'Vietnam';
		locationActive = true;
	}

	function handleError(error: unknown, fallback: string) {
		if (error instanceof ApiError && error.status === 401) auth.expire();
		errorMessage = error instanceof ApiError ? error.message : fallback;
	}

	async function goToPage(nextPage: number) {
		if (loading || nextPage < 1 || nextPage > pageCount || nextPage === currentPage) return;
		currentPage = nextPage;
		await loadData(nextPage);
	}
</script>

<svelte:head>
	<title>Job Location — Admin Go get a job ferris</title>
</svelte:head>

<section class="admin-page-heading">
	<div><h2>Job Location.</h2></div>
</section>

{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadData()}>Thử lại</button></div>{/if}
{#if successMessage}<div class="admin-status success" role="status"><span class="admin-status-symbol">✓</span><span>{successMessage}</span></div>{/if}

<section class="admin-panel admin-management-panel">
	<div class="admin-panel-header"><h3>{isEditing ? 'Sửa Job Location' : 'Thêm Job Location'}</h3></div>
	<form class="admin-form-grid" onsubmit={(event) => { event.preventDefault(); void saveLocation(); }}>
		<div class="admin-field wide"><label for="location-name">Tên hiển thị</label><input id="location-name" bind:value={displayName} placeholder="Thành phố Hồ Chí Minh" autocomplete="off" required /></div>
		<div class="admin-field"><label for="location-province">Tỉnh / thành phố</label><input id="location-province" bind:value={province} placeholder="Hồ Chí Minh" autocomplete="address-level1" required /></div>
		<div class="admin-field"><label for="location-country">Quốc gia</label><input id="location-country" bind:value={country} autocomplete="country-name" required /></div>
		<label class="admin-checkbox-row wide" for="location-active"><input id="location-active" bind:checked={locationActive} type="checkbox" /><span>Đang dùng</span></label>
		<div class="admin-form-actions wide"><button class="admin-primary-button" type="submit" disabled={saving || !displayName.trim() || !province.trim() || !country.trim()}>{saving ? 'Đang lưu…' : isEditing ? 'Lưu thay đổi' : 'Lưu Job Location'}</button>{#if isEditing}<button class="admin-secondary-button" type="button" onclick={resetForm} disabled={saving}>Hủy</button>{/if}</div>
	</form>
</section>

<section class="admin-panel admin-link-list-panel">
	<div class="admin-panel-header"><h3>Danh sách Job Location</h3><span class="admin-record-count">{data?.total ?? 0} locations</span></div>
	{#if loading}
		<div class="admin-empty-state"><span class="admin-loading-orbit" aria-hidden="true"></span><strong>Đang đọc Job Location</strong></div>
	{:else if !data || data.items.length === 0}
		<div class="admin-empty-state"><span class="admin-empty-state-mark" aria-hidden="true">⌖</span><strong>Chưa có Job Location</strong></div>
	{:else}
		<div class="admin-location-list">
			{#each data.items as location (location.id)}
				<div class="admin-location-row">
					<div><strong>{location.displayName}</strong><span>{location.province} · {location.country}</span></div>
					<div class="job-link-actions"><a class="admin-secondary-button" href={`/admin/jobs?location_id=${encodeURIComponent(location.id)}`}>{location.jobCount} job</a><span class:active={location.isActive} class:disabled={!location.isActive} class="job-badge">{location.isActive ? 'ACTIVE' : 'DISABLED'}</span><button class="admin-secondary-button" type="button" onclick={() => editLocation(location)} disabled={saving}>Sửa</button></div>
				</div>
			{/each}
		</div>
	{/if}
	{#if !loading && pageCount > 1}
		<div class="job-pagination" aria-label="Phân trang Job Location"><button class="admin-secondary-button" type="button" onclick={() => void goToPage(currentPage - 1)} disabled={currentPage <= 1}>← Trước</button><span>{currentPage} / {pageCount}</span><button class="admin-secondary-button" type="button" onclick={() => void goToPage(currentPage + 1)} disabled={currentPage >= pageCount}>Sau →</button></div>
	{/if}
</section>
