<script lang="ts">
	import { onMount, getContext } from 'svelte';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { createAdminJobLink, deleteAdminJobLink, getAdminJobLinks, requestAdminJobLinkCrawl, setAdminJobLinkStatus, updateAdminJobLink, type AdminJobLink } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import { reconcileCrawlFeedback, type CrawlFeedback } from './crawl-feedback';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let allowedLink = $state('');
	let links = $state<AdminJobLink[]>([]);
	let loading = $state(true);
	let saving = $state(false);
	let currentPage = $state(1);
	const pageSize = 10;
	let total = $state(0);
	let pageCount = $derived(Math.max(1, Math.ceil(total / pageSize)));
	let errorMessage = $state('');
	let successMessage = $state('');
	let editingId = $state<string | null>(null);
	let isEditing = $derived(editingId !== null);
	let crawlPollTimer: ReturnType<typeof setTimeout> | undefined;
	let crawlFeedback = $state<Record<string, CrawlFeedback>>({});

	onMount(() => {
		void loadLinks();
		return () => {
			if (crawlPollTimer) clearTimeout(crawlPollTimer);
		};
	});

	async function loadLinks(showLoading = true) {
		if (showLoading) loading = true;
		errorMessage = '';
		try {
			const result = await getAdminJobLinks(currentPage, pageSize);
			if (result.items.length === 0 && result.total > 0 && currentPage > 1) {
				currentPage = Math.min(currentPage, Math.ceil(result.total / pageSize));
				const adjusted = await getAdminJobLinks(currentPage, pageSize);
				links = adjusted.items;
				total = adjusted.total;
			} else {
				links = result.items;
				total = result.total;
			}
			syncCrawlFeedback(links);
		} catch (error) {
			handleError(error, 'Không thể đọc danh sách Job Link từ API.');
		} finally {
			if (showLoading) loading = false;
			scheduleCrawlPoll();
		}
	}

	function scheduleCrawlPoll() {
		if (crawlPollTimer || !links.some((link) => isCrawlInProgress(link))) return;
		crawlPollTimer = setTimeout(() => {
			crawlPollTimer = undefined;
			void loadLinks(false);
		}, 3000);
	}

	function isCrawlInProgress(link: AdminJobLink) {
		const feedback = crawlFeedback[link.id];
		return Boolean(link.activeCrawlRequestStatus) || Boolean(feedback && feedback.phase !== 'RESULT');
	}

	function syncCrawlFeedback(nextLinks: AdminJobLink[]) {
		for (const link of nextLinks) {
			const feedback = crawlFeedback[link.id];
			if (!feedback || feedback.phase === 'RESULT') continue;
			const next = reconcileCrawlFeedback(feedback, link);
			if (next.phase === 'RESULT') {
				successMessage = `Crawl hoàn tất: ${next.status ?? 'COMPLETED'} · ${next.pages ?? 0} trang · ${next.jobs ?? 0} jobs${next.errorCode ? ` · ${next.errorCode}` : ''}.`;
			}
			crawlFeedback[link.id] = next;
		}
	}

	async function saveLink() {
		const url = allowedLink.trim();
		if (!url || saving || !$auth.csrfToken) return;
		saving = true;
		errorMessage = '';
		successMessage = '';
		try {
			if (editingId) {
				await updateAdminJobLink(editingId, url, $auth.csrfToken);
				successMessage = 'Đã cập nhật Job Link.';
			} else {
				await createAdminJobLink(url, $auth.csrfToken);
				successMessage = 'Đã thêm Job Link.';
			}
			allowedLink = '';
			editingId = null;
			currentPage = 1;
			await loadLinks();
		} catch (error) {
			handleError(error, 'Không thể lưu Job Link.');
		} finally {
			saving = false;
		}
	}

	function startEditing(link: AdminJobLink) {
		editingId = link.id;
		allowedLink = link.url;
		errorMessage = '';
		successMessage = '';
	}

	function cancelEditing() {
		editingId = null;
		allowedLink = '';
	}

	async function removeLink(link: AdminJobLink) {
		if (!$auth.csrfToken || saving || !window.confirm(`Xóa vĩnh viễn Job Link ${link.url}? Dữ liệu crawl và kết quả match thuộc link này cũng sẽ bị xóa.`)) return;
		saving = true;
		errorMessage = '';
		successMessage = '';
		try {
			await deleteAdminJobLink(link.id, $auth.csrfToken);
			if (editingId === link.id) cancelEditing();
			successMessage = 'Đã xóa Job Link và dữ liệu crawl liên quan.';
			await loadLinks();
		} catch (error) {
			handleError(error, 'Không thể xóa Job Link.');
		} finally {
			saving = false;
		}
	}

	async function changeStatus(link: AdminJobLink) {
		if (!$auth.csrfToken || saving) return;
		const nextStatus = link.approvalStatus === 'ACTIVE' ? 'DISABLED' : 'ACTIVE';
		const action = nextStatus === 'ACTIVE' ? 'khởi động lại' : 'dừng';
		if (!window.confirm(`Bạn muốn ${action} Job Link ${link.url}?`)) return;
		saving = true;
		errorMessage = '';
		successMessage = '';
		try {
			await setAdminJobLinkStatus(link.id, nextStatus, $auth.csrfToken);
			successMessage = nextStatus === 'ACTIVE' ? 'Đã khởi động lại Job Link.' : 'Đã dừng Job Link.';
			await loadLinks();
		} catch (error) {
			handleError(error, 'Không thể thay đổi trạng thái Job Link.');
		} finally {
			saving = false;
		}
	}

	async function crawlNow(link: AdminJobLink) {
		if (!$auth.csrfToken || saving || link.approvalStatus !== 'ACTIVE' || link.activeCrawlRequestStatus) return;
		crawlFeedback[link.id] = { phase: 'REQUESTING', requestedAt: new Date().toISOString() };
		saving = true;
		errorMessage = '';
		successMessage = '';
		try {
			const request = await requestAdminJobLinkCrawl(link.id, $auth.csrfToken);
			crawlFeedback[link.id] = {
				phase: request.status === 'RUNNING' ? 'RUNNING' : 'PENDING',
				requestedAt: request.requestedAt,
				requestId: request.id,
				status: request.status
			};
			successMessage = request.status === 'PENDING' ? 'Đã xếp Job Link vào hàng đợi crawl.' : 'Job Link đang được xử lý.';
			await loadLinks(false);
		} catch (error) {
			delete crawlFeedback[link.id];
			handleError(error, 'Không thể tạo yêu cầu crawl.');
		} finally {
			saving = false;
		}
	}

	function handleError(error: unknown, fallback: string) {
		if (error instanceof ApiError && error.status === 401) auth.expire();
		errorMessage = error instanceof ApiError ? error.message : fallback;
	}

	async function goToPage(nextPage: number) {
		if (loading || nextPage < 1 || nextPage > pageCount || nextPage === currentPage) return;
		currentPage = nextPage;
		await loadLinks();
	}
</script>

<svelte:head>
	<title>Job Link — Admin Go get a job ferris</title>
	<meta name="description" content="Quản lý link được phép crawl của Go get a job ferris." />
</svelte:head>

<section class="admin-page-heading">
	<div><h2>Job Link.</h2></div>
</section>

{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadLinks()}>Thử lại</button></div>{/if}
{#if successMessage}<div class="admin-status success" role="status"><span class="admin-status-symbol">✓</span><span>{successMessage}</span></div>{/if}

<section class="admin-panel">
	<form class="job-link-form" onsubmit={(event) => { event.preventDefault(); void saveLink(); }}>
		<div class="admin-field">
			<label class="visually-hidden" for="allowed-link">Link được phép crawl</label>
			<input id="allowed-link" bind:value={allowedLink} type="url" placeholder="https://example.com/careers" autocomplete="url" required />
		</div>
		<div class="job-link-form-actions">
			{#if isEditing}<button class="admin-secondary-button" type="button" onclick={cancelEditing} disabled={saving}>Hủy</button>{/if}
			<button class="admin-primary-button" type="submit" disabled={saving || !allowedLink.trim()}>{saving ? 'Đang lưu…' : isEditing ? 'Lưu link' : 'Thêm link'}</button>
		</div>
	</form>
</section>

<section class="admin-panel admin-link-list-panel">
	<div class="admin-panel-header">
		<h3>Danh sách link</h3>
	</div>
	<div class="job-link-list-header" aria-hidden="true"><span>Link</span><span>Thao tác</span></div>
	{#if loading}
		<div class="admin-empty-state"><span class="admin-loading-orbit" aria-hidden="true"></span><strong>Đang đọc Job Link</strong></div>
	{:else if links.length === 0}
		<div class="admin-empty-state">
			<span class="admin-empty-state-mark" aria-hidden="true">↗</span>
			<strong>Chưa có link nào</strong>
		</div>
	{:else}
		<div class="job-link-list">
			{#each links as link (link.id)}
				{@const feedback = crawlFeedback[link.id]}
				{@const crawlBusy = isCrawlInProgress(link)}
				<div class="job-link-row">
					<div class="job-link-row-copy">
						<a href={link.url} target="_blank" rel="noreferrer">{link.url}</a>
						<span>{link.displayName} · <span class:active={link.approvalStatus === 'ACTIVE'} class:disabled={link.approvalStatus === 'DISABLED'} class="job-badge">{link.approvalStatus}</span>
							{#if feedback && feedback.phase !== 'RESULT'}<span class="job-link-crawl-status pending" role="status"><span class="admin-button-spinner" aria-hidden="true"></span>{feedback.phase === 'REQUESTING' ? 'ĐANG XẾP…' : `CRAWL ${feedback.status ?? link.activeCrawlRequestStatus ?? 'PENDING'}`}</span>
							{:else if link.activeCrawlRequestStatus}<span class="job-link-crawl-status pending" role="status"><span class="admin-button-spinner" aria-hidden="true"></span>CRAWL {link.activeCrawlRequestStatus}</span>
							{:else if link.lastCrawlStatus}<span class:anomaly={link.lastCrawlStatus !== 'HEALTHY'} class:healthy={link.lastCrawlStatus === 'HEALTHY'} class="job-link-crawl-status" role="status">{link.lastCrawlStatus} · {link.lastCrawlPages ?? 0} trang · {link.lastCrawlJobs ?? 0} jobs{#if link.lastCrawlErrorCode} · {link.lastCrawlErrorCode}{/if}</span>{/if}
						</span>
					</div>
					<div class="job-link-actions">
						<button class="admin-primary-button" type="button" onclick={() => void crawlNow(link)} disabled={saving || link.approvalStatus !== 'ACTIVE' || crawlBusy}>
							{#if crawlBusy}<span class="admin-button-loading"><span class="admin-button-spinner" aria-hidden="true"></span>{feedback?.phase === 'REQUESTING' ? 'Đang xếp…' : 'Đang crawl…'}</span>{:else}Crawl ngay{/if}
						</button>
						<button class="admin-secondary-button" type="button" onclick={() => startEditing(link)} disabled={saving}>Sửa</button>
						<button class="admin-secondary-button" type="button" onclick={() => void changeStatus(link)} disabled={saving}>{link.approvalStatus === 'ACTIVE' ? 'Dừng' : 'Khởi động'}</button>
						<button class="admin-danger-button" type="button" onclick={() => void removeLink(link)} disabled={saving}>Xóa</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
	{#if !loading && total > pageSize}
		<div class="job-link-pagination" aria-label="Phân trang Job Link">
			<button class="admin-secondary-button" type="button" onclick={() => void goToPage(currentPage - 1)} disabled={currentPage <= 1}>Trước</button>
			<span>Trang {currentPage} / {pageCount}</span>
			<button class="admin-secondary-button" type="button" onclick={() => void goToPage(currentPage + 1)} disabled={currentPage >= pageCount}>Sau</button>
		</div>
	{/if}
</section>
