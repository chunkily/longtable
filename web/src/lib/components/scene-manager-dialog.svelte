<script lang="ts">
	// The room's scenes, and everything a GM can do to one after it
	// exists: switch to it, swap its map, throw it away. Before this
	// there was no way to reach a scene other than the one `scene.create`
	// auto-activated, which is why it used to auto-activate at all.
	import { assetUrl } from '$lib/api';
	import type { RoomClient } from '$lib/room.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import AssetPicker from '$lib/components/asset-picker.svelte';

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
		}
	});

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
		{#if replacing}
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
				<p class="text-sm text-muted-foreground">
					No scenes yet — make one with <strong>New scene</strong>.
				</p>
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
		{/if}
	</Dialog.Content>
</Dialog.Root>
