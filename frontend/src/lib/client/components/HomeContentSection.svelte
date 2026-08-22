<script lang="ts">
	import type { HomeSection } from '$lib/shared/types/home-section';

	interface Props { section: HomeSection; }
	let { section }: Props = $props();

	function sectionLabel(): string {
		return section.title?.trim() || `Home section ${section.slot}`;
	}
</script>

	<section class={`home-content-section ${section.layout === 'IMAGE_LEFT' ? 'image-left' : 'content-left'}`} aria-label={sectionLabel()}>
	<div class="home-content-copy">
		{#if section.title}<h2 id={`home-section-title-${section.slot}`}>{section.title}</h2>{/if}
		{#if section.body}<p>{section.body}</p>{/if}
	</div>
	{#if section.imageUrl}
		<div class="home-content-image">
			<img src={section.imageUrl} alt={section.imageAltText ?? section.title ?? 'Ảnh section Home'} loading="lazy" />
		</div>
	{/if}
</section>
