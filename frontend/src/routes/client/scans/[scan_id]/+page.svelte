<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { getScanStatus } from '$lib/client/api/client-api';
	import ClientEmptyState from '$lib/client/components/ClientEmptyState.svelte';
	import ClientErrorState from '$lib/client/components/ClientErrorState.svelte';
	import ClientHeader from '$lib/client/components/ClientHeader.svelte';
	import MatchResults from '$lib/client/components/MatchResults.svelte';
	import ScanProgress from '$lib/client/components/ScanProgress.svelte';
	import { clientAuth } from '$lib/client/stores/client-auth-store';
	import type { ScanStatusResponse } from '$lib/shared/types/client';

	const POLL_INTERVAL_MS = 1200;
	const MAX_POLL_ATTEMPTS = 75;

	let result = $state<ScanStatusResponse | null>(null);
	let errorMessage = $state('');
	let timedOut = $state(false);
	let isLoading = $state(true);
	let destroyed = false;
	let pollTimer: ReturnType<typeof setTimeout> | undefined;

	const scanId = $derived(page.params.scan_id ?? '');

	onMount(() => {
		void bootstrap();
		return () => {
			destroyed = true;
			if (pollTimer) clearTimeout(pollTimer);
		};
	});

	async function bootstrap() {
		if (!scanId) {
			fail('Thiếu mã lần quét. Vui lòng bắt đầu lại từ trang client.');
			return;
		}

		if ($clientAuth.status !== 'authenticated') {
			await clientAuth.loadCurrentUser();
		}
		if (destroyed) return;
		if ($clientAuth.status !== 'authenticated') {
			window.location.href = `/client/login?return_to=${encodeURIComponent(`/client/scans/${scanId}`)}`;
			return;
		}

		await pollScan();
	}

	async function pollScan() {
		isLoading = true;
		errorMessage = '';
		timedOut = false;

		for (let attempt = 0; attempt < MAX_POLL_ATTEMPTS && !destroyed; attempt += 1) {
			try {
				const next = await getScanStatus(scanId);
				if (destroyed) return;
				result = next;
				if (next.status !== 'processing') {
					isLoading = false;
					return;
				}
			} catch (error) {
				fail(getErrorMessage(error));
				return;
			}

			await wait(POLL_INTERVAL_MS);
		}

		if (!destroyed) {
			timedOut = true;
			isLoading = false;
			errorMessage = 'Matching service chưa phản hồi kết quả trong thời gian dự kiến. Bạn có thể thử tải lại.';
		}
	}

	function wait(milliseconds: number) {
		return new Promise<void>((resolve) => {
			pollTimer = setTimeout(resolve, milliseconds);
		});
	}

	function fail(message: string) {
		isLoading = false;
		errorMessage = message;
	}

	function getErrorMessage(error: unknown) {
		if (error instanceof ApiError && error.status === 401) {
			return 'Phiên đăng nhập không còn hợp lệ. Vui lòng đăng nhập lại.';
		}
		if (error instanceof ApiError) return error.message;
		return 'Không thể đọc kết quả quét CV. Vui lòng thử lại.';
	}

	function retry() {
		result = null;
		void pollScan();
	}
</script>

<svelte:head>
	<title>Kết quả quét CV · Sugoi-oniichan</title>
	<meta name="description" content="Kết quả matching job theo CV và Job Location đã chọn." />
</svelte:head>

<div class="client-page">
	<div class="ambient ambient-left" aria-hidden="true"></div>
	<div class="ambient ambient-right" aria-hidden="true"></div>
	<ClientHeader />

	<main class="page-container scan-result-page">
		<div class="scan-result-heading">
			<div>
				<span class="section-kicker">CV SCAN</span>
				<h1>Kết quả matching.</h1>
			</div>
			<a class="back-link scan-back-link" href="/client">Quét CV khác</a>
		</div>

		{#if errorMessage}
			<ClientErrorState message={errorMessage} onRetry={retry} />
		{:else if isLoading || result?.status === 'processing'}
			<ScanProgress status="polling" />
		{:else if result?.status === 'failed'}
			<ClientErrorState message={result.message} onRetry={retry} />
		{:else if result?.status === 'completed' && result.matches.length === 0}
			<ClientEmptyState onRetry={() => (window.location.href = '/client')} />
		{:else if result?.status === 'completed'}
			<MatchResults matches={result.matches} />
		{:else if timedOut}
			<ClientErrorState message={errorMessage} onRetry={retry} />
		{/if}
	</main>
</div>
