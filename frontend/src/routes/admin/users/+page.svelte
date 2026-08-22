<script lang="ts">
	import { getContext, onMount } from 'svelte';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { getAdminClientUsers, type AdminClientUserPage } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let data = $state<AdminClientUserPage | null>(null);
	let query = $state('');
	let loading = $state(true);
	let errorMessage = $state('');

	onMount(() => { void loadUsers(1); });

	async function loadUsers(page: number) {
		loading = true;
		errorMessage = '';
		try {
			data = await getAdminClientUsers(page, 10, query);
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) auth.expire();
			errorMessage = error instanceof ApiError ? error.message : 'Không thể đọc danh sách user từ API.';
		} finally {
			loading = false;
		}
	}

	function submitFilter(event: SubmitEvent) { event.preventDefault(); void loadUsers(1); }
	function formatDate(value: string) { return new Intl.DateTimeFormat('vi-VN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)); }
</script>

<svelte:head>
	<title>Users — Admin Sugoi-oniichan</title>
	<meta name="description" content="Danh sách client user từ PostgreSQL." />
</svelte:head>

<section class="admin-page-heading"><div><h2>Users.</h2></div></section>

<section class="admin-panel">
	<div class="admin-panel-header"><h3>User list</h3>{#if data}<span class="admin-panel-meta">{data.total} user</span>{/if}</div>
	<form class="admin-filter-bar cv-filter-bar" onsubmit={submitFilter}>
		<div class="admin-field"><label for="user-search">User</label><input id="user-search" bind:value={query} placeholder="Email hoặc tên" autocomplete="off" /></div>
		<button class="admin-primary-button" type="submit" disabled={loading}>Tìm kiếm</button>
	</form>

	{#if errorMessage}
		<div class="admin-error-state" role="alert"><strong>Không thể tải user.</strong><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadUsers(data?.page ?? 1)}>Thử lại</button></div>
	{:else if loading}
		<div class="admin-loading-inline"><span class="admin-button-spinner" aria-hidden="true"></span> Đang đọc PostgreSQL…</div>
	{:else if data && data.items.length > 0}
		<div class="admin-user-list">
			{#each data.items as user (user.id)}
				<article class="admin-user-row">
					{#if user.avatarUrl}<img src={user.avatarUrl} alt="" class="admin-user-avatar" referrerpolicy="no-referrer" />{:else}<span class="admin-user-avatar admin-user-avatar-fallback" aria-hidden="true">{user.displayName.slice(0, 1).toUpperCase()}</span>{/if}
					<div class="admin-user-main"><strong>{user.displayName}</strong><span>{user.email}</span></div>
					<div class="admin-user-meta"><span>{user.provider}</span><small>Đăng nhập: {formatDate(user.lastLoginAt)}</small></div>
				</article>
			{/each}
		</div>
	{:else}
		<div class="admin-empty-state"><span class="admin-empty-state-mark" aria-hidden="true">◎</span><strong>Chưa có client user nào</strong><span>Dữ liệu sẽ xuất hiện sau lần đăng nhập Google đầu tiên.</span></div>
	{/if}
</section>

{#if data && data.total > data.pageSize}
	<nav class="admin-pagination" aria-label="Phân trang user"><button class="admin-secondary-button" type="button" onclick={() => void loadUsers(data!.page - 1)} disabled={data.page === 1}>← Trước</button><span>Trang {data.page} / {Math.ceil(data.total / data.pageSize)}</span><button class="admin-secondary-button" type="button" onclick={() => void loadUsers(data!.page + 1)} disabled={data.page >= Math.ceil(data.total / data.pageSize)}>Sau →</button></nav>
{/if}
