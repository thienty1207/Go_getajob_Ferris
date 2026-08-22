<script lang="ts">
	import { onMount } from 'svelte';
	import { get } from 'svelte/store';
	import { ApiError } from '$lib/shared/api/api-errors';
	import { getClientLocations, getPromotions, startScan } from '$lib/client/api/client-api';
	import { getClientHomeSections } from '$lib/client/api/client-home-api';
	import ClientErrorState from '$lib/client/components/ClientErrorState.svelte';
	import ClientHeader from '$lib/client/components/ClientHeader.svelte';
	import CvUploadForm from '$lib/client/components/CvUploadForm.svelte';
	import PromotionCarousel from '$lib/client/components/PromotionCarousel.svelte';
	import { clientAuth } from '$lib/client/stores/client-auth-store';
	import { createScanStore } from '$lib/client/stores/scan-store';
	import type { ClientLocation, PromotionSlide } from '$lib/shared/types/client';
	import type { HomeSection } from '$lib/shared/types/home-section';
	import HomeContentSection from '$lib/client/components/HomeContentSection.svelte';
	import HomeMediaMarquee from '$lib/client/components/HomeMediaMarquee.svelte';
	import { validateCvFile } from '$lib/client/validation/cv-file';
	import { isScanFormValid, validateScanForm } from '$lib/client/validation/scan-form';

	const scanStore = createScanStore();
	let promotions = $state<PromotionSlide[]>([]);
	let locations = $state<ClientLocation[]>([]);
	let locationsLoading = $state(true);
	let locationServiceError = $state('');
	let homeSections = $state<HomeSection[]>([]);

	onMount(async () => {
		const [promotionResult, locationResult, homeSectionResult] = await Promise.allSettled([getPromotions(), getClientLocations(), getClientHomeSections()]);
		promotions = promotionResult.status === 'fulfilled' ? promotionResult.value : [];
		if (locationResult.status === 'fulfilled') {
			locations = locationResult.value;
		} else {
			locationServiceError = getErrorMessage(locationResult.reason);
		}
		homeSections = homeSectionResult.status === 'fulfilled' ? homeSectionResult.value : [];
		locationsLoading = false;
	});

	let isBusy = $derived($scanStore.status === 'submitting' || $scanStore.status === 'polling');
	let isFormValid = $derived(
		isScanFormValid({
			file: $scanStore.selectedFile,
			locationId: $scanStore.locationId
		})
	);

	async function submitScan() {
		if ($clientAuth.status !== 'authenticated') {
			await clientAuth.loadCurrentUser();
			if (get(clientAuth).status !== 'authenticated') {
				window.location.href = '/client/login?return_to=/client';
				return;
			}
		}

		const { selectedFile, locationId } = $scanStore;
		const nextErrors = validateScanForm({ file: selectedFile, locationId });
		scanStore.patch({ errors: nextErrors });

		if (Object.keys(nextErrors).length > 0 || !selectedFile) {
			scanStore.patch({ status: 'idle' });
			return;
		}

		scanStore.patch({ status: 'submitting', errorMessage: '', matches: [] });

		try {
			const accepted = await startScan(
				{ file: selectedFile, locationId: locationId.trim() },
				get(clientAuth).csrfToken
			);
			window.location.href = `/client/scans/${encodeURIComponent(accepted.scanId)}`;
		} catch (error) {
			scanStore.patch({ status: 'error', errorMessage: getErrorMessage(error) });
		}
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
		<title>Sugoi-oniichan · Go get a job ferris</title>
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
				</div>
			{/if}

			<CvUploadForm
				selectedFile={$scanStore.selectedFile}
				locationId={$scanStore.locationId}
				locations={locations}
				locationsLoading={locationsLoading}
				locationServiceError={locationServiceError}
				errors={$scanStore.errors}
				disabled={isBusy}
				formValid={isFormValid}
				authenticated={$clientAuth.status === 'authenticated'}
				onFileChange={handleFileChange}
				onLocationChange={handleLocationChange}
				onSubmit={submitScan}
			/>
		</section>

		{#if $scanStore.status === 'error'}
			<ClientErrorState message={$scanStore.errorMessage} onRetry={() => scanStore.patch({ status: 'idle', errorMessage: '' })} />
		{/if}

		{#if homeSections.length > 0}
			<div class="home-sections">
				{#each homeSections as section (section.slot)}
					{#if section.slot === 4}<HomeMediaMarquee {section} />{:else}<HomeContentSection {section} />{/if}
				{/each}
			</div>
		{/if}
	</main>

</div>
