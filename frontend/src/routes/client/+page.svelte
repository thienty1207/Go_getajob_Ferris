<script lang="ts">
	import { onMount } from 'svelte';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { getClientLocations, getPromotions, getScanStatus, startScan } from '$lib/client/api/client-api';
	import ClientEmptyState from '$lib/client/components/ClientEmptyState.svelte';
	import ClientErrorState from '$lib/client/components/ClientErrorState.svelte';
	import ClientHeader from '$lib/client/components/ClientHeader.svelte';
	import CvUploadForm from '$lib/client/components/CvUploadForm.svelte';
	import MatchResults from '$lib/client/components/MatchResults.svelte';
	import PromotionCarousel from '$lib/client/components/PromotionCarousel.svelte';
	import ScanProgress from '$lib/client/components/ScanProgress.svelte';
	import { createScanStore } from '$lib/client/stores/scan-store';
	import type { ClientLocation, PromotionSlide } from '$lib/shared/types/client';
	import { validateCvFile } from '$lib/client/validation/cv-file';
	import { isScanFormValid, validateScanForm } from '$lib/client/validation/scan-form';

	const POLL_INTERVAL_MS = 1200;
	const POLL_TIMEOUT_MS = 90_000;

	const scanStore = createScanStore();
	let promotions = $state<PromotionSlide[]>([]);
	let locations = $state<ClientLocation[]>([]);
	let locationsLoading = $state(true);
	let locationServiceError = $state('');

	onMount(async () => {
		const [promotionResult, locationResult] = await Promise.allSettled([getPromotions(), getClientLocations()]);
		promotions = promotionResult.status === 'fulfilled' ? promotionResult.value : [];
		if (locationResult.status === 'fulfilled') {
			locations = locationResult.value;
		} else {
			locationServiceError = getErrorMessage(locationResult.reason);
		}
		locationsLoading = false;
	});

	let isBusy = $derived($scanStore.status === 'submitting' || $scanStore.status === 'polling');
	let isFormValid = $derived(
		isScanFormValid({
			file: $scanStore.selectedFile,
			locationId: $scanStore.locationId,
			radiusKm: $scanStore.radiusKm
		})
	);

	async function submitScan() {
		const { selectedFile, locationId, radiusKm } = $scanStore;
		const nextErrors = validateScanForm({ file: selectedFile, locationId, radiusKm });
		scanStore.patch({ errors: nextErrors });

		if (Object.keys(nextErrors).length > 0 || !selectedFile) {
			scanStore.patch({ status: 'idle' });
			return;
		}

		scanStore.patch({ status: 'submitting', errorMessage: '', matches: [] });

		try {
			const accepted = await startScan({ file: selectedFile, locationId: locationId.trim(), radiusKm });
			scanStore.patch({ scanId: accepted.scanId, status: 'polling' });
			await pollScan(accepted.scanId);
		} catch (error) {
			scanStore.patch({ status: 'error', errorMessage: getErrorMessage(error) });
		}
	}

	async function pollScan(id: string) {
		const deadline = Date.now() + POLL_TIMEOUT_MS;

		while (Date.now() < deadline) {
			const response = await getScanStatus(id);

			if (response.status === 'completed') {
				scanStore.patch({ matches: response.matches, status: response.matches.length > 0 ? 'success' : 'empty' });
				return;
			}

			if (response.status === 'failed') {
				throw new ApiError(response.message, 422, 'scan_failed');
			}

			await wait(POLL_INTERVAL_MS);
		}

		throw new ApiError('Quá thời gian chờ kết quả. Anh/chị có thể thử lại.', 408, 'scan_timeout');
	}

	function handleFileChange(file: File | null) {
		scanStore.patch({ selectedFile: file });
		const fileValidation = validateCvFile(file);
		if (fileValidation.valid) {
			const { file: _file, ...rest } = $scanStore.errors;
			scanStore.patch({ errors: rest });
		} else {
			scanStore.patch({ errors: { ...$scanStore.errors, file: fileValidation.message } });
		}

		if ($scanStore.status === 'error' || $scanStore.status === 'empty' || $scanStore.status === 'success') {
			scanStore.patch({ status: 'idle', errorMessage: '', matches: [] });
		}
	}

	function handleLocationChange(value: string) {
		scanStore.patch({ locationId: value });
		if ($scanStore.errors.location && value.trim()) {
			const { location: _locationId, ...rest } = $scanStore.errors;
			scanStore.patch({ errors: rest });
		}
	}

	function handleRadiusChange(value: number) {
		scanStore.patch({ radiusKm: value });
		if ($scanStore.errors.radiusKm && value > 0) {
			const { radiusKm: _radiusKm, ...rest } = $scanStore.errors;
			scanStore.patch({ errors: rest });
		}
	}

	function retryScan() {
		scanStore.patch({ status: 'idle', errorMessage: '' });
	}

	function wait(ms: number) {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	function getErrorMessage(error: unknown) {
		if (error instanceof ApiError) {
			return error.message;
		}
		return 'Đã có lỗi khi xử lý CV. Vui lòng thử lại.';
	}

	function handlePromotionImageError() {
		promotions = [];
	}
</script>

<svelte:head>
	<title>Go get a job ferris — Tìm việc phù hợp với bạn</title>
	<meta name="description" content="Tải CV, chọn khu vực và nhận các cơ hội việc làm phù hợp từ nguồn tuyển dụng chính thức." />
</svelte:head>

<div class="client-page">
	<div class="ambient ambient-left" aria-hidden="true"></div>
	<div class="ambient ambient-right" aria-hidden="true"></div>

	<ClientHeader />

		<main class="page-container">
		<section class:has-promotions={promotions.length > 0} class="hero-grid" aria-label="Bắt đầu tìm việc phù hợp">
			{#if promotions.length > 0}
				<div class="hero-primary">
					<PromotionCarousel slides={promotions} onImageError={handlePromotionImageError} />
					{#if $scanStore.status === 'idle'}
						<section class="results-placeholder hero-results" aria-live="polite">
							<div class="placeholder-orbit" aria-hidden="true"><span></span></div>
							<div><span class="section-kicker">MATCH RESULTS</span><h2>Kết quả sẽ xuất hiện ở đây.</h2><p>Upload CV và chọn khu vực để bắt đầu. Go get a job ferris chỉ hiển thị dữ liệu trả về từ matching service thật.</p></div>
						</section>
					{/if}
				</div>
			{:else}
			<div class="hero-copy">
				<div class="eyebrow"><span class="eyebrow-dot"></span> CV-to-job matching</div>
				<h1 id="hero-title">Tìm công việc thật sự <em>phù hợp</em> với bạn.</h1>
				<p class="hero-lede">Go get a job ferris giúp bạn biến CV thành một hồ sơ kỹ năng rõ ràng, sau đó xếp hạng những cơ hội việc làm đang còn hoạt động theo mức độ phù hợp có thể giải thích.</p>

				<div class="benefit-list" aria-label="Lợi ích chính">
					<div class="benefit-item"><span class="benefit-icon">01</span><div><strong>Hiểu đúng hồ sơ</strong><span>AI đọc kỹ năng, vai trò và kinh nghiệm trong CV.</span></div></div>
					<div class="benefit-item"><span class="benefit-icon">02</span><div><strong>Chấm điểm minh bạch</strong><span>Matching engine tính CV Match % theo rule cố định.</span></div></div>
					<div class="benefit-item"><span class="benefit-icon">03</span><div><strong>Đi thẳng tới nguồn</strong><span>Đọc JD và ứng tuyển trên career page chính thức.</span></div></div>
				</div>

				<div class="profile-signal" aria-label="Luồng xử lý CV">
					<div class="signal-stage signal-stage-primary"><span>CV</span><small>upload</small></div>
					<div class="signal-line"></div>
					<div class="signal-stage"><span>PROFILE</span><small>structured</small></div>
					<div class="signal-line"></div>
					<div class="signal-stage signal-stage-accent"><span>MATCH</span><small>deterministic</small></div>
				</div>
			</div>
			{/if}

			<CvUploadForm
				selectedFile={$scanStore.selectedFile}
				locationId={$scanStore.locationId}
				locations={locations}
				locationsLoading={locationsLoading}
				locationServiceError={locationServiceError}
				radiusKm={$scanStore.radiusKm}
				errors={$scanStore.errors}
				disabled={isBusy}
				formValid={isFormValid}
				onFileChange={handleFileChange}
				onLocationChange={handleLocationChange}
				onRadiusChange={handleRadiusChange}
				onSubmit={submitScan}
			/>
		</section>

		{#if $scanStore.status === 'submitting' || $scanStore.status === 'polling'}
			<ScanProgress status={$scanStore.status} />
		{:else if $scanStore.status === 'success'}
			<MatchResults matches={$scanStore.matches} />
		{:else if $scanStore.status === 'empty'}
			<ClientEmptyState onRetry={retryScan} />
		{:else if $scanStore.status === 'error'}
			<ClientErrorState message={$scanStore.errorMessage} onRetry={retryScan} />
		{:else if promotions.length === 0}
			<section class="results-placeholder" aria-live="polite">
				<div class="placeholder-orbit" aria-hidden="true"><span></span></div>
				<div><span class="section-kicker">MATCH RESULTS</span><h2>Kết quả sẽ xuất hiện ở đây.</h2><p>Upload CV và chọn khu vực để bắt đầu. Go get a job ferris chỉ hiển thị dữ liệu trả về từ matching service thật.</p></div>
			</section>
		{/if}
	</main>

</div>
