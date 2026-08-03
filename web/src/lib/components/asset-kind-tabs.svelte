<script lang="ts">
	// The Tokens/Maps switch. Its own component because it governs
	// different amounts of page in the two places it appears: inside the
	// library grid in the pickers, where it filters a list, and at the top
	// of the assets page, where it also decides what an upload is going to
	// be. Same control, so those can't drift apart into two switches that
	// look alike and mean different things.
	import type { AssetKind } from '$lib/api';

	let {
		kind = $bindable('token'),
		counts,
		controls,
		label = 'Library'
	}: {
		kind?: AssetKind;
		/** Shown beside each label, so an empty half is visibly empty. */
		counts: { token: number; map: number };
		/** id of the region this switches, for `aria-controls`. */
		controls: string;
		label?: string;
	} = $props();

	const tabs: { value: AssetKind; label: string }[] = [
		{ value: 'token', label: 'Tokens' },
		{ value: 'map', label: 'Maps' }
	];
</script>

<!-- Always both tabs, even when one is empty: they're the shape of the
     library, and a tab that appears only once it has something in it
     can't tell you where the thing you're missing should have gone. -->
<div class="flex gap-1 border-b" role="tablist" aria-label={label}>
	{#each tabs as tab (tab.value)}
		<button
			type="button"
			role="tab"
			aria-selected={kind === tab.value}
			aria-controls={controls}
			class={[
				'-mb-px border-b-2 px-3 py-1.5 text-sm',
				kind === tab.value
					? 'border-foreground font-medium'
					: 'border-transparent text-muted-foreground hover:text-foreground'
			]}
			onclick={() => (kind = tab.value)}
		>
			{tab.label}
			<span class="text-xs text-muted-foreground">{counts[tab.value]}</span>
		</button>
	{/each}
</div>
