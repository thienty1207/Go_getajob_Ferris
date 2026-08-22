<script lang="ts">
	import { onMount } from 'svelte';
	import type { HomeSection } from '$lib/shared/types/home-section';

	interface Props { section: HomeSection; }
	let { section }: Props = $props();
	let paused = $state(false);
	let reducedMotion = $state(false);
	let items = $derived(section.media.filter((media) => media.isActive));
	let firstRowItems = $derived(items.filter((_, index) => index % 2 === 0));
	let secondRowItems = $derived(items.filter((_, index) => index % 2 === 1));
	let desktopLoopItems = $derived([...items, ...items]);
	let firstRowLoopItems = $derived([...firstRowItems, ...firstRowItems]);
	let secondRowLoopItems = $derived([...secondRowItems, ...secondRowItems]);

	function sectionLabel(): string {
		return section.title?.trim() || `Home section ${section.slot}`;
	}

	onMount(() => {
		const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
		const update = () => (reducedMotion = mediaQuery.matches);
		update();
		mediaQuery.addEventListener('change', update);
		return () => mediaQuery.removeEventListener('change', update);
	});
</script>

	{#if items.length > 0}
	<section class="home-media-strip" aria-label={sectionLabel()}>
		{#if section.title || section.body}
			<div class="home-media-strip-heading">
				{#if section.title}<h2 id={`home-section-title-${section.slot}`}>{section.title}</h2>{/if}
				{#if section.body}<p>{section.body}</p>{/if}
			</div>
		{/if}
		<div class:paused class:reduced-motion={reducedMotion} class="home-media-viewport" role="region" aria-label="Dải ảnh giới thiệu" onmouseenter={() => (paused = true)} onmouseleave={() => (paused = false)}>
			<div class="home-media-desktop-track" role="list" aria-label="Hình ảnh giới thiệu">
				{#each desktopLoopItems as media, index (media.id + '-desktop-' + index)}
					{#if index < items.length}
						<div class="home-media-card" role="listitem"><img src={media.imageUrl} alt={media.imageAltText} loading="lazy" /></div>
					{:else}
						<div class="home-media-card" role="listitem" aria-hidden="true"><img src={media.imageUrl} alt="" loading="lazy" /></div>
					{/if}
				{/each}
			</div>
			<div class="home-media-mobile-rows" aria-hidden="true">
				{#if firstRowItems.length > 0}<div class="home-media-row"><div class="home-media-track">
					{#each firstRowLoopItems as media, index (media.id + '-first-' + index)}<div class="home-media-card" role="presentation"><img src={media.imageUrl} alt="" loading="lazy" /></div>{/each}
				</div></div>{/if}
				{#if secondRowItems.length > 0}<div class="home-media-row home-media-row-reverse"><div class="home-media-track">
					{#each secondRowLoopItems as media, index (media.id + '-second-' + index)}<div class="home-media-card" role="presentation"><img src={media.imageUrl} alt="" loading="lazy" /></div>{/each}
				</div></div>{/if}
			</div>
		</div>
	</section>
{/if}
