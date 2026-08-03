<script lang="ts">
	// Choosing art for a scene or a token, from what the room already has.
	//
	// This used to upload as well. It doesn't any more: an upload made from
	// inside a form about something else had nowhere to put a name or a grid
	// alignment, so it quietly produced assets that search couldn't find and
	// maps whose squares didn't line up. Adding to the library happens on
	// the assets page, which is linked from here.
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { resolve } from '$app/paths';
	import { listRoomAssets, type Asset, type AssetKind } from '$lib/api';
	import AssetLibrary from '$lib/components/asset-library.svelte';

	let {
		roomSlug,
		sessionToken,
		selectedId = $bindable(null),
		kind = 'token',
		idPrefix = 'asset',
		emptyHint = 'Nothing in the library yet — add an image on the assets page.',
		onpick
	}: {
		roomSlug: string;
		sessionToken: string;
		/**
		 * The chosen asset, or null for "no image". Bindable so the
		 * surrounding dialog can read it on submit and reset it afterwards.
		 */
		selectedId?: string | null;
		/**
		 * The tab to open on — the kind this picker is actually asking for.
		 * Only the starting tab, deliberately: a scene wants a map and a
		 * token wants token art, but a room that filed something under the
		 * other kind (or had it guessed for them by the migration) must
		 * still be able to reach it without a trip to the assets page.
		 */
		kind?: AssetKind;
		/** Distinguishes the input ids when two pickers share a page. */
		idPrefix?: string;
		emptyHint?: string;
		/**
		 * The whole chosen asset, for callers that want more than its id —
		 * the scene dialog defaults its grid size from the map's measured
		 * squares this way.
		 */
		onpick?: (asset: Asset | null) => void;
	} = $props();

	let library = $state<Asset[]>([]);
	let loading = $state(true);
	// The open tab, seeded from the prop and owned here afterwards: which
	// half of the library you're looking at is this picker's business
	// while it's open, and no caller wants to hear about it. `untrack`
	// says that in a way the compiler believes — reading a prop in a
	// `$state` initialiser is otherwise assumed to be a mistake.
	let activeKind = $state(untrack(() => kind));

	// Loaded when the picker mounts. Uploads no longer come through here, so
	// this list can go stale if someone adds to the library in another tab —
	// which is what the "add images" link warns about by opening in one.
	$effect(() => {
		void refresh();
	});

	$effect(() => {
		onpick?.(library.find((a) => a.id === selectedId) ?? null);
	});

	async function refresh() {
		loading = true;
		try {
			library = await listRoomAssets(roomSlug, sessionToken);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to load the asset library');
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex flex-col gap-2">
	<AssetLibrary
		assets={library}
		{loading}
		{idPrefix}
		{emptyHint}
		bind:kind={activeKind}
		bind:selectedId
	/>

	<div class="flex items-center gap-3 text-xs text-muted-foreground">
		<a
			class="underline underline-offset-2"
			href={resolve('/r/[slug]/assets', { slug: roomSlug })}
			target="_blank"
			rel="noopener"
		>
			Add images
		</a>
		{#if selectedId}
			<button
				type="button"
				class="underline underline-offset-2"
				onclick={() => (selectedId = null)}
			>
				Clear selection
			</button>
		{/if}
	</div>
</div>
