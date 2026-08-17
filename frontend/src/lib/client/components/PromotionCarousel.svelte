<script lang="ts">
	import { onMount } from 'svelte';
	import type { PromotionSlide } from '../../shared/types/client';
	import { nextPromotionIndex, previousPromotionIndex, promotionTrackTransform, shouldAutoplay, visiblePromotionSlides } from './promotion-carousel';

	interface Props {
		slides: PromotionSlide[];
		onImageError?: () => void;
	}

	let { slides, onImageError }: Props = $props();
	let activeIndex = $state(0);
	let paused = $state(false);
	let reducedMotion = $state(false);
	let pageHidden = $state(false);
	let carouselRoot = $state<HTMLElement | undefined>(undefined);
	let visibleSlides = $derived(visiblePromotionSlides(slides));
	let activeSlide = $derived(visibleSlides[activeIndex]);
	let trackTransform = $derived(promotionTrackTransform(activeIndex, visibleSlides.length));

	$effect(() => {
		if (visibleSlides.length > 0 && activeIndex >= visibleSlides.length) {
			activeIndex = 0;
		}
	});

	$effect(() => {
		if (!shouldAutoplay(visibleSlides.length, paused, reducedMotion, pageHidden)) return;
		const timer = setInterval(() => {
			activeIndex = nextPromotionIndex(activeIndex, visibleSlides.length);
		}, 5000);
		return () => clearInterval(timer);
	});

	onMount(() => {
		const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
		const updateMotionPreference = () => (reducedMotion = mediaQuery.matches);
		const updateVisibility = () => (pageHidden = document.hidden);
		updateMotionPreference();
		updateVisibility();
		mediaQuery.addEventListener('change', updateMotionPreference);
		document.addEventListener('visibilitychange', updateVisibility);

		return () => {
			mediaQuery.removeEventListener('change', updateMotionPreference);
			document.removeEventListener('visibilitychange', updateVisibility);
		};
	});

	function showNext() {
		activeIndex = nextPromotionIndex(activeIndex, visibleSlides.length);
	}

	function showPrevious() {
		activeIndex = previousPromotionIndex(activeIndex, visibleSlides.length);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (visibleSlides.length <= 1) return;
		if (event.key === 'ArrowRight') {
			event.preventDefault();
			showNext();
		} else if (event.key === 'ArrowLeft') {
			event.preventDefault();
			showPrevious();
		} else if (event.key === 'Home') {
			event.preventDefault();
			activeIndex = 0;
		} else if (event.key === 'End') {
			event.preventDefault();
			activeIndex = visibleSlides.length - 1;
		}
	}

	function handleFocusOut(event: FocusEvent) {
		const nextTarget = event.relatedTarget;
		if (!(nextTarget instanceof Node) || !carouselRoot?.contains(nextTarget)) {
			paused = false;
		}
	}

	function handleImageError() {
		onImageError?.();
	}
</script>

{#if activeSlide}
	<!-- Deliberate interaction exception: the region owns hover/focus pause and arrow-key carousel controls. -->
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<section
		class="promotion-carousel"
		bind:this={carouselRoot}
		aria-label="Quảng bá từ Sugoi-oniichan"
		aria-roledescription="carousel"
		tabindex="0"
		onmouseenter={() => (paused = true)}
		onmouseleave={() => (paused = false)}
		onfocusin={() => (paused = true)}
		onfocusout={handleFocusOut}
		onkeydown={handleKeydown}
	>
		<div class="promotion-stage" aria-live="polite">
			<div
				class="promotion-track"
				style={`--slide-count: ${visibleSlides.length}; transform: ${trackTransform};`}
			>
				{#each visibleSlides as slide, index (slide.slot)}
					<div class="promotion-slide" aria-hidden={index !== activeIndex}>
						<img src={slide.imageUrl} alt={slide.altText} onerror={handleImageError} />
					</div>
			{/each}
			</div>
			<div class="promotion-shade" aria-hidden="true"></div>
		</div>

		{#if visibleSlides.length > 1}
			<div class="promotion-controls" aria-label="Điều khiển quảng bá">
				<button class="promotion-arrow" type="button" aria-label="Slide trước" onclick={showPrevious}>←</button>
				<div class="promotion-dots">
					{#each visibleSlides as slide, index}
						<button
							class:active={index === activeIndex}
							type="button"
							aria-label={`Chuyển tới slide ${index + 1}`}
							aria-pressed={index === activeIndex}
							onclick={() => (activeIndex = index)}
						></button>
					{/each}
				</div>
				<button class="promotion-arrow" type="button" aria-label="Slide tiếp theo" onclick={showNext}>→</button>
			</div>
		{/if}
	</section>
{/if}

<style>
	.promotion-carousel {
		position: relative;
		width: 100%;
		max-width: 100%;
		min-width: 0;
		align-self: start;
		aspect-ratio: 16 / 9;
		overflow: hidden;
		border: 0;
		border-radius: var(--radius-lg);
		background: #0b0f14;
		box-shadow: none;
		isolation: isolate;
	}
	.promotion-carousel:focus-visible {
		outline: 2px solid var(--accent-bright);
		outline-offset: 4px;
	}
	.promotion-stage,
	.promotion-shade {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
	}
	.promotion-stage {
		overflow: hidden;
	}
	.promotion-track {
		display: flex;
		width: calc(var(--slide-count) * 100%);
		height: 100%;
		transition: transform 650ms cubic-bezier(0.22, 0.61, 0.36, 1);
		will-change: transform;
	}
	.promotion-slide {
		position: relative;
		flex: 0 0 calc(100% / var(--slide-count));
		min-width: 0;
		height: 100%;
	}
	.promotion-slide img {
		display: block;
		width: 100%;
		height: 100%;
		object-fit: cover;
		object-position: center;
		background: #0b0f14;
	}
	.promotion-shade {
		z-index: 1;
		background: linear-gradient(180deg, rgba(6, 9, 13, 0.08), transparent 44%, rgba(6, 9, 13, 0.2));
	}
	.promotion-controls { position: absolute; z-index: 3; right: 22px; bottom: 21px; left: 22px; display: flex; align-items: center; justify-content: space-between; }
	.promotion-arrow { display: grid; place-items: center; width: 38px; height: 38px; border: 1px solid rgba(255, 255, 255, 0.28); border-radius: 50%; color: var(--text); background: rgba(7, 9, 12, 0.52); font-size: 1rem; }
	.promotion-arrow:hover { border-color: var(--accent-bright); background: rgba(255, 48, 77, 0.35); }
	.promotion-dots { display: flex; align-items: center; gap: 7px; }
	.promotion-dots button { width: 7px; height: 7px; padding: 0; border: 1px solid rgba(255, 255, 255, 0.62); border-radius: 50%; background: transparent; }
	.promotion-dots button.active { width: 24px; border-color: var(--accent); border-radius: 99px; background: var(--accent); }
	@media (prefers-reduced-motion: reduce) {
		.promotion-track { transition-duration: 0ms; }
	}

	@media (max-width: 900px) {
		.promotion-carousel { aspect-ratio: 16 / 9; }
	}
	@media (max-width: 620px) {
		.promotion-carousel { aspect-ratio: 16 / 9; border-radius: 18px; }
		.promotion-controls { right: 18px; bottom: 17px; left: 18px; }
	}
</style>
