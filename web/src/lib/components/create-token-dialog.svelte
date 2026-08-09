<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { MAX_TOKENS_PER_CREATE, type RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import { ownerOptions } from '$lib/token-owner';
	import UserRoundPlus from '@lucide/svelte/icons/user-round-plus';
	import AssetPicker from '$lib/components/asset-picker.svelte';
	import TokenOwnerPicker from '$lib/components/token-owner-picker.svelte';
	import TokenSizePicker from '$lib/components/token-size-picker.svelte';

	let {
		room,
		sceneId,
		roomSlug,
		sessionToken,
		isGM,
		spawnCell
	}: {
		room: RoomClient;
		sceneId: string;
		roomSlug: string;
		sessionToken: string;
		/**
		 * Which of the two dialogs this is. A Player's differs by exactly
		 * two fields: no owner picker, because a token they make is theirs,
		 * and no visibility toggle, because hiding something from the room
		 * is a GM power — and the one field a Player could use to hide a
		 * token from the GM. The server enforces both regardless of what
		 * this form sends; see handleTokenCreate.
		 */
		isGM: boolean;
		spawnCell: () => { x: number; y: number };
	} = $props();

	let open = $state(false);
	let name = $state('');
	let visibility = $state<'visible' | 'hidden'>('visible');
	let imageAssetId = $state<string | null>(null);
	let squares = $state(1);
	let ownerParticipantId = $state<string | null>(null);
	let count = $state(1);
	let submitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		submitting = true;
		try {
			// dropped near the middle of whatever the creator is currently
			// looking at — drag it into place on the canvas after creation.
			// More than one spreads outward from there, server-side, so they
			// arrive as a block rather than a stack.
			const { x, y } = spawnCell();
			room.createToken(sceneId, name, imageAssetId, x, y, visibility, {
				squares,
				ownerParticipantId,
				count
			});
			open = false;
			name = '';
			imageAssetId = null;
			squares = 1;
			ownerParticipantId = null;
			visibility = 'visible';
			count = 1;
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
			<!-- Icon-only, like every other button on the tool row it sits in —
			     a word-labelled button there was the odd one out and cost the
			     row most of its width on a phone. The accessible name still
			     says "New token": it's what a screen reader reads out, what
			     the tooltip shows, and what the e2e specs find it by. -->
			<Button {...props} variant="ghost" size="sm" aria-label="New token" title="New token">
				<UserRoundPlus class="h-4 w-4" />
			</Button>
		{/snippet}
	</Dialog.Trigger>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>New token</Dialog.Title>
			<!-- The numbering is the surprising part of a batch, so it's said
			     before they're made rather than discovered afterwards: it is
			     cheaper to read here than to rename eight tokens. -->
			<Dialog.Description>
				{#if count > 1}
					They appear near the center of your current view, spread over free squares, named
					{name || 'Monkey'} 1 … {name || 'Monkey'}
					{count}.
				{:else}
					It appears near the center of your current view — drag it into place.
				{/if}
			</Dialog.Description>
		</Dialog.Header>
		<form class="flex flex-col gap-4" onsubmit={handleSubmit}>
			<!-- The count shares the name's row rather than taking one of its
			     own, and that isn't only tidiness: this dialog was already
			     within one field of the window's height, and a dialog taller
			     than the window puts its own close button off the top of the
			     screen where nothing can scroll to it. The number input is
			     the stepper — no need for buttons beside it.

			     Offered to a GM and a Player alike: six goblins is the same
			     eight-trips-through-a-dialog problem as eight monkeys. The
			     cap here only stops the spinner where the server refuses, so
			     it never becomes an error toast after the fact. -->
			<div class="flex gap-2">
				<div class="flex flex-1 flex-col gap-2">
					<Label for="token-name">Name</Label>
					<Input id="token-name" bind:value={name} required />
				</div>
				<div class="flex w-24 flex-col gap-2">
					<Label for="token-count">How many</Label>
					<Input
						id="token-count"
						type="number"
						min="1"
						max={MAX_TOKENS_PER_CREATE}
						class="text-center"
						bind:value={count}
					/>
				</div>
			</div>
			<TokenSizePicker bind:squares idPrefix="token" />
			{#if isGM}
				<!-- A token being made now has no owner to preserve, so this is
				     simply whoever is at the table. -->
				<TokenOwnerPicker
					bind:ownerId={ownerParticipantId}
					options={ownerOptions(room.connectedParticipants, room.participants, null)}
				/>
			{/if}
			<div class="flex flex-col gap-2">
				<Label>Image (optional)</Label>
				<AssetPicker
					{roomSlug}
					{sessionToken}
					idPrefix="token"
					bind:selectedId={imageAssetId}
					emptyHint="Nothing in the library yet — add art on the assets page, or leave blank for a plain marker."
				/>
			</div>
			{#if isGM}
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
				<Button type="submit" disabled={submitting}>
					{submitting ? 'Creating…' : 'Create token'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
