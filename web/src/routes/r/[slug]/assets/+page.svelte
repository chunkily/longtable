<script lang="ts">
	// The room's asset library, and the only place assets are added to it.
	//
	// Preparing art and playing with it are different jobs: naming, crediting
	// and aligning a map wants room and attention, and the scene dialog it
	// used to live in is a form about something else. So the pickers in the
	// room only pick, and everything arrives through here — which also means
	// every asset gets a name, and every map gets the chance to be aligned,
	// rather than that depending on which route someone happened to take.
	//
	// The whole page is tabbed by kind, not just the library grid: the tab
	// decides what an upload will be *before* the file dialog opens, so
	// nobody discovers they filed six maps as token art after the fact. A
	// per-file toggle would have been fewer moving parts and was tried
	// first; it puts the decision after the interesting part is over, which
	// is exactly when it gets skipped.
	import { onDestroy, onMount, untrack } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import {
		listRoomAssets,
		removeAsset,
		updateAsset,
		uploadAsset,
		type Asset,
		type AssetKind,
		type Session
	} from '$lib/api';
	import { guessAssetKind, measureImage } from '$lib/asset-kind';
	import { paddingForOrigin } from '$lib/grid-align';
	import { loadSession } from '$lib/session';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import * as Dialog from '$lib/components/ui/dialog';
	import AssetKindTabs from '$lib/components/asset-kind-tabs.svelte';
	import AssetLibrary from '$lib/components/asset-library.svelte';
	import GridAligner from '$lib/components/grid-aligner.svelte';

	const slug = $derived(page.params.slug ?? '');

	let session = $state<Session | null>(null);
	let library = $state<Asset[]>([]);
	let loading = $state(true);
	let fileInput = $state<HTMLInputElement | null>(null);

	/**
	 * A file chosen but not yet sent. Everything on it is local until "Add
	 * to library" — there is no draft row on the server, so abandoning the
	 * page leaves nothing behind to clean up.
	 */
	interface Staged {
		id: number;
		file: File;
		/** Object URL for the preview and the aligner. Revoked when this goes. */
		url: string;
		name: string;
		attribution: string;
		/**
		 * Which half of the library this is headed for — taken from the tab
		 * that was open when the file was chosen, so it's a decision someone
		 * made before picking rather than a default they have to notice.
		 */
		kind: AssetKind;
		/**
		 * The image's own dimensions, once they've been read, and what its
		 * shape suggests it is. Only ever used to *ask* whether the tab was
		 * right — see `mismatched`. Null while the measurement is in flight,
		 * and for an image that couldn't be measured at all.
		 */
		shape: { width: number; height: number; suggests: AssetKind } | null;
		aligning: boolean;
		gridSize: number;
		originX: number;
		originY: number;
		uploading: boolean;
	}

	/**
	 * A staged file whose shape disagrees with the tab it was added under.
	 * Worth a word, because it's the one case where the up-front choice is
	 * likely to have been the wrong one — a 1400x900 image staged as token
	 * art usually means someone forgot to switch tabs.
	 */
	function mismatched(item: Staged): boolean {
		return item.shape !== null && item.shape.suggests !== item.kind;
	}

	let staged = $state<Staged[]>([]);
	let nextStagedId = 0;

	// The dialog owns its open flag, with `editing` carrying only which
	// asset is being edited — the same split the token and scene dialogs
	// use. Deriving `open` from `editing !== null` and passing it as a
	// plain prop leaves Dialog.Root uncontrolled on the way out, which is
	// a class of bug worth not inviting.
	let editOpen = $state(false);
	let editing = $state<Asset | null>(null);
	let editName = $state('');
	let editAttribution = $state('');
	let editKind = $state<AssetKind>('token');
	let savingEdit = $state(false);

	/**
	 * The open tab, which governs the whole page: what the library grid
	 * shows, and what anything added from here is going to be. It follows
	 * whatever was just added or reclassified, so a change never looks
	 * like nothing happened because the grid on screen was the other one.
	 *
	 * Seeded from `?kind=`, which is how the pickers in the room link
	 * straight to the half they're asking for — a GM who followed "Add
	 * maps" out of the scene dialog would otherwise land on Tokens and
	 * file their map there. Read once rather than derived from the URL:
	 * once the page is up the tab belongs to whoever is looking at it, and
	 * a derived one would snap back the moment they switched. The
	 * consequence is a query string that can go stale, which costs
	 * nothing until a reload and isn't worth a history entry per click.
	 */
	let activeKind = $state<AssetKind>(
		untrack(() => (page.url.searchParams.get('kind') === 'map' ? 'map' : 'token'))
	);
	const counts = $derived({
		token: library.filter((a) => a.kind === 'token').length,
		map: library.filter((a) => a.kind === 'map').length
	});

	/** The asset whose Remove button has been pressed once. */
	let removingId = $state<string | null>(null);
	let removing = $state(false);

	onMount(async () => {
		session = loadSession(slug);
		if (!session) {
			loading = false;
			return;
		}
		await refresh();
	});

	// Object URLs outlive the component unless they're revoked, and a room
	// member who stages a few 20MB maps and navigates away would otherwise
	// leave them all held.
	onDestroy(() => {
		for (const item of staged) URL.revokeObjectURL(item.url);
	});

	async function refresh() {
		if (!session) return;
		loading = true;
		try {
			library = await listRoomAssets(slug, session.sessionToken);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to load the asset library');
		} finally {
			loading = false;
		}
	}

	function stageFiles(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		for (const file of Array.from(input.files ?? [])) {
			const item: Staged = {
				id: nextStagedId++,
				file,
				url: URL.createObjectURL(file),
				name: defaultName(file.name),
				attribution: '',
				kind: activeKind,
				shape: null,
				aligning: false,
				gridSize: 70,
				originX: 0,
				originY: 0,
				uploading: false
			};
			staged.push(item);
			void measure(item);
		}
		// Clearing the input means picking the same file again still fires a
		// change event, which matters after removing one by mistake.
		input.value = '';
	}

	/**
	 * Reads a staged image's dimensions in the background.
	 *
	 * It writes to `staged` rather than to the local `item`, because by the
	 * time this resolves the card may have been discarded — and pushing a
	 * measurement onto an object nobody is rendering is how you end up
	 * revoking an object URL and then measuring it.
	 */
	async function measure(item: Staged) {
		const size = await measureImage(item.url);
		if (!size) return;
		const live = staged.find((s) => s.id === item.id);
		if (!live) return;
		live.shape = { ...size, suggests: guessAssetKind(size.width, size.height) };
	}

	/** The filename minus its extension — a real editable starting value. */
	function defaultName(filename: string): string {
		const base = filename.replace(/^.*[\\/]/, '');
		const withoutExt = base.replace(/\.[^.]+$/, '');
		return withoutExt || base || 'Untitled';
	}

	function discard(item: Staged) {
		URL.revokeObjectURL(item.url);
		staged = staged.filter((s) => s.id !== item.id);
	}

	async function add(item: Staged) {
		if (!session) return;
		if (!item.name.trim()) {
			toast.error('Give it a name first — it’s what search looks at.');
			return;
		}

		item.uploading = true;
		try {
			// Only a map that was actually aligned carries grid figures; token
			// art has no grid to measure, and `aligning` can only be on for a
			// map, but reading both makes that a fact of this call rather
			// than of the toggle's bookkeeping.
			const aligned = item.kind === 'map' && item.aligning;
			const asset = await uploadAsset(slug, session.sessionToken, item.file, {
				name: item.name,
				attribution: item.attribution,
				kind: item.kind,
				gridSize: aligned ? item.gridSize : null,
				gridOffsetX: aligned ? paddingForOrigin(item.gridSize, item.originX) : 0,
				gridOffsetY: aligned ? paddingForOrigin(item.gridSize, item.originY) : 0
			});

			// Front of the list, matching the server's newest-first order
			// without a second round trip. Deduped by ID because re-adding a
			// file the room already had resolves to the same asset.
			library = [asset, ...library.filter((a) => a.id !== asset.id)];
			activeKind = asset.kind;
			discard(item);
			if (asset.flattened) {
				toast.info('Animated images are stored as a still picture — kept the first frame.');
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'upload failed');
		} finally {
			item.uploading = false;
		}
	}

	/**
	 * Accepts the shape's suggestion for a staged file. Alignment is a
	 * map's business, so it goes off on the way to being token art rather
	 * than lurking switched-on behind a hidden control.
	 */
	function switchKind(item: Staged) {
		if (!item.shape) return;
		item.kind = item.shape.suggests;
		if (item.kind !== 'map') item.aligning = false;
	}

	/**
	 * Takes an asset off this room's shelf. Two clicks, like deleting a
	 * scene — though a lighter kind of loss than that one, since the file
	 * itself survives and adding it again brings it straight back.
	 */
	async function remove(asset: Asset) {
		if (!session) return;

		removing = true;
		try {
			await removeAsset(slug, session.sessionToken, asset.id);
			library = library.filter((a) => a.id !== asset.id);
			removingId = null;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to remove the asset');
		} finally {
			removing = false;
		}
	}

	function startEditing(asset: Asset) {
		editing = asset;
		editName = asset.name;
		editAttribution = asset.attribution;
		editKind = asset.kind;
		editOpen = true;
	}

	async function saveEdit(event: SubmitEvent) {
		event.preventDefault();
		if (!session || !editing) return;

		savingEdit = true;
		try {
			const updated = await updateAsset(slug, session.sessionToken, editing.id, {
				name: editName,
				attribution: editAttribution,
				kind: editKind
			});
			library = library.map((a) => (a.id === updated.id ? updated : a));
			// Follow a reclassified asset across, so the tab you're looking at
			// is the one holding the thing you just edited.
			activeKind = updated.kind;
			editOpen = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to save the change');
		} finally {
			savingEdit = false;
		}
	}
</script>

<svelte:head><title>Assets — {session?.roomName ?? slug}</title></svelte:head>

<div class="mx-auto flex max-w-4xl flex-col gap-6 p-6">
	<header class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
		<h1 class="text-2xl font-bold tracking-tight">Assets</h1>
		<span class="text-sm text-muted-foreground">{session?.roomName ?? slug}</span>
		<span class="flex-1"></span>
		<a class="text-sm underline underline-offset-2" href={resolve('/r/[slug]', { slug })}>
			Back to the table
		</a>
	</header>

	{#if !session && !loading}
		<!-- Membership is a session token for this room, so there is nothing
		     to show and nothing to upload with until they've joined. -->
		<Card.Root>
			<Card.Header>
				<Card.Title>Join the room first</Card.Title>
				<Card.Description>
					A room's library is only visible to people in it. Join at the table, then come back.
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<Button href={resolve('/r/[slug]', { slug })} variant="outline">Go to the room</Button>
			</Card.Content>
		</Card.Root>
	{:else}
		<!-- The switch sits above both cards because it governs both: the
		     grid it filters, and the kind an upload is going to be. Choosing
		     before the file dialog opens is the whole point — a control
		     underneath the file picker is one you meet after the decision
		     has effectively been made. -->
		<AssetKindTabs bind:kind={activeKind} {counts} controls="library-grid" label="Assets" />

		<Card.Root>
			<Card.Header>
				<Card.Title>
					{activeKind === 'map' ? 'Add maps' : 'Add token art'}
				</Card.Title>
				<Card.Description>
					{#if activeKind === 'map'}
						Anything added here is filed as a map, and can be aligned to the grid on the way in.
						Switch to <strong>Tokens</strong> for art that goes on a token.
					{:else}
						Anything added here is filed as token art. Switch to <strong>Maps</strong> for a battle map,
						which also lets you align it to the grid.
					{/if}
					Everything is stored as WebP, credited to whoever you say, and available to the whole room.
				</Card.Description>
			</Card.Header>
			<Card.Content class="flex flex-col gap-4">
				<div>
					<Button type="button" variant="outline" onclick={() => fileInput?.click()}>
						{activeKind === 'map' ? 'Choose maps' : 'Choose token art'}
					</Button>
					<!-- Hidden because the native control can't be styled to match,
					     and it's driven by the button above. -->
					<input
						bind:this={fileInput}
						type="file"
						multiple
						accept="image/png,image/jpeg,image/webp,image/gif"
						class="hidden"
						aria-label="Choose images to add"
						onchange={stageFiles}
					/>
				</div>

				{#each staged as item (item.id)}
					<div class="flex flex-col gap-3 rounded-md border p-3">
						<div class="flex gap-3">
							<!-- Shaped like the tile it's about to become, so the choice
							     of kind shows its consequence before it's committed. -->
							<img
								src={item.url}
								alt=""
								class={[
									'shrink-0 rounded border',
									item.kind === 'map' ? 'h-20 w-28 object-cover' : 'size-20 object-contain'
								]}
							/>
							<div class="flex min-w-0 flex-1 flex-col gap-2">
								<div class="flex flex-col gap-1">
									<Label for="staged-{item.id}-name">Name</Label>
									<Input id="staged-{item.id}-name" bind:value={item.name} autocomplete="off" />
								</div>
								<div class="flex flex-col gap-1">
									<Label for="staged-{item.id}-attribution">Attribution or licence (optional)</Label
									>
									<Input
										id="staged-{item.id}-attribution"
										bind:value={item.attribution}
										placeholder="by Alice, CC-BY"
										autocomplete="off"
									/>
								</div>
							</div>
						</div>

						{#if item.kind === 'map' && item.aligning}
							<GridAligner
								src={item.url}
								idPrefix="staged-{item.id}"
								bind:gridSize={item.gridSize}
								bind:originX={item.originX}
								bind:originY={item.originY}
							/>
						{/if}

						{#if mismatched(item)}
							<!-- The shape is only ever a question, never an answer: it
							     can't override a choice someone made on purpose, and it
							     shows the dimensions it's arguing from so the guess can
							     be judged rather than trusted. Wrong sometimes — square
							     maps and long banners both exist — which is exactly why
							     it asks. -->
							<p
								class="flex flex-wrap items-center gap-2 rounded-md border border-amber-500/40 bg-amber-500/10 p-2 text-sm"
							>
								<span>
									{item.shape?.width}×{item.shape?.height} is shaped more like
									{item.shape?.suggests === 'map' ? 'a map' : 'token art'}.
								</span>
								<button
									type="button"
									class="underline underline-offset-2"
									onclick={() => switchKind(item)}
								>
									File it as {item.shape?.suggests === 'map' ? 'a map' : 'token art'}
								</button>
							</p>
						{/if}

						<div class="flex flex-wrap items-center gap-2">
							<!-- Stated rather than asked: the tab already decided, and
							     repeating the question here is what put people off in the
							     first place. -->
							<span class="text-sm text-muted-foreground">
								Adding as {item.kind === 'map' ? 'a map' : 'token art'}
							</span>
							{#if item.kind === 'map'}
								<Button
									type="button"
									variant={item.aligning ? 'secondary' : 'outline'}
									size="sm"
									onclick={() => (item.aligning = !item.aligning)}
								>
									{item.aligning ? 'Skip alignment' : 'Align to grid'}
								</Button>
								{#if item.aligning}
									<span class="text-xs text-muted-foreground">
										{item.gridSize}px squares
									</span>
								{/if}
							{/if}
							<span class="flex-1"></span>
							<!-- "Discard", not "Remove": the library tiles below have a
							     Remove of their own now, and the two undo very different
							     amounts of work. -->
							<Button type="button" variant="ghost" size="sm" onclick={() => discard(item)}>
								Discard
							</Button>
							<Button type="button" size="sm" disabled={item.uploading} onclick={() => add(item)}>
								{item.uploading ? 'Adding…' : 'Add to library'}
							</Button>
						</div>
					</div>
				{/each}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title>In this room</Card.Title>
				<Card.Description>
					{library.length}
					{library.length === 1 ? 'image' : 'images'}, shared with everyone at the table.
				</Card.Description>
			</Card.Header>
			<Card.Content>
				<!-- `kind` is bound even though the visible control is the tab
				     strip at the top of the page: the grid's own empty states
				     offer to look in the other half, and an unbound prop would
				     have those switching the grid while the tabs went on
				     claiming otherwise. -->
				<AssetLibrary
					assets={library}
					{loading}
					bind:kind={activeKind}
					showTabs={false}
					selectable={false}
					idPrefix="library"
					columnsClass="grid-cols-3 sm:grid-cols-5"
					maxHeightClass="max-h-none"
					emptyHint="Nothing here yet — add an image above and it'll be ready to use at the table."
				>
					{#snippet itemActions(asset)}
						{#if removingId === asset.id}
							<!-- Two clicks, and the second one says what it's about to
							     do — the pattern scene deletion uses. Lighter than that
							     one, though: the file survives and adding it again puts
							     it back, which is why the copy says "remove" and not
							     "delete". -->
							<div class="flex flex-col gap-1">
								<p class="text-[10px] text-muted-foreground">Anything already using it keeps it.</p>
								<div class="flex gap-1">
									<Button
										type="button"
										variant="destructive"
										size="sm"
										class="h-7 flex-1 px-2 text-xs"
										disabled={removing}
										aria-label="Confirm removing {asset.name}"
										onclick={() => remove(asset)}
									>
										Remove
									</Button>
									<Button
										type="button"
										variant="ghost"
										size="sm"
										class="h-7 px-2 text-xs"
										onclick={() => (removingId = null)}
									>
										Cancel
									</Button>
								</div>
							</div>
						{:else}
							<div class="flex gap-1">
								<Button
									type="button"
									variant="ghost"
									size="sm"
									class="h-7 flex-1 justify-start px-2 text-xs"
									onclick={() => startEditing(asset)}
								>
									Edit
								</Button>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									class="h-7 px-2 text-xs text-muted-foreground"
									aria-label="Remove {asset.name}"
									onclick={() => (removingId = asset.id)}
								>
									Remove
								</Button>
							</div>
						{/if}
					{/snippet}
				</AssetLibrary>
			</Card.Content>
		</Card.Root>
	{/if}
</div>

<Dialog.Root bind:open={editOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Edit asset</Dialog.Title>
			<Dialog.Description>
				The picture and its grid can't change — those are the file itself. Add it again to replace
				them.
			</Dialog.Description>
		</Dialog.Header>
		<form class="flex flex-col gap-4" onsubmit={saveEdit}>
			<div class="flex flex-col gap-2">
				<Label for="edit-asset-name">Name</Label>
				<Input id="edit-asset-name" bind:value={editName} required autocomplete="off" />
			</div>
			<div class="flex flex-col gap-2">
				<Label for="edit-asset-attribution">Attribution or licence</Label>
				<Input
					id="edit-asset-attribution"
					bind:value={editAttribution}
					placeholder="by Alice, CC-BY"
					autocomplete="off"
				/>
			</div>
			<div class="flex flex-col gap-2">
				<!-- Unlike the pixels, what a room keeps a picture *for* is a
				     decision, and decisions get made wrong. This is also the
				     only way to fix what the migration guessed for a library
				     that predates the split. -->
				<Label>Filed under</Label>
				<div class="flex gap-2">
					<Button
						type="button"
						variant={editKind === 'token' ? 'default' : 'outline'}
						aria-pressed={editKind === 'token'}
						onclick={() => (editKind = 'token')}
						class="flex-1"
					>
						Tokens
					</Button>
					<Button
						type="button"
						variant={editKind === 'map' ? 'default' : 'outline'}
						aria-pressed={editKind === 'map'}
						onclick={() => (editKind = 'map')}
						class="flex-1"
					>
						Maps
					</Button>
				</div>
			</div>
			<Dialog.Footer>
				<Button type="submit" disabled={savingEdit}>
					{savingEdit ? 'Saving…' : 'Save'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
