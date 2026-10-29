<script lang="ts">
	// A token icon off the canvas, whichever pool it came from: a Lucide
	// component, or a vendored game-icons.net SVG drawn in currentColor.
	import type { TokenIcon } from '$lib/token-icons';

	let { icon, class: className = 'h-4 w-4' }: { icon: TokenIcon; class?: string } = $props();
</script>

{#if icon.icon}
	<icon.icon class={className} />
{:else if icon.svg}
	<svg
		xmlns="http://www.w3.org/2000/svg"
		viewBox={icon.svg.viewBox}
		class={className}
		aria-hidden="true"
	>
		<!-- eslint-disable-next-line svelte/no-at-html-tags -- vendored at build time by scripts/vendor-game-icons.mjs, never anything a user sent -->
		{@html icon.svg.body}
	</svg>
{/if}
