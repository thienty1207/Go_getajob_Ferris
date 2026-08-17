<script lang="ts">
	import '../lib/shared/styles/global.css';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href="/brand/sugoi-oniichan-logo.png" />
</svelte:head>

{@render children()}
