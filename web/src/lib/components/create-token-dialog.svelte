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
		sceneId,
		roomSlug,
		sessionToken,
		spawnCell
	}: {
		room: RoomClient;
		sceneId: string;
		roomSlug: string;
		sessionToken: string;
		spawnCell: () => { x: number; y: number };
	} = $props();

	let open = $state(false);
	let name = $state('');
	let visibility = $state<'visible' | 'hidden'>('visible');
	let file = $state<File | null>(null);
	let submitting = $state(false);

	function handleFileChange(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		file = input.files?.[0] ?? null;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		submitting = true;
		try {
			let imageAssetId: string | null = null;
			if (file) {
				const asset = await uploadAsset(roomSlug, sessionToken, file);
				imageAssetId = asset.id;
			}
			// dropped near the middle of whatever the GM is currently looking
			// at — drag it into place on the canvas after creation
			const { x, y } = spawnCell();
			room.createToken(sceneId, name, imageAssetId, x, y, visibility);
			open = false;
			name = '';
			file = null;
			visibility = 'visible';
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to create token');
		} finally {
			submitting = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="outline">New token</Button>
		{/snippet}
	</Dialog.Trigger>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>New token</Dialog.Title>
			<Dialog.Description
				>It appears near the center of your current view — drag it into place.</Dialog.Description
			>
		</Dialog.Header>
		<form class="flex flex-col gap-4" onsubmit={handleSubmit}>
			<div class="flex flex-col gap-2">
				<Label for="token-name">Name</Label>
				<Input id="token-name" bind:value={name} required />
			</div>
			<div class="flex flex-col gap-2">
				<Label for="token-image">Image (optional)</Label>
				<Input id="token-image" type="file" accept="image/*" onchange={handleFileChange} />
			</div>
			<div class="flex flex-col gap-2">
				<Label>Visibility</Label>
				<div class="flex gap-2">
					<Button
						type="button"
						variant={visibility === 'visible' ? 'default' : 'outline'}
						onclick={() => (visibility = 'visible')}
						class="flex-1"
					>
						Visible
					</Button>
					<Button
						type="button"
						variant={visibility === 'hidden' ? 'default' : 'outline'}
						onclick={() => (visibility = 'hidden')}
						class="flex-1"
					>
						Hidden from players
					</Button>
				</div>
			</div>
			<Dialog.Footer>
				<Button type="submit" disabled={submitting}>
					{submitting ? 'Creating…' : 'Create token'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
