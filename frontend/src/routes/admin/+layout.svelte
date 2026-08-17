<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount, setContext } from 'svelte';
	import AdminShell from '$lib/admin/components/AdminShell.svelte';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import { createAdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import '$lib/admin/styles/admin.css';

	let { children } = $props();
	const auth = createAdminAuthStore();
	setContext(ADMIN_AUTH_CONTEXT, auth);
	let isLoginPage = $derived(page.url.pathname === '/admin/login');

	onMount(() => {
		void auth.bootstrap();
	});

	$effect(() => {
		if (!isLoginPage && $auth.status === 'anonymous') {
			void goto('/admin/login');
		}
		if (isLoginPage && $auth.status === 'authenticated') {
			void goto('/admin');
		}
	});
</script>

<svelte:head>
	<meta name="robots" content="noindex,nofollow" />
</svelte:head>

{#if isLoginPage}
	{@render children()}
{:else if $auth.status === 'checking'}
	<div class="admin-loading"><div class="admin-loading-card"><div class="admin-loading-orbit" aria-hidden="true"></div><p>Đang xác thực phiên quản trị…</p></div></div>
{:else if $auth.status === 'authenticated'}
	<AdminShell>{@render children()}</AdminShell>
{:else if $auth.status === 'error'}
	<div class="admin-access-error"><section class="admin-access-card"><h2>Chưa kết nối được workspace.</h2><p>{$auth.errorMessage}</p><button class="admin-primary-button" type="button" onclick={() => void auth.bootstrap()}>Thử lại</button></section></div>
{:else}
	<div class="admin-loading"><div class="admin-loading-card"><p>Đang chuyển tới trang đăng nhập…</p></div></div>
{/if}
