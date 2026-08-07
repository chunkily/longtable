<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { assetUrl, type Asset } from '$lib/api';
	import type { RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import AssetPicker from '$lib/components/asset-picker.svelte';

	// `open` is bindable and `trigger` optional so this can be opened from
	// somewhere that isn't a button of its own — the Scenes dialog offers
	// New scene from inside itself now that neither is on the toolbar, and
	// a trigger rendered there would be a button inside a dialog opening a
	// second dialog over it.
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

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		submitting = true;
		try {
			room.createScene(name, mapAssetId, gridSize, width, height);
			open = false;
			name = '';
			mapAssetId = null;
			gridSize = 70;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to create scene');
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root bind:open>
	{#if trigger}
		<Dialog.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="outline">New scene</Button>
			{/snippet}
		</Dialog.Trigger>
	{/if}
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>New scene</Dialog.Title>
			<Dialog.Description>
				The room's first scene becomes active straight away. After that, new scenes wait in
				<strong>Scenes</strong> until you switch to one.
			</Dialog.Description>
		</Dialog.Header>
		<form class="flex flex-col gap-4" onsubmit={handleSubmit}>
			<div class="flex flex-col gap-2">
				<Label for="scene-name">Name</Label>
				<Input id="scene-name" bind:value={name} required />
			</div>
			<div class="flex flex-col gap-2">
				<Label>Map (optional)</Label>
				<AssetPicker
					{roomSlug}
					{sessionToken}
					idPrefix="scene"
					kind="map"
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
				<Button type="submit" disabled={submitting}>
					{submitting ? 'Creating…' : 'Create scene'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
