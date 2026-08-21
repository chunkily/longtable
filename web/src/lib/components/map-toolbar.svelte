<script lang="ts">
	// The tool row: five families, not eleven tools. Whatever the active
	// family needs lives on the contextual strip (tool-strip.svelte),
	// which the room page places separately — floating under this on a
	// desktop, docked into the sheet on a phone.
	import { Button } from '$lib/components/ui/button';
	import type { RoomClient } from '$lib/room.svelte';
	import { familyOf, toolForFamily, type Tool, type ToolFamily } from '$lib/tool-family';
	import type { Snippet } from 'svelte';
	import Hand from '@lucide/svelte/icons/hand';
	import Pen from '@lucide/svelte/icons/pen';
	import Ruler from '@lucide/svelte/icons/ruler';
	import Cloud from '@lucide/svelte/icons/cloud';
	import MapPin from '@lucide/svelte/icons/map-pin';
	import Undo from '@lucide/svelte/icons/undo';
	import Redo from '@lucide/svelte/icons/redo';
	import RefreshCw from '@lucide/svelte/icons/refresh-cw';
	import Grid3x3 from '@lucide/svelte/icons/grid-3x3';

	let {
		room,
		activeTool = $bindable('none'),
		isGM,
		highContrastGrid = $bindable(false),
		onResetView,
		newToken
	}: {
		room: RoomClient;
		activeTool?: Tool;
		isGM: boolean;
		highContrastGrid?: boolean;
		onResetView: () => void;
		newToken?: Snippet;
	} = $props();

	// Derived, never stored: see the note in $lib/tool-family. The
	// highlight on this row is a read of what the canvas is actually
	// doing, so the two can't drift.
	const activeFamily = $derived(familyOf(activeTool));

	// What each family was last left on, so coming back to Draw puts back
	// the shape you were using. A plain object rather than a SvelteMap:
	// nothing reactive reads it — it's only ever consulted inside a click
	// handler, which runs after any state it would depend on has settled.
	const remembered: Partial<Record<ToolFamily, Tool>> = {};

	// Families deliberately don't toggle off. There is an explicit Hand
	// button to stop what you're doing, so a second click on the active
	// family would be a second way to do the same thing — and a
	// surprising one, since it silently takes away the strip you were
	// reaching for.
	function pickFamily(family: ToolFamily) {
		remembered[activeFamily] = activeTool;
		activeTool = toolForFamily(family, remembered[family]);
	}

	const FAMILIES: { value: ToolFamily; label: string; title: string; icon: typeof Hand }[] = [
		{ value: 'hand', label: 'Hand', title: 'Pan the map and select tokens', icon: Hand },
		{ value: 'draw', label: 'Draw', title: 'Draw on the map, and erase', icon: Pen },
		{ value: 'measure', label: 'Measure', title: 'Measure distances and place areas', icon: Ruler },
		{ value: 'ping', label: 'Ping', title: 'Ping the map', icon: MapPin },
		{ value: 'fog', label: 'Fog', title: 'Reveal and hide the map', icon: Cloud }
	];

	// Everyone's tools, then the GM's — a permission split rather than the
	// gesture grouping above, which is why fog moved to the end of
	// FAMILIES instead of sitting between measure and ping. Fog is hidden
	// outright for a Player rather than shown disabled: there's nothing to
	// gain from knowing the row is one shorter for them.
	const playerFamilies = $derived(FAMILIES.filter((f) => f.value !== 'fog'));
	const gmFamilies = $derived(isGM ? FAMILIES.filter((f) => f.value === 'fog') : []);
</script>

{#snippet familyButton(family: (typeof FAMILIES)[number])}
	<Button
		variant={activeFamily === family.value ? 'default' : 'ghost'}
		size="sm"
		aria-label={family.label}
		aria-pressed={activeFamily === family.value}
		title={family.title}
		onclick={() => pickFamily(family.value)}
	>
		<family.icon class="h-4 w-4" />
	</Button>
{/snippet}

<div class="flex flex-wrap items-start gap-2">
	<div class="flex items-center gap-1 rounded-md border bg-background/95 p-1 shadow-sm">
		{#each playerFamilies as family (family.value)}
			{@render familyButton(family)}
		{/each}
		<!-- New token opens a dialog rather than entering a mode, so it
		     isn't a tool and doesn't get a family — but it's available to
		     everyone, so it folds into this cluster rather than earning a
		     divider of its own. -->
		{#if newToken}
			<div class="ml-1 border-l pl-1">{@render newToken()}</div>
		{/if}
		<!-- Everything past this divider is the GM's alone; everything
		     before it is every Room Member's. -->
		{#if gmFamilies.length > 0}
			<div class="ml-1 flex items-center gap-1 border-l pl-1">
				{#each gmFamilies as family (family.value)}
					{@render familyButton(family)}
				{/each}
			</div>
		{/if}
	</div>

	<div class="flex items-center gap-1 rounded-md border bg-background/95 p-1 shadow-sm">
		<Button
			variant="ghost"
			size="sm"
			disabled={!room.canUndo}
			title="Undo your last drawing, erase, token move or deletion (Ctrl+Z)"
			aria-label="Undo"
			onclick={() => room.undo()}
		>
			<Undo class="h-4 w-4" />
		</Button>
		<!-- Redo and reset view drop off this cluster on a phone and turn up
		     in the room menu instead: at 375px the row has to give something
		     back, and undo is the one of the three you reach for mid-game. -->
		<Button
			variant="ghost"
			size="sm"
			class="hidden lg:inline-flex"
			disabled={!room.canRedo}
			title="Redo (Ctrl+Shift+Z)"
			aria-label="Redo"
			onclick={() => room.redo()}
		>
			<Redo class="h-4 w-4" />
		</Button>
		<Button
			variant="ghost"
			size="sm"
			class="hidden lg:inline-flex"
			aria-label="Reset view"
			title="Reset view"
			onclick={onResetView}
		>
			<RefreshCw class="h-4 w-4" />
		</Button>
		<!-- On this cluster rather than in a family, because it isn't a
		     tool: it changes how the map is *shown*, which is what undo and
		     reset view beside it do too. It keeps its place on a phone
		     while redo and reset view drop off — those two have the room
		     menu to fall back on and this has nowhere, and a small screen
		     is where a faint grid is hardest to see anyway. -->
		<Button
			variant={highContrastGrid ? 'default' : 'ghost'}
			size="sm"
			aria-label="Bold grid"
			aria-pressed={highContrastGrid}
			title="Bolder grid lines"
			onclick={() => (highContrastGrid = !highContrastGrid)}
		>
			<Grid3x3 class="h-4 w-4" />
		</Button>
	</div>
</div>
