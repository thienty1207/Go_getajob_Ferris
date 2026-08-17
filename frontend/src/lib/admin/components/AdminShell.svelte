<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { getContext, onMount } from 'svelte';
	import { ADMIN_AUTH_CONTEXT } from '../api/admin-session';
	import type { AdminAuthStore } from '../stores/admin-auth-store';

	interface Props {
		children: import('svelte').Snippet;
	}

	let { children }: Props = $props();
	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let sidebarOpen = $state(true);
	let isMobileViewport = $state(false);
	let activePath = $derived(page.url.pathname);
	let userEmail = $derived($auth.user?.email ?? 'Admin');
	let manageJobOpen = $state(false);
	let manageUserOpen = $state(false);
	let isManageJobPath = $derived(activePath.startsWith('/admin/sources') || activePath.startsWith('/admin/locations') || activePath.startsWith('/admin/jobs'));
	let isManageUserPath = $derived(activePath.startsWith('/admin/users') || activePath.startsWith('/admin/cvs'));

	$effect(() => {
		if (isManageJobPath) manageJobOpen = true;
		if (isManageUserPath) manageUserOpen = true;
	});

	onMount(() => {
		const mobileQuery = window.matchMedia('(max-width: 760px)');
		isMobileViewport = mobileQuery.matches;
		sidebarOpen = !mobileQuery.matches;

		const handleViewportChange = (event: MediaQueryListEvent) => {
			isMobileViewport = event.matches;
			sidebarOpen = !event.matches;
		};
		mobileQuery.addEventListener('change', handleViewportChange);
		return () => mobileQuery.removeEventListener('change', handleViewportChange);
	});

	async function handleLogout() {
		closeSidebar();
		await auth.logout();
		await goto('/admin/login');
	}

	function toggleSidebar() {
		sidebarOpen = !sidebarOpen;
	}

	function closeSidebar() {
		sidebarOpen = false;
	}

	function handleNavigation() {
		if (isMobileViewport) closeSidebar();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') closeSidebar();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class:sidebar-open={sidebarOpen} class="admin-root">
	<aside id="admin-navigation" class="admin-sidebar" aria-label="Điều hướng admin">
		<a class="admin-brand" href="/admin" onclick={handleNavigation} aria-label="Go get a job ferris admin dashboard">
			<img class="admin-brand-logo" src="/brand/sugoi-oniichan-logo.png" alt="Sugoi-oniichan" />
			<span class="admin-brand-copy"><strong>Go get a job ferris</strong></span>
		</a>

		<nav class="admin-nav">
			<a class:active={activePath === '/admin'} href="/admin" onclick={handleNavigation}><span class="admin-nav-icon">⌂</span><span>Overview</span></a>
			<a class:active={activePath.startsWith('/admin/promotions')} href="/admin/promotions" onclick={handleNavigation}><span class="admin-nav-icon">✦</span><span>Promotions</span></a>
			<button class:active={isManageJobPath} class="admin-nav-group-toggle" type="button" aria-expanded={manageJobOpen} onclick={() => { manageJobOpen = !manageJobOpen; }}><span class="admin-nav-icon">◈</span><span>Manage Job</span><span class="admin-nav-group-caret" aria-hidden="true">{manageJobOpen ? '−' : '+'}</span></button>
			{#if manageJobOpen}<div class="admin-nav-submenu">
				<a class:active={activePath.startsWith('/admin/sources')} href="/admin/sources" onclick={handleNavigation}><span class="admin-nav-icon">↗</span><span>Job Link</span></a>
				<a class:active={activePath.startsWith('/admin/locations')} href="/admin/locations" onclick={handleNavigation}><span class="admin-nav-icon">⌖</span><span>Job Location</span></a>
				<a class:active={activePath.startsWith('/admin/jobs')} href="/admin/jobs" onclick={handleNavigation}><span class="admin-nav-icon">⌕</span><span>Job Cache</span></a>
			</div>{/if}
			<button class:active={isManageUserPath} class="admin-nav-group-toggle" type="button" aria-expanded={manageUserOpen} onclick={() => { manageUserOpen = !manageUserOpen; }}><span class="admin-nav-icon">◎</span><span>Manage User</span><span class="admin-nav-group-caret" aria-hidden="true">{manageUserOpen ? '−' : '+'}</span></button>
			{#if manageUserOpen}<div class="admin-nav-submenu">
				<a class:active={activePath.startsWith('/admin/users')} href="/admin/users" onclick={handleNavigation}><span class="admin-nav-icon">◎</span><span>Users</span></a>
				<a class:active={activePath.startsWith('/admin/cvs')} href="/admin/cvs" onclick={handleNavigation}><span class="admin-nav-icon">▤</span><span>CV Profiles</span></a>
			</div>{/if}
			<a class:active={activePath.startsWith('/admin/settings')} href="/admin/settings" onclick={handleNavigation}><span class="admin-nav-icon">⚙</span><span>Settings</span></a>
		</nav>

		<div class="admin-sidebar-account" aria-label="Tài khoản admin">
			<div class="admin-sidebar-account-user">
				<span class="admin-user-badge" aria-hidden="true">{userEmail.slice(0, 1).toUpperCase()}</span>
				<span class="admin-user-email" title={userEmail}>{userEmail}</span>
			</div>
			<button class="admin-logout admin-sidebar-logout" type="button" onclick={handleLogout}>Đăng xuất</button>
		</div>
	</aside>
	<button class="admin-sidebar-toggle" type="button" aria-controls="admin-navigation" aria-expanded={sidebarOpen} aria-label={sidebarOpen ? 'Đóng sidebar' : isMobileViewport ? 'Mở menu' : 'Mở sidebar'} title={sidebarOpen ? 'Đóng sidebar' : isMobileViewport ? 'Mở menu' : 'Mở sidebar'} onclick={toggleSidebar}>
		<span aria-hidden="true">{#if !sidebarOpen && isMobileViewport}☰{:else if sidebarOpen}←{:else}→{/if}</span>
		<span class="visually-hidden">{sidebarOpen ? 'Đóng sidebar' : isMobileViewport ? 'Mở menu' : 'Mở sidebar'}</span>
	</button>
	{#if sidebarOpen}<button class="admin-sidebar-backdrop" type="button" aria-label="Đóng sidebar" onclick={closeSidebar}></button>{/if}

	<div class="admin-main">
		<main class="admin-content">{@render children()}</main>
	</div>
</div>
