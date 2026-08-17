<script lang="ts">
	import type { ScanFieldErrors } from '$lib/client/validation/scan-form';
	import type { ClientLocation } from '$lib/shared/types/client';

	interface Props {
		selectedFile: File | null;
		locationId: string;
		locations: ClientLocation[];
		locationsLoading: boolean;
		locationServiceError: string;
		radiusKm: number;
		errors: ScanFieldErrors;
		disabled: boolean;
		formValid: boolean;
		onFileChange: (file: File | null) => void;
		onLocationChange: (value: string) => void;
		onRadiusChange: (value: number) => void;
		onSubmit: () => void;
	}

	let { selectedFile, locationId, locations, locationsLoading, locationServiceError, radiusKm, errors, disabled, formValid, onFileChange, onLocationChange, onRadiusChange, onSubmit }: Props = $props();
	let isDragOver = $state(false);
	let fileInput: HTMLInputElement;

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		onSubmit();
	}

	function chooseFile() {
		fileInput?.click();
	}

	function handleInput(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		onFileChange(input.files?.[0] ?? null);
		input.value = '';
	}

	function handleDrop(event: DragEvent) {
		event.preventDefault();
		isDragOver = false;
		if (!disabled) onFileChange(event.dataTransfer?.files?.[0] ?? null);
	}

	function formatSize(bytes: number) {
		return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
	}
</script>

<section class="upload-card" aria-labelledby="upload-title">
	<div class="upload-heading"><span class="upload-icon" aria-hidden="true">↑</span><div><h2 id="upload-title">Tải CV của bạn lên</h2></div></div>
	<p class="upload-intro">Chọn khu vực, bán kính và để matching engine tìm các job đang hoạt động phù hợp với bạn.</p>

	<form onsubmit={handleSubmit} novalidate>
		<div
			class:drag-over={isDragOver}
			class:has-file={selectedFile}
			class="dropzone"
			role="group"
			aria-label="Khu vực tải CV"
			ondragover={(event) => { event.preventDefault(); if (!disabled) isDragOver = true; }}
			ondragleave={() => (isDragOver = false)}
			ondrop={handleDrop}
		>
			<input
				bind:this={fileInput}
				class="visually-hidden"
				hidden
				type="file"
				accept=".pdf,.docx,.txt,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,text/plain"
				disabled={disabled}
				tabindex="-1"
				aria-hidden="true"
				onchange={handleInput}
				aria-describedby="file-help file-error"
				aria-invalid={Boolean(errors.file)}
				aria-required="true"
			/>

			{#if selectedFile}
				<div class="selected-file" aria-live="polite">
					<span class="file-badge">CV</span>
					<span class="file-detail"><strong>{selectedFile.name}</strong><small>{formatSize(selectedFile.size)} · Sẵn sàng để quét</small></span>
					<button class="file-remove" type="button" onclick={() => onFileChange(null)} disabled={disabled} aria-label="Bỏ file CV đã chọn">×</button>
				</div>
			{:else}
				<span class="drop-icon" aria-hidden="true">＋</span>
				<strong>Kéo thả CV vào đây</strong>
				<span id="file-help">PDF, DOCX hoặc TXT · tối đa 10 MB</span>
				<button class="secondary-button" type="button" onclick={chooseFile} disabled={disabled}>Chọn file</button>
			{/if}
		</div>
		{#if errors.file}<p class="field-error" id="file-error" role="alert">{errors.file}</p>{/if}

		<div class="field-grid">
			<label class="field-group" for="location">
				<span><span class="field-icon" aria-hidden="true">⌖</span> Khu vực</span>
				<select
					id="location"
					name="location_id"
					value={locationId}
					disabled={disabled || locationsLoading || locations.length === 0}
					onchange={(event) => onLocationChange((event.currentTarget as HTMLSelectElement).value)}
					aria-describedby="location-error"
					aria-invalid={Boolean(errors.location)}
					aria-required="true"
					required
				>
					<option value="" disabled>{locationsLoading ? 'Đang tải location…' : locations.length === 0 ? 'Chưa có location khả dụng' : 'Chọn tỉnh / thành phố'}</option>
					{#each locations as location (location.id)}
						<option value={location.id}>{location.displayName}</option>
					{/each}
				</select>
				{#if errors.location}<small class="field-error" id="location-error">{errors.location}</small>{/if}
				{#if locationServiceError}<small class="field-error">{locationServiceError}</small>{/if}
			</label>

			<label class="field-group" for="radius">
				<span><span class="field-icon" aria-hidden="true">◎</span> Bán kính</span>
				<select
					id="radius"
					name="radius_km"
					value={String(radiusKm)}
					disabled={disabled}
					onchange={(event) => onRadiusChange(Number((event.currentTarget as HTMLSelectElement).value))}
					aria-describedby="radius-error"
					aria-invalid={Boolean(errors.radiusKm)}
					aria-required="true"
					required
				>
					<option value="5">5 km</option>
					<option value="10">10 km</option>
					<option value="25">25 km</option>
					<option value="50">50 km</option>
					<option value="100">100 km</option>
				</select>
				{#if errors.radiusKm}<small class="field-error" id="radius-error">{errors.radiusKm}</small>{/if}
			</label>
		</div>

		<button class="primary-button" type="submit" disabled={disabled || !formValid}>
			{#if disabled}<span class="button-spinner" aria-hidden="true"></span> Đang xử lý…{:else}Quét CV & tìm việc <span aria-hidden="true">→</span>{/if}
		</button>
	</form>

	<p class="privacy-note"><span aria-hidden="true">⌑</span> File CV chỉ được xử lý tạm thời, không lưu như hồ sơ CV dài hạn.</p>
</section>
