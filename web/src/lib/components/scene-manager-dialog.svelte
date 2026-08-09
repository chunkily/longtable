<script lang="ts">
	// The room's scenes, and everything a GM can do to one: make it,
	// switch to it, swap its map, throw it away. Before this there was no
	// way to reach a scene other than the one `scene.create`
	// auto-activated, which is why it used to auto-activate at all.
	//
	// Making one used to be a dialog of its own with its own menu entry,
	// which meant the menu asked a question nobody has — "do you want the
	// list of scenes, or a new one?" — before showing you the list that
	// would have answered it. It is a mode of this dialog now, the same
	// one-job-at-a-time swap `Replace map` already uses.
	import { toast } from 'svelte-sonner';
	import { assetUrl, type Asset } from '$lib/api';
	import type { RoomClient } from '$lib/room.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import AssetPicker from '$lib/components/asset-picker.svelte';
	import Plus from '@lucide/svelte/icons/plus';

	// Bindable `open` and an optional trigger, so the room menu can open
	// this without a toolbar button of its own — neither Scenes nor New
	// scene is on the toolbar since the full-bleed layout.
	let {
		room,
		roomSlug,
		sessionToken,
		open = $bindable(false),
		trigger = true
	}: {
		room: RoomClient;
		roomSlug: string;
		sessionToken: string;
		open?: boolean;
		trigger?: boolean;
	} = $props();

	// The scene whose map is being replaced, which swaps the dialog over
	// to the picker — one job on screen at a time, rather than a picker
	// expanding inside a row of a list that also has delete buttons.
	let replacingId = $state<string | null>(null);
	// Whether the dialog is showing the new-scene form instead of the
	// list. Same rule as replacing: one job on screen.
	let creating = $state(false);
	// Delete confirms in place by arming the row's own button. A second
	// dialog can't open over this one, and a scene takes its tokens, fog
	// and drawings with it — too much to lose to a stray click.
	let confirmingDeleteId = $state<string | null>(null);

	let replacementAssetId = $state<string | null>(null);
	let replacementWidth = $state(0);
	let replacementHeight = $state(0);

	const replacing = $derived(room.scenes.find((s) => s.id === replacingId) ?? null);

	// Same reasoning as the create dialog: the new map's real dimensions,
	// read from the image itself rather than trusted from the picker's
	// clipped 64px thumbnail, which is object-covered and would give the
	// wrong aspect ratio for anything not already square.
	$effect(() => {
		const id = replacementAssetId;
		if (!id) {
			replacementWidth = 0;
			replacementHeight = 0;
			return;
		}

		const img = new Image();
		img.onload = () => {
			replacementWidth = img.naturalWidth;
			replacementHeight = img.naturalHeight;
		};
		img.src = assetUrl(id);
	});

	// Reopening the dialog should never land on a half-finished action
	// from last time.
	$effect(() => {
		if (!open) {
			replacingId = null;
			confirmingDeleteId = null;
			replacementAssetId = null;
			creating = false;
		}
	});

	// --- the new-scene form ---

	let name = $state('');
	let gridSize = $state(70);
	let width = $state(1400);
	let height = $state(1000);
	let mapAssetId = $state<string | null>(null);
	let submitting = $state(false);

	// Width/height default to the chosen map's real dimensions. Loading it
	// as a plain Image rather than trusting anything cached from the
	// picker's thumbnail keeps this the actual asset dimensions, not a
	// guess from a clipped 64px preview.
	$effect(() => {
		const id = mapAssetId;
		if (!id) return;

		const img = new Image();
		img.onload = () => {
			width = img.naturalWidth;
			height = img.naturalHeight;
		};
		img.src = assetUrl(id);
	});

	// A map aligned on the assets page carries the square size that was
	// measured while aligning it. Defaulting to it is what makes the
	// alignment worth doing — the offset is already baked into the pixels,
	// but a scene created at the wrong grid size undoes it just as
	// thoroughly as a wrong offset would.
	function adoptGridSize(asset: Asset | null) {
		if (asset?.gridSize) gridSize = asset.gridSize;
	}

	function startCreating() {
		confirmingDeleteId = null;
		replacingId = null;
		creating = true;
	}

	// Closes the whole dialog rather than falling back to the list.
	// Making a scene is nearly always the last thing you came here to do —
	// the room's first one is already on screen behind this — and being
	// returned to a list you then have to dismiss reads as the form having
	// failed. The list is one click away for the times it isn't.
	function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		submitting = true;
		try {
			room.createScene(name, mapAssetId, gridSize, width, height);
			name = '';
			mapAssetId = null;
			gridSize = 70;
			creating = false;
			open = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to create scene');
		} finally {
			submitting = false;
		}
	}

	function startReplacing(sceneId: string) {
		confirmingDeleteId = null;
		replacementAssetId = null;
		replacingId = sceneId;
	}

	function confirmReplace() {
		if (!replacingId) return;
		room.setSceneMap(replacingId, replacementAssetId, replacementWidth, replacementHeight);
		replacingId = null;
		replacementAssetId = null;
	}

	function handleDelete(sceneId: string) {
		if (confirmingDeleteId !== sceneId) {
			confirmingDeleteId = sceneId;
			return;
		}
		room.deleteScene(sceneId);
		confirmingDeleteId = null;
	}
</script>

<Dialog.Root bind:open>
	{#if trigger}
		<Dialog.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="outline">Scenes</Button>
			{/snippet}
		</Dialog.Trigger>
	{/if}
	<Dialog.Content>
		{#if creating}
			<Dialog.Header>
				<Dialog.Title>New scene</Dialog.Title>
				<Dialog.Description>
					The room's first scene becomes active straight away. After that, new scenes wait in this
					list until you switch to one.
				</Dialog.Description>
			</Dialog.Header>
			<form class="flex flex-col gap-4" onsubmit={handleCreate}>
				<div class="flex flex-col gap-2">
					<Label for="scene-name">Name</Label>
					<Input id="scene-name" bind:value={name} required />
				</div>
				<div class="flex flex-col gap-2">
					<Label>Map (optional)</Label>
					<!-- Maps only, with no Tokens tab: a scene is asking what goes
					     *under* the tokens, so the other half of the library is
					     noise here, and an offer to file the wrong thing. -->
					<AssetPicker
						{roomSlug}
						{sessionToken}
						idPrefix="scene"
						kind="map"
						lockKind
						bind:selectedId={mapAssetId}
						onpick={adoptGridSize}
						emptyHint="Nothing in the library yet — add a map on the assets page."
					/>
				</div>
				<div class="grid grid-cols-3 gap-2">
					<div class="flex flex-col gap-2">
						<Label for="scene-grid">Grid size (px)</Label>
						<Input id="scene-grid" type="number" min="10" bind:value={gridSize} required />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="scene-width">Width (px)</Label>
						<Input id="scene-width" type="number" min="1" bind:value={width} required />
					</div>
					<div class="flex flex-col gap-2">
						<Label for="scene-height">Height (px)</Label>
						<Input id="scene-height" type="number" min="1" bind:value={height} required />
					</div>
				</div>
				<Dialog.Footer>
					<Button type="button" variant="ghost" onclick={() => (creating = false)}>Cancel</Button>
					<Button type="submit" disabled={submitting}>
						{submitting ? 'Creating…' : 'Create scene'}
					</Button>
				</Dialog.Footer>
			</form>
		{:else if replacing}
			<Dialog.Header>
				<Dialog.Title>Replace the map for {replacing.name}</Dialog.Title>
				<Dialog.Description>
					The tokens, fog and drawings already on this scene stay exactly where they are.
				</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-4">
				<div class="flex flex-col gap-2">
					<Label>New map</Label>
					<AssetPicker
						{roomSlug}
						{sessionToken}
						idPrefix="replace-map"
						kind="map"
						lockKind
						bind:selectedId={replacementAssetId}
						emptyHint="Nothing in the library yet — add a map on the assets page."
					/>
				</div>
				<Dialog.Footer>
					<Button variant="ghost" onclick={() => (replacingId = null)}>Cancel</Button>
					<Button onclick={confirmReplace} disabled={!replacementAssetId}>Replace map</Button>
				</Dialog.Footer>
			</div>
		{:else}
			<Dialog.Header>
				<Dialog.Title>Scenes</Dialog.Title>
				<Dialog.Description>
					Switching moves everyone in the room to that map straight away.
				</Dialog.Description>
			</Dialog.Header>
			{#if room.scenes.length === 0}
				<p class="text-sm text-muted-foreground">No scenes yet.</p>
			{:else}
				<ul class="flex max-h-80 flex-col gap-2 overflow-y-auto">
					{#each room.scenes as scene (scene.id)}
						{@const isActive = room.scene?.id === scene.id}
						<li class="flex flex-wrap items-center gap-2 rounded-md border p-2">
							<span class="min-w-0 flex-1 truncate text-sm">{scene.name}</span>
							{#if isActive}
								<Badge variant="secondary">Active</Badge>
							{:else}
								<Button
									size="sm"
									variant="outline"
									aria-label="Switch to {scene.name}"
									onclick={() => room.setActiveScene(scene.id)}
								>
									Switch to
								</Button>
							{/if}
							<Button
								size="sm"
								variant="outline"
								aria-label="Replace the map for {scene.name}"
								onclick={() => startReplacing(scene.id)}
							>
								Replace map
							</Button>
							<!-- The active scene can't be deleted: the room points at it
							     by id with nothing to clean that up, so the server refuses
							     it too. Saying why beats a button that errors. -->
							<Button
								size="sm"
								variant={confirmingDeleteId === scene.id ? 'destructive' : 'outline'}
								disabled={isActive}
								title={isActive ? 'Switch to another scene before deleting this one' : undefined}
								aria-label={confirmingDeleteId === scene.id
									? `Confirm deleting ${scene.name}`
									: `Delete ${scene.name}`}
								onclick={() => handleDelete(scene.id)}
							>
								{confirmingDeleteId === scene.id ? 'Really delete?' : 'Delete'}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}
			<!-- The way in to making one, at the foot of the list it will
			     join. It was a separate menu entry and a separate dialog,
			     which meant choosing between "the scenes" and "a new scene"
			     from a menu, before seeing either. -->
			<Dialog.Footer>
				<Button variant="outline" onclick={startCreating}>
					<Plus class="h-4 w-4" /> New scene
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
