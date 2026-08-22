<script lang="ts">
	import type { ScanFieldErrors } from '$lib/client/validation/scan-form';
	import type { ClientLocation } from '$lib/shared/types/client';
	import ClientLocationSelect from './ClientLocationSelect.svelte';

	interface Props {
		selectedFile: File | null;
		locationId: string;
		locations: ClientLocation[];
		locationsLoading: boolean;
		locationServiceError: string;
		errors: ScanFieldErrors;
		disabled: boolean;
		formValid: boolean;
		authenticated: boolean;
		onFileChange: (file: File | null) => void;
		onLocationChange: (value: string) => void;
		onSubmit: () => void;
	}

	let { selectedFile, locationId, locations, locationsLoading, locationServiceError, errors, disabled, formValid, authenticated, onFileChange, onLocationChange, onSubmit }: Props = $props();
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
	<p class="upload-intro">Chọn Job Location để matching engine tìm các job đang hoạt động phù hợp với bạn.</p>

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

		<div class="field-grid single-field">
			<div class="field-group">
				<span><span class="field-icon" aria-hidden="true">⌖</span> Khu vực</span>
				<ClientLocationSelect locations={locations} value={locationId} loading={locationsLoading} disabled={disabled} error={errors.location || locationServiceError} onChange={onLocationChange} />
			</div>
		</div>

		<button class="primary-button" type="submit" disabled={disabled || !formValid}>
			{#if disabled}<span class="button-spinner" aria-hidden="true"></span> Đang xử lý…{:else if authenticated}Quét CV & tìm việc <span aria-hidden="true">→</span>{:else}Đăng nhập để quét CV <span aria-hidden="true">→</span>{/if}
		</button>
	</form>

	<p class="privacy-note"><span aria-hidden="true">⌑</span> File gốc chỉ dùng để parse và sẽ được xóa; lịch sử của bạn chỉ giữ structured profile.</p>
</section>
