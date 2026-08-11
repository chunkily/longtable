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
	import { setMode, userPrefersMode } from 'mode-watcher';
	import { Button } from '$lib/components/ui/button';

	let {
		/** Shown above the buttons. Left off where the surrounding UI already says it. */
		label = 'Theme'
	}: { label?: string } = $props();

	// Not imported from mode-watcher: it exports the runtime helpers and
	// the state classes, but not the union of the three values themselves.
	type ThemeChoice = 'system' | 'light' | 'dark';

	const CHOICES: { value: ThemeChoice; label: string }[] = [
		{ value: 'system', label: 'System' },
		{ value: 'light', label: 'Light' },
		{ value: 'dark', label: 'Dark' }
	];
</script>

<!-- Bound to `userPrefersMode` rather than the resolved `mode`: the
     difference between them is the whole point of the System option.
     Marking Dark as selected because the OS happens to be dark would
     leave no way to see which of the two you had actually chosen, and
     no way back once you'd tapped either.

     A labelled group, and every button says its own name — three
     unlabelled sun/moon/monitor icons is exactly the pair-of-icons
     problem the fog controls already hit, with a third icon added. -->
<div class="flex flex-col gap-1">
	<span class="px-1 text-xs text-muted-foreground">{label}</span>
	<div class="grid grid-cols-3 gap-1" role="group" aria-label={label}>
		{#each CHOICES as choice (choice.value)}
			<Button
				variant={userPrefersMode.current === choice.value ? 'default' : 'outline'}
				size="sm"
				aria-pressed={userPrefersMode.current === choice.value}
				onclick={() => setMode(choice.value)}
			>
				{choice.label}
			</Button>
		{/each}
	</div>
</div>
