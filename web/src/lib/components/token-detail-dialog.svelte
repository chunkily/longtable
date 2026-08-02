<script lang="ts">
	// Editing a token after it exists. Opens from the Edit button in the
	// token details section above chat, never from a hover and never from
	// the token itself — a deliberate call recorded in the backlog item,
	// so the panel only ever appears on purpose.
	import { toast } from 'svelte-sonner';
	import type { RoomClient, Token } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import AssetPicker from '$lib/components/asset-picker.svelte';
	import TokenSizePicker, { sizeForSquares } from '$lib/components/token-size-picker.svelte';

	let {
		room,
		token,
		roomSlug,
		sessionToken
	}: {
		room: RoomClient;
		token: Token;
		roomSlug: string;
		sessionToken: string;
	} = $props();

	let open = $state(false);
	let name = $state('');
	let visibility = $state<'visible' | 'hidden'>('visible');
	let imageAssetId = $state<string | null>(null);
	let squares = $state(1);

	// Filled from the token when the dialog opens rather than kept in step
	// with it. Someone else moving or renaming the token mid-edit would
	// otherwise overwrite what's been typed — and on submit the form is
	// the intent, so it wins.
	function load() {
		name = token.name;
		visibility = token.visibility;
		imageAssetId = token.imageAssetId;
		squares = sizeForSquares(token.width).squares;
	}

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		try {
			room.updateToken(token.id, {
				name,
				imageAssetId,
				width: squares,
				height: squares,
				visibility
			});
			open = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to update token');
		}
	}
</script>

<Dialog.Root
	bind:open
	onOpenChange={(next) => {
		if (next) load();
	}}
>
	<Dialog.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="outline" size="sm" aria-label="Edit token">Edit</Button>
		{/snippet}
	</Dialog.Trigger>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Edit token</Dialog.Title>
			<Dialog.Description>
				Changes reach everyone in the room. Drag the token to move it.
			</Dialog.Description>
		</Dialog.Header>
		<form class="flex flex-col gap-4" onsubmit={handleSubmit}>
			<div class="flex flex-col gap-2">
				<Label for="edit-token-name">Name</Label>
				<Input id="edit-token-name" bind:value={name} required />
			</div>
			<TokenSizePicker bind:squares idPrefix="edit-token" />
			<div class="flex flex-col gap-2">
				<Label>Image (optional)</Label>
				<AssetPicker
					{roomSlug}
					{sessionToken}
					idPrefix="edit-token"
					bind:selectedId={imageAssetId}
					emptyHint="Nothing in the library yet — upload an image, or leave blank for a plain marker."
				/>
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
				<Button type="submit">Save changes</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
