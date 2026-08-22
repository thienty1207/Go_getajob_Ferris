<script lang="ts">
	import { onDestroy } from 'svelte';
	import { positionMenu } from '../location-select-position';

	export interface LocationSelectOption {
		id: string;
		label: string;
		disabled?: boolean;
	}

	interface Props {
		options: LocationSelectOption[];
		value: string;
		label: string;
		/** id ổn định dùng cho listbox id (tránh trùng khi lặp nhiều dropdown). */
		id: string;
		disabled?: boolean;
		onChange: (value: string | null) => void;
	}

	let { options, value, label, id, disabled = false, onChange }: Props = $props();

	let listboxId = $derived(`ls-listbox-${id}`);
	let open = $state(false);
	let activeIndex = $state(-1);
	let rootEl = $state<HTMLDivElement | null>(null);
	let triggerEl = $state<HTMLButtonElement | null>(null);
	let menuEl = $state<HTMLElement | null>(null);
	let menuStyle = $state('');

	let firstOptionIndex = $derived(options.findIndex((o) => !o.disabled));
	const selectedIndex = $derived(options.findIndex((o) => o.id === value));

	const activeOptionIndex = $derived.by(() => {
		const idx = options.findIndex((o) => o.id === value && !o.disabled);
		return idx >= 0 ? idx : firstOptionIndex;
	});
	const selectedOption = $derived(options[selectedIndex] ?? options[0]);

	function emitChange(idOrEmpty: string) {
		// '' đại diện "Chưa gán" -> null; id thật -> giữ nguyên string.
		onChange(idOrEmpty === '' ? null : idOrEmpty);
	}

	function selectIndex(index: number) {
		const opt = options[index];
		if (!opt || opt.disabled) return;
		emitChange(opt.id);
	}

	function openList() {
		if (disabled) return;
		open = true;
		activeIndex = activeOptionIndex;
	}

	function closeList() {
		open = false;
		activeIndex = -1;
	}

	// Render menu bằng position:fixed trong viewport để thoát khỏi ancestor bị
	// clip (`.job-table-wrap` overflow-x và `.admin-root` overflow:hidden). Vì
	// fixed nên tọa độ phải tính lại từ rect của trigger mỗi khi mở hoặc cuộn.
	function repositionMenu() {
		if (!open || !triggerEl) return;
		const rect = triggerEl.getBoundingClientRect();
		const pos = positionMenu(
			{ top: rect.top, left: rect.left, width: rect.width, bottom: rect.bottom },
			{ width: window.innerWidth, height: window.innerHeight }
		);
		menuStyle =
			`width:${pos.width}px;` +
			`left:${pos.left}px;` +
			(pos.top === undefined ? `bottom:${pos.bottom}px;` : `top:${pos.top}px;`) +
			`max-height:${pos.maxHeight}px;`;
	}

	function moveFocus(dir: -1 | 1) {
		if (!open) {
			openList();
			return;
		}
		let next = activeIndex;
		for (;;) {
			next = (next + dir + options.length) % options.length;
			const opt = options[next];
			if (opt && !opt.disabled) break;
			if (next === activeIndex) return;
		}
		activeIndex = next;
		repositionMenu();
	}

	function onKeydown(event: KeyboardEvent) {
		if (disabled) return;
		switch (event.key) {
			case 'Enter':
			case ' ':
			case 'ArrowDown':
				event.preventDefault();
				if (open && (event.key === 'Enter' || event.key === ' ')) {
					if (activeIndex >= 0) selectIndex(activeIndex);
					closeList();
				} else {
					moveFocus(1);
				}
				return;
			case 'ArrowUp':
				event.preventDefault();
				moveFocus(-1);
				return;
			case 'Escape':
				event.preventDefault();
				closeList();
				return;
			case 'Tab':
				closeList();
				return;
		}
	}

	function onDocumentPointerdown(event: PointerEvent) {
		if (!rootEl?.contains(event.target as Node) && !menuEl?.contains(event.target as Node)) closeList();
	}

	// Khi menu cố định theo viewport, cuộn/bó image bất kỳ đều có thể làm lệch tọa độ
	// đã tính -> tính lại tọa độ; resize thực sự thay đổi viewport thì phải đóng lại.
	function onAnyScroll() {
		repositionMenu();
	}
	function onResize() {
		if (open) closeList();
	}

	$effect(() => {
		if (open) {
			repositionMenu();
			document.addEventListener('scroll', onAnyScroll, true);
			window.addEventListener('resize', onResize);
			document.addEventListener('pointerdown', onDocumentPointerdown);
		} else {
			document.removeEventListener('scroll', onAnyScroll, true);
			window.removeEventListener('resize', onResize);
			document.removeEventListener('pointerdown', onDocumentPointerdown);
		}
		return () => {
			document.removeEventListener('scroll', onAnyScroll, true);
			window.removeEventListener('resize', onResize);
			document.removeEventListener('pointerdown', onDocumentPointerdown);
		};
	});

	onDestroy(() => {
		document.removeEventListener('scroll', onAnyScroll, true);
		window.removeEventListener('resize', onResize);
		document.removeEventListener('pointerdown', onDocumentPointerdown);
	});
</script>

<div
	bind:this={rootEl}
	id={id}
	class="location-select"
	class:open={open}
	class:disabled={disabled}
	aria-expanded={open}
	aria-haspopup="listbox"
>
	<button
		bind:this={triggerEl}
		class="location-select-trigger"
		type="button"
		aria-label={label}
		aria-controls={listboxId}
		aria-haspopup="listbox"
		aria-expanded={open}
		disabled={disabled}
		onclick={(event) => {
			event.preventDefault();
			if (open) closeList();
			else openList();
		}}
		onkeydown={onKeydown}
	>
		<span class="location-select-value">{selectedOption?.label ?? ''}</span>
		<span class="location-select-caret" aria-hidden="true">▾</span>
	</button>
</div>

{#if open}
	<div class="location-select-portal" style={menuStyle} role="presentation">
		<ul bind:this={menuEl} id={listboxId} class="location-select-listbox" role="listbox" aria-label={label}>
			{#each options as opt, index (opt.id)}
				<li
					role="option"
					class:selected={index === selectedIndex}
					class:active={index === activeIndex}
					class:disabled={opt.disabled}
					aria-selected={index === selectedIndex ? 'true' : 'false'}
					aria-disabled={opt.disabled ? 'true' : undefined}
					tabindex={open && index === activeIndex ? 0 : -1}
					data-index={index}
					data-empty={opt.id === '' ? 'true' : undefined}
					onmouseenter={() => {
						if (!opt.disabled) activeIndex = index;
					}}
					onkeydown={(event) => {
						if (disabled || opt.disabled) return;
						if (event.key === 'Enter' || event.key === ' ') {
							event.preventDefault();
							selectIndex(index);
							closeList();
						} else if (event.key === 'ArrowDown') {
							event.preventDefault();
							moveFocus(1);
						} else if (event.key === 'ArrowUp') {
							event.preventDefault();
							moveFocus(-1);
						} else if (event.key === 'Escape') {
							event.preventDefault();
							closeList();
						}
					}}
					onclick={() => {
						if (disabled || opt.disabled) return;
						selectIndex(index);
						closeList();
					}}
				>
					{opt.label}
				</li>
			{/each}
		</ul>
	</div>
{/if}
