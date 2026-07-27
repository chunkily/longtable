<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { uploadAsset } from '$lib/api';
	import type { RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';

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
	let file = $state<File | null>(null);
	let submitting = $state(false);

	function handleFileChange(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		file = input.files?.[0] ?? null;
		if (!file) return;

		const img = new Image();
		img.onload = () => {
			width = img.naturalWidth;
			height = img.naturalHeight;
			URL.revokeObjectURL(img.src);
		};
		img.src = URL.createObjectURL(file);
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		submitting = true;
		try {
			let mapAssetId: string | null = null;
			if (file) {
				const asset = await uploadAsset(roomSlug, sessionToken, file);
				mapAssetId = asset.id;
			}
			room.createScene(name, mapAssetId, gridSize, width, height);
			open = false;
			name = '';
			file = null;
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
				Uploading a map makes this the room's active scene immediately.
			</Dialog.Description>
		</Dialog.Header>
		<form class="flex flex-col gap-4" onsubmit={handleSubmit}>
			<div class="flex flex-col gap-2">
				<Label for="scene-name">Name</Label>
				<Input id="scene-name" bind:value={name} required />
			</div>
			<div class="flex flex-col gap-2">
				<Label for="scene-map">Map image (optional)</Label>
				<Input id="scene-map" type="file" accept="image/*" onchange={handleFileChange} />
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
