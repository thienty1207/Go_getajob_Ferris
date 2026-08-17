<script lang="ts">
	import { getContext, onMount } from 'svelte';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { getAdminSettings, updateAdminCrawlerSettings, type AdminSettings } from '$lib/admin/api/admin-api';
	import { ADMIN_AUTH_CONTEXT } from '$lib/admin/api/admin-session';
	import type { AdminAuthStore } from '$lib/admin/stores/admin-auth-store';
	import { countdownSeconds, formatCountdown } from './crawler-runtime';

	const auth = getContext<AdminAuthStore>(ADMIN_AUTH_CONTEXT);
	let settings = $state<AdminSettings | null>(null);
	let intervalHours = $state(0);
	let intervalMinutes = $state(0);
	let now = $state(Date.now());
	let loading = $state(true);
	let saving = $state(false);
	let errorMessage = $state('');
	let successMessage = $state('');
	let refreshTimer: ReturnType<typeof setInterval> | undefined;
	let clockTimer: ReturnType<typeof setInterval> | undefined;
	let remainingSeconds = $derived(settings ? countdownSeconds(settings.runtime.nextCycleAt, now) : null);
	let runtimeStatus = $derived(settings?.runtime.status ?? 'OFFLINE');

	onMount(() => {
		void loadSettings();
		refreshTimer = setInterval(() => void loadSettings(false), 15_000);
		clockTimer = setInterval(() => { now = Date.now(); }, 1_000);
		return () => {
			if (refreshTimer) clearInterval(refreshTimer);
			if (clockTimer) clearInterval(clockTimer);
		};
	});

	async function loadSettings(showLoading = true) {
		if (showLoading) loading = true;
		errorMessage = '';
		try {
			const result = await getAdminSettings();
			settings = result;
			intervalHours = result.crawler.intervalHours;
			intervalMinutes = result.crawler.intervalMinutes;
		} catch (error) {
			handleError(error, 'Không thể đọc Settings.');
		} finally {
			if (showLoading) loading = false;
		}
	}

	async function saveSettings() {
		if (!$auth.csrfToken || saving || !settings) return;
		saving = true;
		errorMessage = '';
		successMessage = '';
		try {
			settings = await updateAdminCrawlerSettings(intervalHours, intervalMinutes, $auth.csrfToken);
			intervalHours = settings.crawler.intervalHours;
			intervalMinutes = settings.crawler.intervalMinutes;
			successMessage = 'Đã lưu thời gian crawl.';
		} catch (error) {
			handleError(error, 'Không thể lưu Settings.');
		} finally {
			saving = false;
		}
	}

	function formatDate(value: string | undefined) {
		if (!value) return '—';
		const date = new Date(value);
		return Number.isNaN(date.getTime()) ? '—' : new Intl.DateTimeFormat('vi-VN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
	}

	function handleError(error: unknown, fallback: string) {
		if (error instanceof ApiError && error.status === 401) auth.expire();
		errorMessage = error instanceof ApiError ? error.message : fallback;
	}
</script>

<svelte:head>
	<title>Settings — Admin Go get a job ferris</title>
</svelte:head>

<section class="admin-page-heading">
	<div><h2>Settings.</h2></div>
</section>

{#if errorMessage}<div class="admin-status error" role="alert"><span class="admin-status-symbol">!</span><span>{errorMessage}</span><button class="admin-secondary-button" type="button" onclick={() => void loadSettings()}>Thử lại</button></div>{/if}
{#if successMessage}<div class="admin-status success" role="status"><span class="admin-status-symbol">✓</span><span>{successMessage}</span></div>{/if}

{#if loading}
	<section class="admin-panel admin-empty-state"><span class="admin-loading-orbit" aria-hidden="true"></span><strong>Đang đọc Settings</strong></section>
{:else if settings}
	<section class="admin-panel admin-settings-panel">
		<div class="admin-panel-header"><h3>Crawler</h3><span class={`runtime-status ${runtimeStatus.toLowerCase()}`}>{runtimeStatus}</span></div>
		<form class="admin-form-grid" onsubmit={(event) => { event.preventDefault(); void saveSettings(); }}>
			<div class="admin-field"><label for="crawler-hours">Giờ</label><input id="crawler-hours" bind:value={intervalHours} type="number" min="0" max={Math.floor(settings.crawler.maxIntervalMinutes / 60)} step="1" required /></div>
			<div class="admin-field"><label for="crawler-minutes">Phút</label><input id="crawler-minutes" bind:value={intervalMinutes} type="number" min="0" max="59" step="1" required /></div>
			<div class="admin-form-actions wide"><button class="admin-primary-button" type="submit" disabled={saving}>{saving ? 'Đang lưu…' : 'Lưu Settings'}</button><span class="admin-settings-current">Hiện tại: {settings.crawler.intervalHours} giờ {settings.crawler.intervalMinutes} phút</span></div>
		</form>
	</section>

	<section class="admin-panel admin-runtime-panel">
		<div class="admin-panel-header"><h3>Crawler runtime</h3><button class="admin-secondary-button" type="button" onclick={() => void loadSettings(false)}>Làm mới</button></div>
		<div class="admin-runtime-grid">
			<div><span>Trạng thái</span><strong class={`runtime-value ${runtimeStatus.toLowerCase()}`}>{runtimeStatus}</strong></div>
			<div><span>Chu kỳ kế tiếp</span><strong>{runtimeStatus === 'RUNNING' ? 'Đang crawl' : remainingSeconds === null ? '—' : remainingSeconds === 0 ? 'Đến hạn' : formatCountdown(remainingSeconds)}</strong></div>
			<div><span>Heartbeat</span><strong>{formatDate(settings.runtime.lastHeartbeatAt)}</strong></div>
			<div><span>Chu kỳ gần nhất</span><strong>{formatDate(settings.runtime.lastCycleFinishedAt)}</strong></div>
		</div>
		{#if settings.runtime.currentSourceKey || settings.runtime.lastErrorCode}<div class="admin-runtime-meta">{#if settings.runtime.currentSourceKey}<span>Source: {settings.runtime.currentSourceKey}</span>{/if}{#if settings.runtime.lastErrorCode}<span>Error: {settings.runtime.lastErrorCode}</span>{/if}</div>{/if}
	</section>
{/if}
