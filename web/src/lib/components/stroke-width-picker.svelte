<script lang="ts">
	// The draw strip's stroke width: one button showing the width it will
	// draw at, and the three choices behind it. Three buttons sat on the
	// strip first — correct, and a third of the row for a setting that is
	// picked once and then left alone, on the strip that scrolls first on
	// a phone.
	//
	// On the popover primitive for the two things that are expensive to
	// hand-roll and easy to leave out: focus, which it moves into the
	// panel and puts back on this button when the panel closes, and
	// placement, which it measures and flips with floating-ui. This was
	// hand-rolled first — a transparent full-screen backdrop and a panel
	// placed from the trigger's rectangle, the way room-menu.svelte used
	// to do it — and did neither.
	//
	// It also portals to the body, which settles a trap this particular
	// strip is sitting in. Below lg it docks into the sheet's
	// horizontally scrolling bar, and `overflow-x: auto` makes the other
	// axis `auto` too. A panel anchored to a `relative` wrapper inside
	// that bar — the shape a hand-rolled one takes — is clipped by it:
	// measured on a 375px viewport, the panel had a box above the strip
	// and `elementFromPoint` at its centre returned the canvas. It
	// escapes only if nothing between it and the sheet is positioned,
	// which is a property no one would think to preserve.
	import * as Popover from '$lib/components/ui/popover';
	import { STROKE_WIDTH_CHOICES } from '$lib/stroke-width';
	import { DRAWING_STROKE_WIDTH } from '$lib/drawing-hit';
	import { Button } from '$lib/components/ui/button';
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';

	let { strokeWidth = $bindable(DRAWING_STROKE_WIDTH) }: { strokeWidth?: number } = $props();

	let open = $state(false);

	// A width that isn't one of the three can only come from a stale
	// stored value or a choice being removed later; showing the first is
	// better than showing a blank button.
	const current = $derived(
		STROKE_WIDTH_CHOICES.find((choice) => choice.value === strokeWidth) ?? STROKE_WIDTH_CHOICES[0]
	);

	function pick(value: number) {
		strokeWidth = value;
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
				aria-label="Stroke width: {current.label}"
				title="Stroke width"
			>
				<span class="w-4 rounded-full bg-current" style="height: {current.bar}px"></span>
				<ChevronDown class="h-3 w-3 opacity-60" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<!-- Aligned to the button's own edge rather than centred on it, since
	     the button sits at the left-hand end of a strip that starts at the
	     edge of the screen. -->
	<Popover.Content class="w-44 p-1" align="start" aria-label="Stroke width">
		<div class="flex flex-col gap-1">
			{#each STROKE_WIDTH_CHOICES as choice (choice.value)}
				<button
					type="button"
					aria-label="{choice.label} stroke"
					aria-pressed={strokeWidth === choice.value}
					class="flex items-center gap-3 rounded-sm px-2 py-2 text-sm hover:bg-accent"
					onclick={() => pick(choice.value)}
				>
					<!-- Wider here than on the trigger: the panel has the room, and
					     the bar is the whole of what tells one row from another. -->
					<span class="w-10 rounded-full bg-foreground" style="height: {choice.bar}px"></span>
					<span>{choice.label}</span>
					{#if strokeWidth === choice.value}
						<!-- The tick, not a highlighted row: a highlight is what hover
						     already does, and the two reading the same is how you end
						     up pointing at the wrong one. -->
						<Check class="ml-auto h-4 w-4" />
					{/if}
				</button>
			{/each}
		</div>
	</Popover.Content>
</Popover.Root>
