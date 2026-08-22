<script lang="ts">
	import type { AdminJobPage, AdminLocationOption } from '../api/admin-api';
	import LocationSelect, { type LocationSelectOption } from './LocationSelect.svelte';
	import { toLocationPayload } from '../location';
	import { workModeLabel, employmentLabels, sourceDisplayLabel } from '../job-labels';

	interface Props {
		data: AdminJobPage;
		locations: AdminLocationOption[];
		assigningJobId: string | null;
		onLocationChange: (jobId: string, locationId: string | null) => Promise<void>;
		onPageChange: (page: number) => void;
	}

	let { data, locations, assigningJobId, onLocationChange, onPageChange }: Props = $props();
	let totalPages = $derived(Math.max(1, Math.ceil(data.total / data.pageSize)));

	function optionFor(jobId: string): LocationSelectOption[] {
		return [
			{ id: '', label: 'Chưa gán' },
			...locations.map((location) => ({
				id: location.id,
				label: `${location.displayName}${location.isActive ? '' : ' · DISABLED'}`,
				disabled: !location.isActive && location.id !== data.items.find((j) => j.id === jobId)?.locationId
			}))
		];
	}

	function dateLabel(value: string) {
		const date = new Date(value);
		return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('vi-VN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
	}

	function statusClass(status: string) {
		return ['active', 'disabled', 'verifying', 'closed', 'expired'].includes(status) ? status : '';
	}

	function changeLocation(jobId: string, locationId: string | null) {
		void onLocationChange(jobId, toLocationPayload(locationId));
	}
</script>

<div class="job-toolbar"><span><strong>{data.total}</strong> structured job rows</span><span>Trang {data.page} / {totalPages}</span></div>
{#if data.items.length === 0}
	<div class="admin-status"><span class="admin-status-symbol">⌕</span><span>Job cache hiện chưa có dữ liệu. Khi crawler có source được duyệt, các structured row sẽ xuất hiện ở đây.</span></div>
{:else}
	<div class="job-table-wrap">
		<table class="job-table">
			<thead><tr><th>Job</th><th>Source</th><th>Location</th><th>Lifecycle</th><th>Mode</th><th>Type</th><th>Last seen</th><th>Original</th></tr></thead>
			<tbody>
				{#each data.items as job (job.id)}
					<tr>
						<td class="job-title-cell"><strong title={job.title}>{job.title}</strong><span>{job.company}{#if job.location} · {job.location}{/if}</span></td>
						<td class="job-source-cell"><span>{sourceDisplayLabel(job.sourceName)}</span>{#if job.isDevelopmentFixture}<span class="job-badge fixture">Dev fixture</span>{/if}</td>
						<td class="job-location-cell">
							<LocationSelect
								id="job-loc-{job.id}"
								options={optionFor(job.id)}
								value={job.locationId ?? ''}
								label="Location cho {job.title}"
								disabled={assigningJobId !== null}
								onChange={(id) => changeLocation(job.id, id)}
							/>
						</td>
						<td><span class={`job-badge ${statusClass(job.status)}`}>{job.status}</span></td>
						<td><span class="job-mode-badge is-{job.workMode.toLowerCase()}">{workModeLabel(job.workMode)}</span></td>
						<td class="job-type-badges">{#each employmentLabels(job.employmentType) as emp, i (i)}<span class="job-type-badge">{emp}</span>{/each}</td>
						<td class="job-lastseen-cell">{dateLabel(job.lastSeenAt)}</td>
						<td><a class="job-link-small" href={job.originalUrl} target="_blank" rel="noreferrer" title={job.originalUrl} aria-label="Mở job gốc trong tab mới">Original ↗</a></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<div class="job-mobile-list">
		{#each data.items as job (job.id)}
			<article class="job-mobile-card">
				<div class="job-mobile-topline"><span class={`job-badge ${statusClass(job.status)}`}>{job.status}</span>{#if job.isDevelopmentFixture}<span class="job-badge fixture">Dev fixture</span>{/if}</div>
				<h3 class="job-mobile-title">{job.title}</h3>
				<p class="job-mobile-sub">{job.company}{#if job.location} · {job.location}{/if}</p>
				<div class="job-mobile-row"><span class="job-mobile-label">Location</span>
					<LocationSelect
						id="job-loc-{job.id}-mobile"
						options={optionFor(job.id)}
						value={job.locationId ?? ''}
						label="Location cho {job.title}"
						disabled={assigningJobId !== null}
						onChange={(id) => changeLocation(job.id, id)}
					/>
				</div>
				<div class="job-mobile-grid">
					<div class="job-mobile-field"><span class="job-mobile-label">Lifecycle</span><span class={`job-badge ${statusClass(job.status)}`}>{job.status}</span></div>
					<div class="job-mobile-field job-mobile-wide"><span class="job-mobile-label">Chế độ làm việc</span><span class="job-mode-badge is-{job.workMode.toLowerCase()}">{workModeLabel(job.workMode)}</span></div>
					<div class="job-mobile-field job-mobile-wide"><span class="job-mobile-label">Loại hình</span><span class="job-type-badges">{#each employmentLabels(job.employmentType) as emp, i (i)}<span class="job-type-badge">{emp}</span>{/each}</span></div>
					<div class="job-mobile-field job-mobile-wide"><span class="job-mobile-label">Nguồn</span><span>{sourceDisplayLabel(job.sourceName)}</span></div>
				</div>
				<div class="job-mobile-field job-mobile-wide"><span class="job-mobile-label">Cập nhật lần cuối</span><span>{dateLabel(job.lastSeenAt)}</span></div>
				<a class="job-mobile-link" href={job.originalUrl} target="_blank" rel="noreferrer" title={job.originalUrl} aria-label="Mở job gốc trong tab mới">Original ↗</a>
			</article>
		{/each}
	</div>
{/if}

<div class="job-pagination"><button class="admin-secondary-button" type="button" disabled={data.page <= 1} onclick={() => onPageChange(data.page - 1)}>← Trước</button><span>{data.page} / {totalPages}</span><button class="admin-secondary-button" type="button" disabled={data.page >= totalPages} onclick={() => onPageChange(data.page + 1)}>Sau →</button></div>
