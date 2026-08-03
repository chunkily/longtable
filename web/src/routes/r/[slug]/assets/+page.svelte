<script lang="ts">
	// The room's asset library, and the only place assets are added to it.
	//
	// Preparing art and playing with it are different jobs: naming, crediting
	// and aligning a map wants room and attention, and the scene dialog it
	// used to live in is a form about something else. So the pickers in the
	// room only pick, and everything arrives through here — which also means
	// every asset gets a name, and every map gets the chance to be aligned,
	// rather than that depending on which route someone happened to take.
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { listRoomAssets, updateAsset, uploadAsset, type Asset, type Session } from '$lib/api';
	import { paddingForOrigin } from '$lib/grid-align';
	import { loadSession } from '$lib/session';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import * as Dialog from '$lib/components/ui/dialog';
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
		aligning: boolean;
		gridSize: number;
		originX: number;
		originY: number;
		uploading: boolean;
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
	let savingEdit = $state(false);

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
			staged.push({
				id: nextStagedId++,
				file,
				url: URL.createObjectURL(file),
				name: defaultName(file.name),
				attribution: '',
				aligning: false,
				gridSize: 70,
				originX: 0,
				originY: 0,
				uploading: false
			});
		}
		// Clearing the input means picking the same file again still fires a
		// change event, which matters after removing one by mistake.
		input.value = '';
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
			const asset = await uploadAsset(slug, session.sessionToken, item.file, {
				name: item.name,
				attribution: item.attribution,
				// Only a map that was actually aligned carries grid figures;
				// token art has no grid to measure.
				gridSize: item.aligning ? item.gridSize : null,
				gridOffsetX: item.aligning ? paddingForOrigin(item.gridSize, item.originX) : 0,
				gridOffsetY: item.aligning ? paddingForOrigin(item.gridSize, item.originY) : 0
			});

			// Front of the list, matching the server's newest-first order
			// without a second round trip. Deduped by ID because re-adding a
			// file the room already had resolves to the same asset.
			library = [asset, ...library.filter((a) => a.id !== asset.id)];
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

	function startEditing(asset: Asset) {
		editing = asset;
		editName = asset.name;
		editAttribution = asset.attribution;
		editOpen = true;
	}

	async function saveEdit(event: SubmitEvent) {
		event.preventDefault();
		if (!session || !editing) return;

		savingEdit = true;
		try {
			const updated = await updateAsset(slug, session.sessionToken, editing.id, {
				name: editName,
				attribution: editAttribution
			});
			library = library.map((a) => (a.id === updated.id ? updated : a));
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
		<Card.Root>
			<Card.Header>
				<Card.Title>Add images</Card.Title>
				<Card.Description>
					Everything here is stored as WebP, credited to whoever you say, and available to the whole
					room. Maps can be aligned to the grid before they go in.
				</Card.Description>
			</Card.Header>
			<Card.Content class="flex flex-col gap-4">
				<div>
					<Button type="button" variant="outline" onclick={() => fileInput?.click()}>
						Choose images
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
							<img src={item.url} alt="" class="h-20 w-28 shrink-0 rounded border object-cover" />
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

						{#if item.aligning}
							<GridAligner
								src={item.url}
								idPrefix="staged-{item.id}"
								bind:gridSize={item.gridSize}
								bind:originX={item.originX}
								bind:originY={item.originY}
							/>
						{/if}

						<div class="flex flex-wrap items-center gap-2">
							<Button
								type="button"
								variant={item.aligning ? 'secondary' : 'outline'}
								size="sm"
								onclick={() => (item.aligning = !item.aligning)}
							>
								{item.aligning ? 'Not a map' : 'Align to grid'}
							</Button>
							{#if item.aligning}
								<span class="text-xs text-muted-foreground">
									{item.gridSize}px squares
								</span>
							{/if}
							<span class="flex-1"></span>
							<Button type="button" variant="ghost" size="sm" onclick={() => discard(item)}>
								Remove
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
				<AssetLibrary
					assets={library}
					{loading}
					selectable={false}
					idPrefix="library"
					columnsClass="grid-cols-3 sm:grid-cols-5"
					maxHeightClass="max-h-none"
					emptyHint="Nothing here yet — add an image above and it'll be ready to use at the table."
				>
					{#snippet itemActions(asset)}
						<Button
							type="button"
							variant="ghost"
							size="sm"
							class="h-7 justify-start px-2 text-xs"
							onclick={() => startEditing(asset)}
						>
							Edit
						</Button>
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
			<Dialog.Footer>
				<Button type="submit" disabled={savingEdit}>
					{savingEdit ? 'Saving…' : 'Save'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
