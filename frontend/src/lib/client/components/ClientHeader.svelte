<script lang="ts">
import { onMount, onDestroy } from 'svelte';
import { clientAuth } from '../stores/client-auth-store';
import type { ClientUser } from '../../shared/types/client-auth';

interface Props {
	hideLoginAction?: boolean;
}

let { hideLoginAction = false }: Props = $props();
let menuOpen = $state(false);
let mounted = $state(false);

	$effect(() => {
		if ($clientAuth.status === 'unknown' && mounted) {
			clientAuth.loadCurrentUser();
		}
	});

	onMount(() => {
		mounted = true;
		if ($clientAuth.status === 'unknown') {
			clientAuth.loadCurrentUser();
		}
		function onKeydown(event: KeyboardEvent) {
			if (event.key === 'Escape') menuOpen = false;
		}
		function onPointerDown(event: PointerEvent) {
			if (menuOpen && event.target instanceof Node && menuRef && !menuRef.contains(event.target)) {
				menuOpen = false;
			}
		}
		window.addEventListener('keydown', onKeydown);
		window.addEventListener('pointerdown', onPointerDown);
		return () => {
			window.removeEventListener('keydown', onKeydown);
			window.removeEventListener('pointerdown', onPointerDown);
		};
	});

	let menuRef: HTMLElement | undefined = $state();

	function toggleMenu() {
		if ($clientAuth.status === 'authenticated') menuOpen = !menuOpen;
	}

	function logout() {
		void clientAuth.logout().then(() => {
			menuOpen = false;
			window.location.href = '/client';
		});
	}

	function initial(user: ClientUser): string {
		const source = user.displayName?.trim() || user.email?.trim() || 'U';
		return source.charAt(0).toUpperCase();
	}
</script>

<header class="site-header">
	<a class="brand" href="/client" aria-label="Sugoi-oniichan — Go get a job ferris">
		<img class="brand-logo" src="/brand/sugoi-oniichan-logo.png" alt="Sugoi-oniichan" />
		<span class="brand-copy"><strong><span class="brand-sugoi">Sugoi</span><span class="brand-oniichan">-oniichan</span></strong><small>Go get a job ferris</small></span>
	</a>
	<div class="header-nav">
		{#if $clientAuth.status === 'authenticated' && $clientAuth.user}
			<div class="avatar-wrap" bind:this={menuRef}>
				<button
					type="button"
					class="avatar-button"
					aria-label="Mở menu tài khoản"
					aria-haspopup="menu"
					aria-expanded={menuOpen}
					onclick={toggleMenu}
				>
					{#if $clientAuth.user.avatarUrl}
						<img class="avatar-img" src={$clientAuth.user.avatarUrl} alt="Ảnh đại diện tài khoản" referrerpolicy="no-referrer" />
					{:else}
						<span class="avatar-fallback" aria-hidden="true">{initial($clientAuth.user)}</span>
					{/if}
				</button>
				{#if menuOpen}
					<div class="avatar-menu" role="menu" aria-orientation="vertical">
						<a class="avatar-menu-item" role="menuitem" href="/client/profile" onclick={() => (menuOpen = false)}>Profile</a>
						<a class="avatar-menu-item" role="menuitem" href="/client/cv" onclick={() => (menuOpen = false)}>CV của tôi</a>
						<button type="button" class="avatar-menu-item avatar-menu-logout" role="menuitem" onclick={logout}>Đăng xuất</button>
					</div>
				{/if}
			</div>
		{:else if !hideLoginAction && ($clientAuth.status === 'unauthenticated' || $clientAuth.status === 'error')}
			<a class="login-link" href="/client/login">Đăng nhập Google</a>
		{/if}
	</div>
</header>
