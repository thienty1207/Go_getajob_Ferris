<script lang="ts">
	import { page } from '$app/state';
	import { onMount, getContext } from 'svelte';
	import JobCacheTable from '$lib/admin/components/JobCacheTable.svelte';
	import { assignAdminJobLocation, getAdminJobs, getAdminLocationOptions, type AdminJobPage, type AdminLocationOption } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import { ApiError } from '$lib/shared/api/api-errors';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let data = $state<AdminJobPage | null>(null);
	let loading = $state(true);
	let errorMessage = $state('');
	let locations = $state<AdminLocationOption[]>([]);
	let search = $state('');
	let selectedLocationId = $state('');
	let unresolvedLocation = $state(false);
	let assigningJobId = $state<string | null>(null);

	onMount(() => {
		search = page.url.searchParams.get('q') ?? '';
		selectedLocationId = page.url.searchParams.get('location_id') ?? '';
		unresolvedLocation = page.url.searchParams.get('unresolved') === 'true';
		void loadData();
	});

	async function loadData() {
		loading = true;
		errorMessage = '';
		try {
			const [locationRows, jobPage] = await Promise.all([getAdminLocationOptions(), getAdminJobs(1, 10, currentFilter())]);
			locations = locationRows;
			data = jobPage;
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể đọc dữ liệu job cache từ API.';
		} finally {
			loading = false;
		}
	}

	async function loadJobs(page: number) {
		loading = true;
		errorMessage = '';
		try {
			data = await getAdminJobs(page, 10, currentFilter());
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể đọc job cache từ API.';
		} finally {
			loading = false;
		}
	}

	function applyFilter() {
		if (unresolvedLocation) selectedLocationId = '';
		void loadJobs(1);
	}

	function currentFilter() {
		return { search: search.trim() || undefined, locationId: selectedLocationId || undefined, unresolvedLocation };
	}

	async function changeJobLocation(jobId: string, locationId: string | null) {
		if (!$auth.csrfToken || assigningJobId) return;
		assigningJobId = jobId;
		errorMessage = '';
		try {
			await assignAdminJobLocation(jobId, locationId, $auth.csrfToken);
			await loadJobs(data?.page ?? 1);
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể cập nhật location cho job.';
		} finally {
			assigningJobId = null;
		}
	}
</script>

<svelte:head>
	<title>Job Cache — Admin Go get a job ferris</title>
	<meta name="description" content="Danh sách structured job cache của Go get a job ferris." />
</svelte:head>

<section class="admin-page-heading">
	<div><h2>Job cache.</h2></div>
</section>

<section class="admin-panel admin-filter-panel">
	<form class="job-cache-filter" onsubmit={(event) => { event.preventDefault(); applyFilter(); }}>
		<div class="admin-field"><label for="job-search">Tìm kiếm</label><input id="job-search" bind:value={search} type="search" placeholder="Tên job, công ty, role, link…" autocomplete="off" /></div>
		<div class="admin-field"><label for="job-location-filter">Location</label><select id="job-location-filter" bind:value={selectedLocationId} disabled={unresolvedLocation}><option value="">Tất cả location</option>{#each locations as location (location.id)}<option value={location.id}>{location.displayName}{location.isActive ? '' : ' · DISABLED'}</option>{/each}</select></div>
		<label class="admin-checkbox-row"><input type="checkbox" bind:checked={unresolvedLocation} /><span>Chưa gán location</span></label>
		<button class="admin-primary-button" type="submit" disabled={loading}>Tìm</button>
	</form>
</section>

{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadJobs(data?.page ?? 1)}>Thử lại</button></div>{/if}
{#if loading}
	<div class="admin-loading-card"><div class="admin-loading-orbit" aria-hidden="true"></div><p>Đang đọc job cache…</p></div>
{:else if data}
	<JobCacheTable data={data} locations={locations} assigningJobId={assigningJobId} onLocationChange={changeJobLocation} onPageChange={loadJobs} />
{/if}
