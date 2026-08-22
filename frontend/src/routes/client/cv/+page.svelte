<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { deleteClientCV, getClientCVHistory } from '$lib/client/api/client-cv-api';
	import { clampCVHistoryPage } from '$lib/client/cv-history-pagination';
	import ClientHeader from '$lib/client/components/ClientHeader.svelte';
	import { clientAuth } from '$lib/client/stores/client-auth-store';
	import type { ClientCVHistoryItem } from '$lib/shared/types/client-cv';

	let items = $state<ClientCVHistoryItem[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let errorMessage = $state('');
	let deletingId = $state('');
	let confirmingId = $state('');
	let currentPage = $state(1);
	const pageSize = 10;

	onMount(() => {
		void bootstrap();
	});

	async function bootstrap() {
		if ($clientAuth.status !== 'authenticated') {
			await clientAuth.loadCurrentUser();
		}
		if ($clientAuth.status !== 'authenticated') {
			window.location.href = `/client/login?return_to=${encodeURIComponent(page.url.pathname)}`;
			return;
		}
		await loadHistory();
	}

	async function loadHistory() {
		loading = true;
		errorMessage = '';
		try {
			let response = await getClientCVHistory(currentPage, pageSize);
			const correctedPage = clampCVHistoryPage(currentPage, response.total, pageSize);
			if (correctedPage !== currentPage) {
				currentPage = correctedPage;
				response = await getClientCVHistory(currentPage, pageSize);
			}
			items = response.items;
			total = response.total;
		} catch (error) {
			errorMessage = error instanceof ApiError ? error.message : 'Không thể tải lịch sử CV.';
		} finally {
			loading = false;
		}
	}

	async function deleteHistoryItem(item: ClientCVHistoryItem) {
		if (!$clientAuth.csrfToken || deletingId) return;
		deletingId = item.scanId;
		errorMessage = '';
		try {
			await deleteClientCV(item.scanId, $clientAuth.csrfToken);
			confirmingId = '';
			await loadHistory();
		} catch (error) {
			errorMessage = error instanceof ApiError ? error.message : 'Không thể xóa CV lúc này.';
		} finally {
			deletingId = '';
		}
	}

	function statusLabel(status: ClientCVHistoryItem['status']) {
		return { received: 'Đã nhận', parsing: 'Đang đọc', matching: 'Đang match', completed: 'Hoàn tất', failed: 'Thất bại' }[status];
	}

	function formatDate(value: string) {
		return new Intl.DateTimeFormat('vi-VN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
	}

	function totalPages() {
		return Math.max(1, Math.ceil(total / pageSize));
	}

	function goToPage(nextPage: number) {
		if (nextPage < 1 || nextPage > totalPages() || nextPage === currentPage) return;
		currentPage = nextPage;
		void loadHistory();
	}
</script>

<svelte:head>
	<title>CV của tôi · Sugoi-oniichan</title>
	<meta name="description" content="Lịch sử CV đã submit và structured profile của tài khoản." />
</svelte:head>

<div class="client-page">
	<div class="ambient ambient-left" aria-hidden="true"></div>
	<div class="ambient ambient-right" aria-hidden="true"></div>
	<ClientHeader />

	<main class="page-container cv-history-page">
		<div class="cv-history-heading">
			<div><span class="section-kicker">MY CV</span><h1>CV của tôi.</h1><p>Chỉ structured profile được giữ lại để bạn xem lại; file CV gốc không nằm trong lịch sử.</p></div>
			<a class="primary-link" href="/client">Quét CV mới <span aria-hidden="true">→</span></a>
		</div>

		{#if errorMessage}
			<section class="state-card error-card" role="alert"><div class="state-symbol" aria-hidden="true">!</div><div><h2>Không thể tải lịch sử CV.</h2><p>{errorMessage}</p></div><button class="secondary-button" type="button" onclick={() => void loadHistory()}>Thử lại</button></section>
		{:else if loading}
			<section class="state-card" aria-live="polite"><div class="processing-orbit" aria-hidden="true"><span></span></div><div><h2>Đang tải lịch sử CV…</h2><p>Đang đọc dữ liệu thuộc tài khoản của bạn.</p></div></section>
		{:else if items.length === 0}
			<section class="state-card empty-card"><div class="state-symbol" aria-hidden="true">∅</div><div><h2>Chưa có CV nào.</h2><p>CV đầu tiên của bạn sẽ xuất hiện ở đây sau khi quét thành công.</p></div></section>
		{:else}
			<section class="cv-history-list" aria-label="Lịch sử CV">
				{#each items as item (item.scanId)}
					<article class="cv-history-card">
						<div class="cv-history-card-head"><div><span class="cv-status cv-status-{item.status}">{statusLabel(item.status)}</span><h2>{item.location}</h2><p>{formatDate(item.createdAt)}</p></div><div class="cv-history-actions"><span>{item.matchCount} job match</span>{#if confirmingId === item.scanId}<button class="danger-button" type="button" onclick={() => void deleteHistoryItem(item)} disabled={deletingId === item.scanId}>{deletingId === item.scanId ? 'Đang xóa…' : 'Xác nhận xóa'}</button><button class="secondary-button" type="button" onclick={() => (confirmingId = '')} disabled={Boolean(deletingId)}>Hủy</button>{:else}<button class="danger-button" type="button" onclick={() => (confirmingId = item.scanId)}>Xóa CV</button>{/if}</div></div>
						{#if item.profile}
							<div class="cv-profile-grid"><div><span>Role</span><strong>{item.profile.roles.join(', ') || 'Chưa xác định'}</strong></div><div><span>Kỹ năng</span><strong>{item.profile.skills.slice(0, 5).join(', ') || 'Chưa xác định'}</strong></div><div><span>Kinh nghiệm</span><strong>{item.profile.yearsOfExperience} năm · {item.profile.seniority}</strong></div><div><span>Domain</span><strong>{item.profile.domains.join(', ') || 'Chưa xác định'}</strong></div></div>
						{:else}
							<p class="cv-history-muted">Structured profile chưa sẵn sàng cho lần quét này.</p>
						{/if}
					</article>
				{/each}
			</section>
			<nav class="cv-history-pagination" aria-label="Phân trang lịch sử CV"><button class="secondary-button" type="button" onclick={() => goToPage(currentPage - 1)} disabled={currentPage === 1}>← Trước</button><span>Trang {currentPage} / {totalPages()}</span><button class="secondary-button" type="button" onclick={() => goToPage(currentPage + 1)} disabled={currentPage >= totalPages()}>Sau →</button></nav>
		{/if}
	</main>
</div>
