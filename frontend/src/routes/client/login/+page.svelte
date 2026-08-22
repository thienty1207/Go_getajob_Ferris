<script lang="ts">
	import { onMount } from 'svelte';
	import { clientAuth } from '$lib/client/stores/client-auth-store';
	import { googleLoginUrl } from '$lib/client/api/client-auth-api';
	import ClientHeader from '$lib/client/components/ClientHeader.svelte';

	let error = $state('');
	let starting = $state(false);

	$effect(() => {
		if ($clientAuth.status === 'authenticated') {
			window.location.href = '/client';
		}
	});

	onMount(async () => {
		const params = new URLSearchParams(window.location.search);
		const oauthError = params.get('error');
		if (oauthError) {
			error = messageForOAuthError(oauthError);
		}
		if ($clientAuth.status === 'unknown') {
			await clientAuth.loadCurrentUser();
		}
	});

	function messageForOAuthError(code: string): string {
		const messages: Record<string, string> = {
			state_error: 'Phiên đăng nhập không hợp lệ hoặc đã hết hạn. Vui lòng thử lại.',
			token_exchange_error: 'Google không thể hoàn tất đăng nhập (exchange lỗi). Vui lòng thử lại.',
			id_token_error: 'Không xác thực được danh tính Google. Vui lòng thử lại.',
			email_not_verified: 'Tài khoản Google email chưa xác thực.'
		};
		return messages[code] ?? 'Đăng nhập Google không thành công. Vui lòng thử lại.';
	}

	function startGoogle() {
		if (starting || $clientAuth.status === 'loading' || $clientAuth.status === 'authenticated') return;
		starting = true;
		window.location.href = googleLoginUrl();
	}
</script>

<svelte:head>
	<title>Đăng nhập · Sugoi-oniichan</title>
</svelte:head>

<ClientHeader hideLoginAction />

<main class="login-page">
	<section class="login-card" aria-labelledby="login-title">
		<h1 id="login-title">Đăng nhập</h1>

		{#if error}
			<p class="login-error" role="alert">{error}</p>
		{/if}

		<button
			type="button"
			class="google-button"
			onclick={startGoogle}
			disabled={starting || $clientAuth.status === 'loading' || $clientAuth.status === 'authenticated'}
		>
			{#if starting || $clientAuth.status === 'loading'}
				<span class="spinner" aria-hidden="true"></span> Đang chuyển…
			{:else}
				<svg class="google-icon" viewBox="0 0 48 48" aria-hidden="true"><path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/><path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/><path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/><path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/></svg>
				Tiếp tục với Google
			{/if}
		</button>

		<a class="back-link" href="/client">Quay lại trang client</a>
	</section>
</main>
