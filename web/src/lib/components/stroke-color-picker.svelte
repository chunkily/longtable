<script lang="ts">
	// The draw strip's colour: one button wearing the colour it will draw
	// in, and the eight behind it.
	//
	// The four light-map colours sat on the strip itself first, and a
	// second row for dark maps was added below them. That worked and was
	// too thick — 66px of strip is a band of map nobody can see, and the
	// strip floats over the top-left corner where the art usually starts.
	// Behind a button the whole palette costs less room than half of it
	// did on the strip, which is the same trade stroke-width-picker.svelte
	// makes one button to the left, for the same reason: a setting picked
	// once and then left alone shouldn't hold the map hostage.
	//
	// On the popover primitive for focus and placement, and because it
	// portals to the body — see stroke-width-picker.svelte, which has the
	// long version of why a hand-rolled panel is clipped by the bottom
	// sheet below lg.
	import * as Popover from '$lib/components/ui/popover';
	import { Button } from '$lib/components/ui/button';
	import { DEFAULT_STROKE_COLOR, STROKE_COLOR_ROWS } from '$lib/stroke-colors';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';

	let { strokeColor = $bindable(DEFAULT_STROKE_COLOR) }: { strokeColor?: string } = $props();

	let open = $state(false);

	// A colour that isn't in the palette can only come from a stale stored
	// value or a swatch being removed later; showing the first is better
	// than showing a blank button.
	const current = $derived(
		STROKE_COLOR_ROWS.flat().find((colour) => colour.value === strokeColor) ??
			STROKE_COLOR_ROWS[0][0]
	);

	function pick(value: string) {
		strokeColor = value;
		open = false;
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		<!-- The child snippet so the trigger is the app's own Button: the
		     primitive's attributes come through `props`, which is where
		     aria-expanded and aria-controls arrive from. -->
		{#snippet child({ props })}
			<Button
				{...props}
				variant="outline"
				size="sm"
				aria-label="Stroke colour: {current.label}"
				title="Stroke colour"
			>
				<!-- Bordered, like the swatches in the panel and for the same
				     reason: white on a light strip and black on a dark one are
				     otherwise a button with nothing in it. -->
				<span class="h-4 w-4 rounded-full border" style="background-color: {current.value}"></span>
				<ChevronDown class="h-3 w-3 opacity-60" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<!-- Aligned to the button's own edge rather than centred on it, since
	     the strip starts at the edge of the screen. -->
	<Popover.Content class="w-auto p-2" align="start" aria-label="Stroke colour">
		<!-- Four to a line, so each row is one map's worth of palette and
		     every dark colour sits under the light one it answers to. The
		     rows are always both here — the app's scheme says what the page
		     is wearing, and a dark battle map under a light UI is the case
		     the second row exists for. -->
		<div class="grid grid-cols-4 gap-2">
			{#each STROKE_COLOR_ROWS as row, i (i)}
				{#each row as colour (colour.value)}
					<button
						type="button"
						aria-label={colour.label}
						aria-pressed={strokeColor === colour.value}
						title={colour.label}
						class={[
							'h-7 w-7 rounded-full border',
							strokeColor === colour.value && 'outline-2 outline-offset-2 outline-sky-400'
						]}
						style="background-color: {colour.value}"
						onclick={() => pick(colour.value)}
					></button>
				{/each}
			{/each}
		</div>
	</Popover.Content>
</Popover.Root>
