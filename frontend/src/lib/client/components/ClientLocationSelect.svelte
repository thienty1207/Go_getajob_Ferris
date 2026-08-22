<script lang="ts">
	import { onDestroy } from 'svelte';
	import { positionMenu } from '$lib/admin/location-select-position';
	import type { ClientLocation } from '$lib/shared/types/client';

	interface Props {
		locations: ClientLocation[];
		value: string;
		loading: boolean;
		disabled?: boolean;
		error?: string;
		onChange: (value: string) => void;
	}

	let { locations, value, loading, disabled = false, error = '', onChange }: Props = $props();
	let open = $state(false);
	let activeIndex = $state(-1);
	let rootEl = $state<HTMLDivElement | null>(null);
	let triggerEl = $state<HTMLButtonElement | null>(null);
	let menuEl = $state<HTMLElement | null>(null);
	let menuStyle = $state('');

	let selectedIndex = $derived(locations.findIndex((location) => location.id === value));
	let selectedLocation = $derived(locations[selectedIndex] ?? null);
	let firstLocationIndex = $derived(locations.length > 0 ? 0 : -1);

	function openList() {
		if (disabled || loading || locations.length === 0) return;
		open = true;
		activeIndex = selectedIndex >= 0 ? selectedIndex : firstLocationIndex;
	}

	function closeList() {
		open = false;
		activeIndex = -1;
	}

	function selectLocation(index: number) {
		const location = locations[index];
		if (!location) return;
		onChange(location.id);
		closeList();
		triggerEl?.focus();
	}

	function repositionMenu() {
		if (!open || !triggerEl) return;
		const rect = triggerEl.getBoundingClientRect();
		const pos = positionMenu(
			{ top: rect.top, left: rect.left, width: rect.width, bottom: rect.bottom },
			{ width: window.innerWidth, height: window.innerHeight }
		);
		menuStyle =
			`width:${pos.width}px;left:${pos.left}px;` +
			(pos.top === undefined ? `bottom:${pos.bottom}px;` : `top:${pos.top}px;`) +
			`max-height:${pos.maxHeight}px;`;
	}

	function moveActive(direction: -1 | 1) {
		if (!open) {
			openList();
			return;
		}
		activeIndex = (activeIndex + direction + locations.length) % locations.length;
	}

	function onKeydown(event: KeyboardEvent) {
		if (disabled) return;
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			if (open) selectLocation(activeIndex);
			else openList();
			return;
		}
		if (event.key === 'ArrowDown' || event.key === 'ArrowRight') {
			event.preventDefault();
			moveActive(1);
			return;
		}
		if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') {
			event.preventDefault();
			moveActive(-1);
			return;
		}
		if (event.key === 'Escape' || event.key === 'Tab') closeList();
	}

	function onDocumentPointerdown(event: PointerEvent) {
		const target = event.target;
		if (target instanceof Node && !rootEl?.contains(target) && !menuEl?.contains(target)) closeList();
	}

	function onAnyScroll() {
		repositionMenu();
	}

	function onResize() {
		if (open) closeList();
	}

	$effect(() => {
		if (!open) return;
		repositionMenu();
		document.addEventListener('pointerdown', onDocumentPointerdown);
		document.addEventListener('scroll', onAnyScroll, true);
		window.addEventListener('resize', onResize);
		return () => {
			document.removeEventListener('pointerdown', onDocumentPointerdown);
			document.removeEventListener('scroll', onAnyScroll, true);
			window.removeEventListener('resize', onResize);
		};
	});

	onDestroy(() => {
		if (typeof document === 'undefined') return;
		document.removeEventListener('pointerdown', onDocumentPointerdown);
		document.removeEventListener('scroll', onAnyScroll, true);
		window.removeEventListener('resize', onResize);
	});
</script>

<div bind:this={rootEl} class="client-location-select" class:open class:disabled aria-invalid={Boolean(error)}>
	<button
		bind:this={triggerEl}
		type="button"
		class="client-location-trigger"
		aria-label="Chọn Job Location"
		aria-haspopup="listbox"
		aria-expanded={open}
		disabled={disabled || loading || locations.length === 0}
		onclick={() => (open ? closeList() : openList())}
		onkeydown={onKeydown}
	>
		<span>{loading ? 'Đang tải Job Location…' : selectedLocation?.displayName ?? (locations.length ? 'Chọn Job Location' : 'Chưa có Job Location')}</span>
		<span aria-hidden="true">▾</span>
	</button>
	{#if error}<small class="field-error" role="alert">{error}</small>{/if}
</div>

{#if open}
	<div class="client-location-portal" style={menuStyle} role="presentation">
		<ul bind:this={menuEl} class="client-location-listbox" role="listbox" aria-label="Job Location">
			{#each locations as location, index (location.id)}
				<li
					role="option"
					aria-selected={location.id === value}
					class:active={index === activeIndex}
					onmouseenter={() => (activeIndex = index)}
					onkeydown={(event) => {
						if (event.key === 'Enter' || event.key === ' ') {
							event.preventDefault();
							selectLocation(index);
						}
					}}
					onclick={() => selectLocation(index)}
				>
					{location.displayName}
				</li>
			{/each}
		</ul>
	</div>
{/if}
