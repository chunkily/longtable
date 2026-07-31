<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { assetUrl } from '$lib/api';
	import type { RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import AssetPicker from '$lib/components/asset-picker.svelte';

	let {
		room,
		roomSlug,
		sessionToken
	}: { room: RoomClient; roomSlug: string; sessionToken: string } = $props();

	let open = $state(false);
	let name = $state('');
	let gridSize = $state(70);
	let width = $state(1400);
	let height = $state(1000);
	let mapAssetId = $state<string | null>(null);
	let submitting = $state(false);

	// Width/height default to the chosen map's real dimensions, whichever
	// way it was chosen — freshly uploaded or picked from the library.
	// Loading it as a plain Image rather than trusting anything cached
	// from the picker's thumbnail keeps this the actual asset dimensions,
	// not a guess from a clipped 64px preview.
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
	<Dialog.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="outline">New scene</Button>
		{/snippet}
	</Dialog.Trigger>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>New scene</Dialog.Title>
			<Dialog.Description>
				Creating a scene makes it the room's active scene immediately.
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
					bind:selectedId={mapAssetId}
					emptyHint="Nothing in the library yet — upload a map to get started."
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
