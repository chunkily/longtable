<script lang="ts">
	// The room's library as a searchable, tabbed grid. One component for
	// both places it appears — the assets page and the pick-from-library
	// step of the scene and token dialogs — so search and the token/map
	// split can't land in one and not the other, which is what the backlog
	// item warned about.
	import { assetUrl, type Asset, type AssetKind } from '$lib/api';
	import { filterAssets } from '$lib/asset-search';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import AssetKindTabs from '$lib/components/asset-kind-tabs.svelte';
	import type { Snippet } from 'svelte';

	let {
		assets,
		loading = false,
		selectedId = $bindable(null),
		kind = $bindable('token'),
		showTabs = true,
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
		/**
		 * Which tab is open. Bindable so the assets page can follow a
		 * newly added asset into its half of the library, and so a picker
		 * can open on the kind it's actually asking for.
		 */
		kind?: AssetKind;
		/**
		 * False on the assets page, which renders the same switch at the top
		 * of the page instead — there it governs the upload card as well as
		 * this grid, so it can't live inside the grid.
		 */
		showTabs?: boolean;
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

	const tokens = $derived(assets.filter((a) => a.kind === 'token'));
	const maps = $derived(assets.filter((a) => a.kind === 'map'));
	const inTab = $derived(kind === 'map' ? maps : tokens);
	const matches = $derived(filterAssets(inTab, query));
	// What the same search would have found in the other tab. A picture
	// filed under the wrong kind is the one way this split can lose
	// something, so the empty state points across rather than leaving
	// someone to conclude the room doesn't have it.
	const matchesElsewhere = $derived(filterAssets(kind === 'map' ? tokens : maps, query).length);

	const counts = $derived({ token: tokens.length, map: maps.length });
	const otherLabel = $derived(kind === 'map' ? 'Tokens' : 'Maps');

	function toggle(id: string) {
		if (!selectable) return;
		selectedId = selectedId === id ? null : id;
	}

	function showOtherKind() {
		kind = kind === 'map' ? 'token' : 'map';
	}
</script>

{#snippet tile(asset: Asset)}
	<!-- Token art is shown whole in a square, because a token *is* square
	     on the table and a crop hides the thing you're picking. A map is
	     never square and never seen whole at thumbnail size anyway, so it
	     keeps the wide crop, which fits more of the library on screen. -->
	<img
		src={assetUrl(asset.id)}
		alt={asset.name}
		loading="lazy"
		class={[
			'w-full rounded',
			asset.kind === 'map' ? 'h-16 object-cover' : 'aspect-square object-contain'
		]}
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
	{#if showTabs}
		<AssetKindTabs bind:kind {counts} controls="{idPrefix}-library-grid" />
	{/if}

	<!-- Hidden while the library is empty: a search field over nothing is
	     just a control that can't do anything. Not per-tab, though — with
	     one tab full and the other empty, a field that came and went as
	     you switched would read as a bug. -->
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

	<div id="{idPrefix}-library-grid" role="tabpanel">
		{#if loading}
			<p class="text-sm text-muted-foreground">Loading library…</p>
		{:else if assets.length === 0}
			<p class="text-sm text-muted-foreground">{emptyHint}</p>
		{:else if inTab.length === 0}
			<!-- Three empty states, not one: "you have nothing", "you have
			     nothing of this kind", and "you have things of this kind, none
			     of them this" are different problems with different next
			     steps. -->
			<p class="text-sm text-muted-foreground">
				Nothing filed as {kind === 'map' ? 'a map' : 'token art'} yet.
				<button type="button" class="underline underline-offset-2" onclick={showOtherKind}>
					Look in {otherLabel}
				</button>
				instead.
			</p>
		{:else if matches.length === 0}
			<p class="text-sm text-muted-foreground">
				Nothing here matches “{query}”.
				{#if matchesElsewhere > 0}
					<button type="button" class="underline underline-offset-2" onclick={showOtherKind}>
						{matchesElsewhere}
						{matchesElsewhere === 1 ? 'match' : 'matches'} in {otherLabel}
					</button>.
				{:else}
					<button type="button" class="underline underline-offset-2" onclick={() => (query = '')}
						>Clear the search</button
					>
					to see all {inTab.length}.
				{/if}
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
</div>
