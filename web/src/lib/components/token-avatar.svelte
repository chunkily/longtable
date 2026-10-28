<script lang="ts">
	// A token's picture anywhere outside the map — its art, or the disc a
	// token without art wears, with its icon or its initials on it.
	//
	// One component so the initiative panel, the party dialog and the token
	// dialogs all draw a goblin the way the map does, from discLook and the
	// icon list rather than each working it out. No ring here: the map's
	// two-tone ring is there to stand out against map art, and these sit on
	// a themed panel instead.
	import { assetUrl } from '$lib/api';
	import { discLook, initials } from '$lib/token-look';
	import { tokenIcon } from '$lib/token-icons';

	let {
		id,
		name,
		imageAssetId = null,
		icon = '',
		color = '',
		class: className = 'h-6 w-6'
	}: {
		/** The token's id — what its colour hashes from when none was chosen. */
		id: string;
		name: string;
		imageAssetId?: string | null;
		icon?: string | null;
		color?: string | null;
		/** Size classes, square. */
		class?: string;
	} = $props();

	const look = $derived(discLook({ id, color }));
	const glyph = $derived(tokenIcon(icon));
</script>

{#if imageAssetId}
	<img
		src={assetUrl(imageAssetId)}
		alt=""
		class={['shrink-0 rounded-full object-cover', className]}
	/>
{:else}
	<span
		class={['flex shrink-0 items-center justify-center rounded-full', className]}
		style="background-color: {look.fill}; color: {look.ink};"
		aria-hidden="true"
	>
		{#if glyph}
			<glyph.icon class="h-3/5 w-3/5" />
		{:else}
			<span class="text-[10px] leading-none font-bold">{initials(name)}</span>
		{/if}
	</span>
{/if}
