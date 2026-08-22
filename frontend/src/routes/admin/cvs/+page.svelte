<script lang="ts">
	import { getContext, onMount } from 'svelte';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { deleteAdminCVProfile, getAdminCVProfiles, type AdminCVProfile, type AdminCVProfilePage } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let data = $state<AdminCVProfilePage | null>(null);
	let user = $state('');
	let role = $state('');
	let loading = $state(true);
	let errorMessage = $state('');
	let confirmingId = $state('');
	let deletingId = $state('');

	onMount(() => { void loadCVs(1); });

	async function loadCVs(page: number) {
		loading = true;
		errorMessage = '';
		try {
			data = await getAdminCVProfiles(page, 10, user, role);
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể đọc danh sách CV từ API.';
		} finally {
			loading = false;
		}
	}

	function submitFilter(event: SubmitEvent) { event.preventDefault(); void loadCVs(1); }

	async function deleteCV(item: AdminCVProfile) {
		if (!$auth.csrfToken || deletingId) return;
		deletingId = item.scanId;
		try {
			await deleteAdminCVProfile(item.scanId, $auth.csrfToken);
			confirmingId = '';
			await loadCVs(data?.page ?? 1);
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể xóa CV lúc này.';
		} finally { deletingId = ''; }
	}

	function formatDate(value: string) { return new Intl.DateTimeFormat('vi-VN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)); }
</script>

<svelte:head>
	<title>CV Profiles — Admin Sugoi-oniichan</title>
	<meta name="description" content="Structured CV profiles từ client user, không có raw CV." />
</svelte:head>

<section class="admin-page-heading"><div><h2>CV profiles.</h2></div></section>

<section class="admin-panel">
	<div class="admin-panel-header"><h3>Profile registry</h3>{#if data}<span class="admin-panel-meta">{data.total} CV</span>{/if}</div>
	<form class="admin-filter-bar cv-filter-bar" onsubmit={submitFilter}>
		<div class="admin-field"><label for="cv-user">User</label><input id="cv-user" bind:value={user} placeholder="Email hoặc tên" autocomplete="off" /></div>
		<div class="admin-field"><label for="cv-role">Role</label><input id="cv-role" bind:value={role} placeholder="Role trong structured profile" autocomplete="off" /></div>
		<button class="admin-primary-button" type="submit" disabled={loading}>Tìm kiếm</button>
	</form>

	{#if errorMessage}
		<div class="admin-error-state" role="alert"><strong>Không thể tải CV.</strong><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadCVs(data?.page ?? 1)}>Thử lại</button></div>
	{:else if loading}
		<div class="admin-loading-inline"><span class="admin-button-spinner" aria-hidden="true"></span> Đang đọc structured profile…</div>
	{:else if data && data.items.length > 0}
		<div class="admin-cv-list">
			{#each data.items as item (item.scanId)}
				<article class="admin-cv-row">
					<div class="admin-cv-row-head"><div><span class="admin-status-chip">{item.status}</span><h3>{item.displayName}</h3><p>{item.email} · {item.location} · {formatDate(item.createdAt)}</p></div><div class="admin-cv-actions"><span>{item.matchCount} job match</span>{#if confirmingId === item.scanId}<button class="admin-danger-button" type="button" onclick={() => void deleteCV(item)} disabled={deletingId === item.scanId}>{deletingId === item.scanId ? 'Đang xóa…' : 'Xác nhận xóa'}</button><button class="admin-secondary-button" type="button" onclick={() => (confirmingId = '')} disabled={Boolean(deletingId)}>Hủy</button>{:else}<button class="admin-danger-button" type="button" onclick={() => (confirmingId = item.scanId)}>Xóa CV</button>{/if}</div></div>
					{#if item.profile}<div class="admin-cv-profile-grid"><div><span>Role</span><strong>{item.profile.roles.join(', ') || 'Chưa xác định'}</strong></div><div><span>Kỹ năng</span><strong>{item.profile.skills.slice(0, 5).join(', ') || 'Chưa xác định'}</strong></div><div><span>Kinh nghiệm</span><strong>{item.profile.yearsOfExperience} năm · {item.profile.seniority}</strong></div><div><span>Domain</span><strong>{item.profile.domains.join(', ') || 'Chưa xác định'}</strong></div></div>{:else}<p class="admin-cv-muted">Structured profile chưa sẵn sàng.</p>{/if}
				</article>
			{/each}
		</div>
	{:else}
		<div class="admin-empty-state"><span class="admin-empty-state-mark" aria-hidden="true">▤</span><strong>Chưa có CV nào</strong><span>CV user submit sẽ xuất hiện ở đây dưới dạng structured profile.</span></div>
	{/if}
</section>

{#if data && data.total > data.pageSize}
	<nav class="admin-pagination" aria-label="Phân trang CV"><button class="admin-secondary-button" type="button" onclick={() => void loadCVs(data!.page - 1)} disabled={data.page === 1}>← Trước</button><span>Trang {data.page} / {Math.ceil(data.total / data.pageSize)}</span><button class="admin-secondary-button" type="button" onclick={() => void loadCVs(data!.page + 1)} disabled={data.page >= Math.ceil(data.total / data.pageSize)}>Sau →</button></nav>
{/if}
