<script lang="ts">
	// Editing a token after it exists. Opens from the Edit button in the
	// token details section above chat, never from a hover and never from
	// the token itself — a deliberate call recorded in the backlog item,
	// so the panel only ever appears on purpose.
	import { toast } from 'svelte-sonner';
	import { tokenTrackers, type RoomClient, type Token, type Tracker } from '$lib/room.svelte';
	import { sameTokenFields, type TokenFields } from '$lib/token-fields';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import { ownerOptions } from '$lib/token-owner';
	import SquarePen from '@lucide/svelte/icons/square-pen';
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

	// What the form held when it opened, for telling "nothing typed" from
	// "typed and about to be thrown away".
	let loaded = $state<TokenFields | null>(null);

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
		loaded = current();
	}

	function current(): TokenFields {
		return {
			name,
			imageAssetId,
			width: squares,
			height: squares,
			ownerParticipantId,
			visibility,
			trackers,
			conditions
		};
	}

	// Whether there is anything here worth warning about losing.
	const dirty = $derived(!!loaded && !sameTokenFields(loaded, current()));

	/**
	 * Whether the warning is on screen instead of the form.
	 *
	 * One dialog at a time, deliberately: the edit dialog closes and this
	 * takes its place, rather than stacking a second focus trap over a
	 * form nobody can reach. The form's values live in this component, so
	 * they survive the swap and Save changes still has something to send.
	 */
	let warning = $state(false);

	// Closing without sending. The next open reloads every field from the
	// token, so what was typed goes with the dialog.
	function cancel() {
		warning = false;
		open = false;
	}

	/**
	 * Set while the editor is being reopened from the warning, so the form
	 * isn't reloaded from the token on the way back in — the whole point
	 * of Back is that what was typed is still there.
	 */
	let resuming = false;

	function back() {
		resuming = true;
		warning = false;
		open = true;
	}

	function save() {
		try {
			// The GM-only fields go back exactly as they were loaded when a
			// Player submits. The server ignores them for a non-GM, and
			// sending them keeps this one call good for both roles — see
			// updateToken's note on why there isn't a narrower command.
			// updateToken drops a submit that changed nothing.
			room.updateToken(token.id, current());
			warning = false;
			open = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to update token');
		}
	}

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		save();
	}

	/**
	 * Clicking away from a form with something in it.
	 *
	 * Escape, the X and Cancel all discard, which is what those three
	 * controls mean everywhere else. Clicking away is the ambiguous one —
	 * it's as often a misclick as a decision — so it asks rather than
	 * guessing, and asks *instead of* the form rather than on top of it.
	 * With nothing typed there is nothing to ask about, and it just closes.
	 */
	function handleInteractOutside(event: PointerEvent) {
		if (!dirty) return;
		// Stops the primitive closing it for us, so the swap to the warning
		// happens in one step rather than as a close followed by an open.
		event.preventDefault();
		open = false;
		warning = true;
	}
</script>

<Dialog.Root
	bind:open
	onOpenChange={(next) => {
		// Reopened from the warning, the form keeps what was typed; opened
		// from the pen, it starts from the token.
		if (next && !resuming) load();
		resuming = false;
	}}
>
	<Dialog.Trigger>
		{#snippet child({ props })}
			<!-- An icon, like the delete button beside it: the two sit
			     together in a 368px rail where a word-labelled button next to
			     an icon one reads as two different kinds of control. The
			     accessible name still says "Edit token" — what a screen
			     reader announces, what the tooltip shows, and what the specs
			     find it by. -->
			<Button
				{...props}
				variant="outline"
				size="sm"
				aria-label="Edit token"
				title="Edit this token"
			>
				<SquarePen class="h-4 w-4" />
			</Button>
		{/snippet}
	</Dialog.Trigger>
	<!-- Bounded by the window rather than by a fraction of it, and laid
	     out as header / scrolling body / pinned footer. It used to cap the
	     *form* at 70vh, which meant a 792px form scrolling inside a 700px
	     box on a 1000px screen with 208px going spare — and because the
	     footer was inside that scroller, `Save changes` sat below the
	     dialog's own bottom edge until you found it. The first person to
	     hit it typed an edit, clicked away and lost it, never having seen
	     that there was a button to press. -->
	<Dialog.Content
		class="flex max-h-[calc(100dvh-2rem)] flex-col"
		onInteractOutside={handleInteractOutside}
	>
		<Dialog.Header>
			<Dialog.Title>Edit token</Dialog.Title>
			<Dialog.Description>
				{canEditAll
					? 'Changes reach everyone in the room. Drag the token to move it.'
					: 'Your token — track its damage and status here. Everything else is the GM’s.'}
			</Dialog.Description>
		</Dialog.Header>
		<form class="flex min-h-0 flex-1 flex-col gap-4" onsubmit={handleSubmit}>
			<!-- Only the fields scroll. The footer below is outside this box
			     on purpose: a primary action that scrolls away is one nobody
			     knows is there. -->
			<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto">
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
			</div>
			<!-- Cancel on the left, away from the button it undoes. It is the
			     third way out — Escape and the X do the same thing — and the
			     form is reloaded from the token on every open, so all three
			     are simply closing without sending. -->
			<Dialog.Footer class="justify-between">
				<Button type="button" variant="ghost" onclick={cancel}>Cancel</Button>
				<Button type="submit">Save changes</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<!-- The edit dialog has already closed by the time this opens: one
     dialog on screen at a time, so this is a question about the form
     rather than a second layer over it. -->
<Dialog.Root bind:open={warning}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Keep your changes to {token.name}?</Dialog.Title>
			<Dialog.Description>
				You clicked away from the editor with unsaved changes.
			</Dialog.Description>
		</Dialog.Header>
		<!-- Three answers, and Back is the one that costs nothing: the
		     question is most often the result of a misclick, and the reply
		     to a misclick should be "put it back how it was". It keeps the
		     form's values, which is why reopening skips the reload. -->
		<Dialog.Footer class="justify-between">
			<Button type="button" variant="outline" onclick={back}>Back</Button>
			<div class="flex gap-2">
				<!-- Named for what it does, not "Cancel": this dialog is a
				     question with two answers and a way out, and "cancel" beside
				     "save" reads as cancelling the *save* rather than the edit.
				     The edit dialog's own Cancel keeps that name, where it is
				     unambiguous. -->
				<Button type="button" variant="ghost" onclick={cancel}>Discard changes</Button>
				<Button type="button" onclick={save}>Save changes</Button>
			</div>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
