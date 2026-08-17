<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext } from 'svelte';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let email = $state('');
	let password = $state('');
	let submitting = $state(false);
	let errorMessage = $state('');

	async function submitLogin(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = '';
		if (!email.trim() || password.length < 12) {
			errorMessage = 'Vui lòng nhập email và mật khẩu hợp lệ.';
			return;
		}
		submitting = true;
		try {
			await auth.login(email.trim(), password);
			await goto('/admin');
		} catch (error) {
			errorMessage = error instanceof ApiError ? error.message : 'Không thể đăng nhập admin lúc này.';
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Admin login — Go get a job ferris</title>
	<meta name="description" content="Đăng nhập khu vực quản trị Go get a job ferris." />
</svelte:head>

<main class="admin-login-page">
	<section class="admin-login-grid" aria-label="Đăng nhập admin">
		<div class="admin-login-hero">
			<div><a class="admin-brand" href="/client" aria-label="Quay lại Go get a job ferris"><img class="admin-brand-logo" src="/brand/sugoi-oniichan-logo.png" alt="Sugoi-oniichan" /><span class="admin-brand-copy"><strong>Go get a job ferris</strong><small>Admin control room</small></span></a>
				<h1>Giữ cho nguồn việc <em>sạch</em> và đang hoạt động.</h1>
			</div>
		</div>

		<div class="admin-login-form-panel">
			<form class="admin-login-form" onsubmit={submitLogin} novalidate>
				<h2>Đăng nhập</h2>
				{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span></div>{/if}
				<div class="admin-login-fields" class:error-spacing={errorMessage}>
					<div class="admin-login-field"><label for="admin-email">Email</label><input id="admin-email" type="email" autocomplete="username" bind:value={email} placeholder="admin@example.com" disabled={submitting} /></div>
					<div class="admin-login-field"><label for="admin-password">Mật khẩu</label><input id="admin-password" type="password" autocomplete="current-password" bind:value={password} placeholder="••••••••••••" disabled={submitting} /></div>
				</div>
				<button class="admin-login-submit" type="submit" disabled={submitting}>{submitting ? 'Đang xác thực…' : 'Vào control room →'}</button>
			</form>
		</div>
	</section>
</main>
