<script lang="ts">
	import type { AdminJobPage, AdminLocationOption } from '../api/admin-api';

	interface Props {
		data: AdminJobPage;
		locations: AdminLocationOption[];
		assigningJobId: string | null;
		onLocationChange: (jobId: string, locationId: string | null) => Promise<void>;
		onPageChange: (page: number) => void;
	}

	let { data, locations, assigningJobId, onLocationChange, onPageChange }: Props = $props();
	let totalPages = $derived(Math.max(1, Math.ceil(data.total / data.pageSize)));

	function dateLabel(value: string) {
		const date = new Date(value);
		return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('vi-VN', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
	}

	function statusClass(status: string) {
		return ['active', 'disabled', 'verifying', 'closed', 'expired'].includes(status) ? status : '';
	}

	function changeLocation(jobId: string, event: Event) {
		const locationId = (event.currentTarget as HTMLSelectElement).value || null;
		void onLocationChange(jobId, locationId);
	}
</script>

<div class="job-toolbar"><span><strong>{data.total}</strong> structured job rows</span><span>Trang {data.page} / {totalPages}</span></div>
{#if data.items.length === 0}
	<div class="admin-status"><span class="admin-status-symbol">⌕</span><span>Job cache hiện chưa có dữ liệu. Khi crawler có source được duyệt, các structured row sẽ xuất hiện ở đây.</span></div>
{:else}
	<div class="job-table-wrap">
		<table class="job-table">
			<thead><tr><th>Job</th><th>Source</th><th>Location</th><th>Lifecycle</th><th>Mode</th><th>Last seen</th><th>Hash</th></tr></thead>
			<tbody>
				{#each data.items as job (job.id)}
					<tr>
						<td class="job-title-cell"><strong title={job.title}>{job.title}</strong><span>{job.company} · {job.location}</span></td>
						<td class="job-source-cell"><span>{job.sourceName}</span><small>{job.sourceKey}</small>{#if job.isDevelopmentFixture}<span class="job-badge fixture">Dev fixture</span>{/if}</td>
						<td class="job-location-cell"><select aria-label={`Location cho ${job.title}`} value={job.locationId ?? ''} onchange={(event) => changeLocation(job.id, event)} disabled={assigningJobId !== null}><option value="">Chưa gán</option>{#each locations as location (location.id)}<option value={location.id} disabled={!location.isActive && location.id !== job.locationId}>{location.displayName}{location.isActive ? '' : ' · DISABLED'}</option>{/each}</select></td>
						<td><span class={`job-badge ${statusClass(job.status)}`}>{job.status}</span></td>
						<td>{job.workMode}<br /><small>{job.employmentType}</small></td>
						<td>{dateLabel(job.lastSeenAt)}</td>
						<td><span class="job-hash" title={job.contentHash}>{job.contentHash}</span><a class="job-link-small" href={job.originalUrl} target="_blank" rel="noreferrer">Original ↗</a></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<div class="job-mobile-list">
		{#each data.items as job (job.id)}
			<article class="job-mobile-card">
				<div style="display:flex; align-items:center; justify-content:space-between; gap:10px;"><span class={`job-badge ${statusClass(job.status)}`}>{job.status}</span>{#if job.isDevelopmentFixture}<span class="job-badge fixture">Dev fixture</span>{/if}</div>
				<h3>{job.title}</h3><p>{job.company} · {job.location}</p>
				<label class="job-mobile-location">Location<select aria-label={`Location cho ${job.title}`} value={job.locationId ?? ''} onchange={(event) => changeLocation(job.id, event)} disabled={assigningJobId !== null}><option value="">Chưa gán</option>{#each locations as location (location.id)}<option value={location.id} disabled={!location.isActive && location.id !== job.locationId}>{location.displayName}{location.isActive ? '' : ' · DISABLED'}</option>{/each}</select></label>
				<div class="job-mobile-meta"><span>{job.workMode}</span><span>{job.employmentType}</span><span>{job.sourceKey}</span></div>
				<p>Last seen: {dateLabel(job.lastSeenAt)}</p>
				<a class="job-link-small" href={job.originalUrl} target="_blank" rel="noreferrer">Mở original URL ↗</a>
			</article>
		{/each}
	</div>
{/if}

<div class="job-pagination"><button class="admin-secondary-button" type="button" disabled={data.page <= 1} onclick={() => onPageChange(data.page - 1)}>← Trước</button><span>{data.page} / {totalPages}</span><button class="admin-secondary-button" type="button" disabled={data.page >= totalPages} onclick={() => onPageChange(data.page + 1)}>Sau →</button></div>
