<script lang="ts">
	// The room's library as a searchable grid. One component for both
	// places it appears — the assets page and the pick-from-library step
	// of the scene and token dialogs — so search can't land in one and not
	// the other, which is what the backlog item warned about.
	import { assetUrl, type Asset } from '$lib/api';
	import { filterAssets } from '$lib/asset-search';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import type { Snippet } from 'svelte';

	let {
		assets,
		loading = false,
		selectedId = $bindable(null),
		selectable = true,
		idPrefix = 'asset',
		emptyHint = 'Nothing in the library yet.',
		columnsClass = 'grid-cols-3 sm:grid-cols-4',
		maxHeightClass = 'max-h-64',
		itemActions
	}: {
		assets: Asset[];
		loading?: boolean;
		/** The chosen asset, or null for "no image". Ignored when not selectable. */
		selectedId?: string | null;
		/** False on the assets page, where clicking a tile picks nothing. */
		selectable?: boolean;
		/** Distinguishes the input ids when two libraries share a page. */
		idPrefix?: string;
		emptyHint?: string;
		columnsClass?: string;
		maxHeightClass?: string;
		/** Rendered under each tile — "Edit" on the assets page, nothing in the pickers. */
		itemActions?: Snippet<[Asset]>;
	} = $props();

	let query = $state('');
	const matches = $derived(filterAssets(assets, query));

	function toggle(id: string) {
		if (!selectable) return;
		selectedId = selectedId === id ? null : id;
	}
</script>

{#snippet tile(asset: Asset)}
	<img
		src={assetUrl(asset.id)}
		alt={asset.name}
		loading="lazy"
		class="h-16 w-full rounded object-cover"
	/>
	<span class="truncate text-xs" title={asset.name}>{asset.name}</span>
	{#if asset.attribution}
		<span class="truncate text-[10px] text-muted-foreground">{asset.attribution}</span>
	{/if}
	{#if asset.gridSize}
		<span class="truncate text-[10px] text-muted-foreground">{asset.gridSize}px grid</span>
	{/if}
{/snippet}

<div class="flex flex-col gap-2">
	<!-- Hidden while the library is empty: a search field over nothing is
	     just a control that can't do anything. -->
	{#if assets.length > 0}
		<div class="flex flex-col gap-1">
			<Label class="sr-only" for="{idPrefix}-search">Search the library</Label>
			<Input
				id="{idPrefix}-search"
				type="search"
				bind:value={query}
				placeholder="Search by name or credit"
				autocomplete="off"
			/>
		</div>
	{/if}

	{#if loading}
		<p class="text-sm text-muted-foreground">Loading library…</p>
	{:else if assets.length === 0}
		<p class="text-sm text-muted-foreground">{emptyHint}</p>
	{:else if matches.length === 0}
		<!-- Distinct from the empty-library hint on purpose: "you have
		     nothing" and "you have things, none of them this" are different
		     problems with different next steps. -->
		<p class="text-sm text-muted-foreground">
			Nothing matches “{query}”.
			<button type="button" class="underline underline-offset-2" onclick={() => (query = '')}
				>Clear the search</button
			>
			to see all {assets.length}.
		</p>
	{:else}
		<ul class="grid {columnsClass} {maxHeightClass} gap-2 overflow-y-auto">
			{#each matches as asset (asset.id)}
				<li class="flex flex-col gap-1">
					<!-- A real <button> only where clicking does something. On the
					     assets page the tile isn't a control, and dressing a div up
					     with role="button" would promise a keyboard interaction that
					     doesn't exist. -->
					{#if selectable}
						<button
							type="button"
							aria-pressed={selectedId === asset.id}
							title={asset.attribution ? `${asset.name} — ${asset.attribution}` : asset.name}
							class={[
								'flex w-full flex-col gap-1 rounded-md border p-1 text-left',
								selectedId === asset.id && 'outline-2 outline-offset-2 outline-sky-400'
							]}
							onclick={() => toggle(asset.id)}
						>
							{@render tile(asset)}
						</button>
					{:else}
						<div class="flex w-full flex-col gap-1 rounded-md border p-1 text-left">
							{@render tile(asset)}
						</div>
					{/if}
					{#if itemActions}{@render itemActions(asset)}{/if}
				</li>
			{/each}
		</ul>
	{/if}
</div>
