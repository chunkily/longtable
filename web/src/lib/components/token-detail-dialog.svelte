<script lang="ts">
	// Editing a token after it exists. Opens from the Edit button in the
	// token details section above chat, never from a hover and never from
	// the token itself — a deliberate call recorded in the backlog item,
	// so the panel only ever appears on purpose.
	import { toast } from 'svelte-sonner';
	import { tokenTrackers, type RoomClient, type Token, type Tracker } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import { ownerOptions } from '$lib/token-owner';
	import AssetPicker from '$lib/components/asset-picker.svelte';
	import TokenOwnerPicker from '$lib/components/token-owner-picker.svelte';
	import TokenSizePicker, { sizeForSquares } from '$lib/components/token-size-picker.svelte';
	import TokenTrackerFields from '$lib/components/token-tracker-fields.svelte';

	let {
		room,
		token,
		roomSlug,
		sessionToken,
		canEditAll = true
	}: {
		room: RoomClient;
		token: Token;
		roomSlug: string;
		sessionToken: string;
		/**
		 * False for a Player editing a token they merely own: they get the
		 * trackers and conditions and nothing else, mirroring the per-field
		 * check in handleTokenUpdate. The server enforces it regardless —
		 * this only decides what's worth putting on screen.
		 */
		canEditAll?: boolean;
	} = $props();

	let open = $state(false);
	let name = $state('');
	let visibility = $state<'visible' | 'hidden'>('visible');
	let imageAssetId = $state<string | null>(null);
	let squares = $state(1);
	let ownerParticipantId = $state<string | null>(null);
	let trackers = $state<Tracker[]>([]);
	let conditions = $state<string[]>([]);

	// Filled from the token when the dialog opens rather than kept in step
	// with it. Someone else moving or renaming the token mid-edit would
	// otherwise overwrite what's been typed — and on submit the form is
	// the intent, so it wins.
	function load() {
		name = token.name;
		visibility = token.visibility;
		imageAssetId = token.imageAssetId;
		squares = sizeForSquares(token.width).squares;
		ownerParticipantId = token.ownerParticipantId;
		// Copied rather than referenced: the fields bind straight into these
		// arrays, and editing the ones hanging off `token` would rewrite
		// RoomClient's own state as it was typed, before anything was saved.
		trackers = tokenTrackers(token).map((t) => ({ ...t }));
		conditions = [...(token.conditions ?? [])];
	}

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		try {
			// The GM-only fields go back exactly as they were loaded when a
			// Player submits. The server ignores them for a non-GM, and
			// sending them keeps this one call good for both roles — see
			// updateToken's note on why there isn't a narrower command.
			room.updateToken(token.id, {
				name,
				imageAssetId,
				width: squares,
				height: squares,
				ownerParticipantId,
				visibility,
				trackers,
				conditions
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
				{canEditAll
					? 'Changes reach everyone in the room. Drag the token to move it.'
					: 'Your token — track its damage and status here. Everything else is the GM’s.'}
			</Dialog.Description>
		</Dialog.Header>
		<!-- Scrollable because the GM's form is now long enough to run off a
		     laptop screen, and the footer button has to stay reachable. -->
		<form class="flex max-h-[70vh] flex-col gap-4 overflow-y-auto" onsubmit={handleSubmit}>
			{#if canEditAll}
				<div class="flex flex-col gap-2">
					<Label for="edit-token-name">Name</Label>
					<Input id="edit-token-name" bind:value={name} required />
				</div>
				<TokenSizePicker bind:squares idPrefix="edit-token" />
				<!-- Whoever is connected, plus this token's own owner if they have
				     since left: the update carries the owner every time, so a list
				     that dropped them would have a rename quietly unassign them. -->
				<TokenOwnerPicker
					bind:ownerId={ownerParticipantId}
					options={ownerOptions(
						room.connectedParticipants,
						room.participants,
						token.ownerParticipantId
					)}
					idPrefix="edit-token"
				/>
				<div class="flex flex-col gap-2">
					<Label>Image (optional)</Label>
					<!-- Token art only, like the create dialog beside it. -->
					<AssetPicker
						{roomSlug}
						{sessionToken}
						idPrefix="edit-token"
						kind="token"
						lockKind
						bind:selectedId={imageAssetId}
						emptyHint="Nothing in the library yet — add art on the assets page, or leave blank for a plain marker."
					/>
				</div>
			{/if}
			<TokenTrackerFields bind:trackers bind:conditions idPrefix="edit-token" />
			{#if canEditAll}
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
			{/if}
			<Dialog.Footer>
				<Button type="submit">Save changes</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
