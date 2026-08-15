<script lang="ts">
	// The row of colours somebody picks their identity from, on every
	// form that makes a seat: creating a room, taking a new chair, and a
	// GM setting one out before anyone arrives.
	//
	// A fixed palette rather than a colour input, which is the story's
	// first criterion and not an arbitrary limit: these have to stay
	// legible on a light map and a dark one, and stay clear of the
	// colours the canvas already speaks with. A free picker hands someone
	// a black ping on a black battle map.
	//
	// `taken` marks the colours already in the room. It marks rather than
	// disables — two people may be the same colour, and the room does not
	// argue about it. Knowing is the whole ask.
	import { IDENTITY_COLORS } from '$lib/identity-color';

	let {
		value = $bindable(''),
		taken = [],
		label = 'Your colour'
	}: { value?: string; taken?: readonly string[]; label?: string } = $props();
</script>

<div role="radiogroup" aria-label={label} class="flex flex-wrap items-center gap-2">
	{#each IDENTITY_COLORS as colour (colour.key)}
		{@const isTaken = taken.includes(colour.key)}
		<button
			type="button"
			role="radio"
			aria-checked={value === colour.key}
			aria-label={isTaken ? `${colour.label}, taken` : colour.label}
			title={isTaken ? `${colour.label} — someone here has this` : colour.label}
			class={[
				'flex h-7 w-7 items-center justify-center rounded-full',
				value === colour.key && 'outline-2 outline-offset-2 outline-sky-400'
			]}
			style="background-color: {colour.hex}"
			onclick={() => (value = colour.key)}
		>
			<!-- A hole in the middle for one that's spoken for. Not a tick and
			     not a cross: both read as a verdict on the click, and this is
			     only telling you what the room already looks like. -->
			{#if isTaken}
				<span class="h-2.5 w-2.5 rounded-full bg-background/80"></span>
			{/if}
		</button>
	{/each}
</div>
