<script lang="ts">
	import { onMount } from 'svelte';
	import { clientAuth } from '$lib/client/stores/client-auth-store';
	import ClientHeader from '$lib/client/components/ClientHeader.svelte';

	let retrying = $state(false);

	$effect(() => {
		if ($clientAuth.status === 'unauthenticated') {
			window.location.href = '/client/login';
		}
	});

	onMount(() => {
		if ($clientAuth.status === 'unknown' || $clientAuth.status === 'error') {
			void clientAuth.loadCurrentUser();
		}
	});

	function retry() {
		retrying = true;
		void clientAuth.loadCurrentUser().then(() => {
			retrying = false;
		});
	}

	function initial(): string {
		const user = $clientAuth.user;
		const source = user?.displayName?.trim() || user?.email?.trim() || 'U';
		return source.charAt(0).toUpperCase();
	}

	function logout() {
		void clientAuth.logout().then(() => {
			window.location.href = '/client/login';
		});
	}
</script>

<svelte:head>
	<title>Profile · Go get a job ferris</title>
</svelte:head>

<ClientHeader />

<main class="profile-page">
	{#if $clientAuth.status === 'loading' || ($clientAuth.status === 'unknown' && !retrying)}
		<div class="profile-card">
			<p class="profile-loading" role="status">Đang tải thông tin tài khoản…</p>
		</div>
	{:else if $clientAuth.status === 'error'}
		<div class="profile-card">
			<p class="profile-error">Không thể tải thông tin tài khoản. Vui lòng thử lại.</p>
			<button type="button" class="logout-button" onclick={retry} disabled={retrying}>
				{retrying ? 'Đang thử lại…' : 'Thử lại'}
			</button>
		</div>
	{:else if $clientAuth.status === 'authenticated' && $clientAuth.user}
		<section class="profile-card" aria-labelledby="profile-title">
			{#if $clientAuth.user.avatarUrl}
				<img class="profile-avatar" src={$clientAuth.user.avatarUrl} alt="Ảnh đại diện tài khoản" referrerpolicy="no-referrer" />
			{:else}
				<div class="profile-avatar-fallback" aria-hidden="true">{initial()}</div>
			{/if}
			<h1 id="profile-title" class="profile-name">{$clientAuth.user.displayName}</h1>
			<p class="profile-email">{$clientAuth.user.email}</p>
			<span class="profile-status"><i aria-hidden="true"></i> Đã kết nối Google</span>
			<div class="profile-actions">
				<button type="button" class="logout-button" onclick={logout}>Đăng xuất</button>
				<a class="back-link" href="/client">Quay lại trang client</a>
			</div>
		</section>
	{/if}
</main>
