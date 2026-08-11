<script lang="ts">
	// Light, dark, or whatever the device is already doing.
	//
	// Three options rather than a switch. "System" is an answer, not the
	// absence of one — a laptop that goes dark at sunset is doing what its
	// owner asked for — and a two-state switch has nowhere to put it, so
	// the first tap of one costs you the setting permanently.
	//
	// Nothing here touches the room. The choice is this browser's, stored
	// beside its sessions and never sent anywhere, so two people at the
	// same table can read the same map in opposite schemes.
	import MonitorIcon from '@lucide/svelte/icons/monitor';
	import SunIcon from '@lucide/svelte/icons/sun';
	import MoonIcon from '@lucide/svelte/icons/moon';
	import { setMode, userPrefersMode } from 'mode-watcher';
	import { Button } from '$lib/components/ui/button';

	let {
		/**
		 * `menu` is a labelled row for the room menu; `floating` is the
		 * bare pill that sits in the home page's corner.
		 *
		 * One component with a shape prop rather than two, because the
		 * three buttons and the state behind them are the same in both —
		 * and a second copy is how the readonly/editable pair of tracker
		 * boxes nearly drifted apart once already.
		 */
		variant = 'menu'
	}: { variant?: 'menu' | 'floating' } = $props();

	// Not imported from mode-watcher: it exports the runtime helpers and
	// the state classes, but not the union of the three values themselves.
	type ThemeChoice = 'system' | 'light' | 'dark';

	// Icons, not words, and the group label carries the naming instead.
	//
	// Sun and moon are about as widely understood as icons get. The
	// monitor for "follow my device" is a learned convention rather than
	// an obvious one, which is why `Theme` stays beside the row and every
	// button keeps a `title` and an `aria-label` — those names are also
	// what the e2e specs match on.
	//
	// Unlabelled icons were argued against here originally, on the
	// strength of the fog controls: two icons meaning cover-everything and
	// uncover-everything, which get mis-hit. That reasoning doesn't
	// transfer. Those are destructive bulk actions you might not notice
	// going wrong; a mis-hit here repaints the whole screen and the fix is
	// the button next to your finger.
	const CHOICES: { value: ThemeChoice; label: string; icon: typeof MonitorIcon }[] = [
		{ value: 'system', label: 'System', icon: MonitorIcon },
		{ value: 'light', label: 'Light', icon: SunIcon },
		{ value: 'dark', label: 'Dark', icon: MoonIcon }
	];
</script>

<!-- Bound to `userPrefersMode` rather than the resolved `mode`: the
     difference between them is the whole point of the System option.
     Marking Dark as selected because the OS happens to be dark would
     leave no way to see which of the two you had actually chosen, and
     no way back once you'd tapped either. -->
{#snippet choices(round: boolean)}
	{#each CHOICES as choice (choice.value)}
		<Button
			variant={userPrefersMode.current === choice.value ? 'default' : 'ghost'}
			size="icon-sm"
			class={round ? 'rounded-full' : undefined}
			aria-label={choice.label}
			aria-pressed={userPrefersMode.current === choice.value}
			title={choice.label}
			onclick={() => setMode(choice.value)}
		>
			<choice.icon class="h-4 w-4" />
		</Button>
	{/each}
{/snippet}

{#if variant === 'floating'}
	<!-- A pill, so it reads as one control resting on the page rather
	     than as three buttons someone left in the corner. Positioning is
	     the page's business, not this component's. -->
	<div
		class="flex items-center gap-1 rounded-full border bg-popover p-1 shadow-md"
		role="group"
		aria-label="Theme"
	>
		{@render choices(true)}
	</div>
{:else}
	<div class="flex items-center justify-between gap-2 px-1">
		<span class="text-xs text-muted-foreground">Theme</span>
		<div class="flex items-center gap-1" role="group" aria-label="Theme">
			{@render choices(false)}
		</div>
	</div>
{/if}
