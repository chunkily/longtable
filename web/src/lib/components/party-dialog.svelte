<script lang="ts">
	// The GM's party: who it is, and a button to put them all on this scene.
	//
	// A member is defined here and nowhere else — the tokens on maps are
	// copies made from it (see internal/ws/party.go for why the party
	// stopped being a mark on tokens). Editing a member is a mode of this
	// dialog rather than a second dialog or a popover, for the reason the
	// Scenes dialog gives: one dialog on screen at a time, and a popover
	// inside a dialog comes out unpositioned and under the overlay.
	import { assetUrl } from '$lib/api';
	import type { PartyMember, RoomClient } from '$lib/room.svelte';
	import { partyOwnerOptions } from '$lib/token-owner';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';
	import AssetPicker from '$lib/components/asset-picker.svelte';
	import TokenOwnerPicker from '$lib/components/token-owner-picker.svelte';
	import TokenSizePicker, { sizeForSquares } from '$lib/components/token-size-picker.svelte';
	import Users from '@lucide/svelte/icons/users';
	import Plus from '@lucide/svelte/icons/plus';
	import SquarePen from '@lucide/svelte/icons/square-pen';
	import Trash2 from '@lucide/svelte/icons/trash-2';

	let {
		room,
		sceneId,
		roomSlug,
		sessionToken,
		spawnCell
	}: {
		room: RoomClient;
		/** The scene on this screen, which is where Add to this scene puts them. */
		sceneId: string;
		roomSlug: string;
		sessionToken: string;
		/** The square the party spreads out from — the middle of the GM's view. */
		spawnCell: () => { x: number; y: number };
	} = $props();

	let open = $state(false);

	/**
	 * The member form, when it's showing instead of the list. `id` is null
	 * for a new member — including one started from a token on the map,
	 * which only fills the form in.
	 */
	let editing = $state<{ id: string | null } | null>(null);
	let name = $state('');
	let imageAssetId = $state<string | null>(null);
	let squares = $state(1);
	let ownerId = $state<string | null>(null);

	// Removing is armed by the first click and done by the second, like
	// deleting a scene. Nothing on a map goes with it — the copies stay as
	// ordinary tokens — but the member's details do.
	let confirmingRemoveId = $state<string | null>(null);

	function startEditing(id: string | null, from: Partial<PartyMember>) {
		confirmingRemoveId = null;
		name = from.name ?? '';
		imageAssetId = from.imageAssetId ?? null;
		squares = sizeForSquares(from.width ?? 1).squares;
		ownerId = from.ownerParticipantId ?? null;
		editing = { id };
	}

	// The token is a starting point, not a link: what's copied is what the
	// form shows, and the GM can change any of it before saving.
	function startFromToken(tokenId: string) {
		const token = room.tokens.find((t) => t.id === tokenId);
		if (token) startEditing(null, token);
	}

	function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		if (!editing) return;
		const fields = {
			name,
			imageAssetId,
			width: squares,
			height: squares,
			ownerParticipantId: ownerId,
			// Not on the form — the checkbox in the list is where it's set —
			// so an edit carries what the member already had, and someone new
			// is here unless the GM says otherwise.
			attending: editingMember?.attending ?? true
		};
		if (editing.id) room.updatePartyMember(editing.id, fields);
		else room.createPartyMember(fields);
		editing = null;
	}

	function handleRemove(member: PartyMember) {
		if (confirmingRemoveId !== member.id) {
			confirmingRemoveId = member.id;
			return;
		}
		confirmingRemoveId = null;
		room.deletePartyMember(member.id);
	}

	function addToScene() {
		const { x, y } = spawnCell();
		room.addParty(sceneId, x, y);
		// Closed so the GM sees them land. Anyone already here is skipped,
		// and a party that's all here already says so in a toast.
		open = false;
	}

	// Its own update rather than a field on the form: one click, sent as
	// it's made. The member's other details go back exactly as they were.
	function setAttending(member: PartyMember, attending: boolean) {
		const { id, ...fields } = member;
		room.updatePartyMember(id, { ...fields, attending });
	}

	const attendingCount = $derived(room.party.filter((m) => m.attending).length);

	function ownerName(id: string | null): string | null {
		if (!id) return null;
		return room.participants.find((p) => p.id === id)?.displayName ?? null;
	}

	const editingMember = $derived(
		editing?.id ? room.party.find((m) => m.id === editing?.id) : undefined
	);
</script>

<Dialog.Root
	bind:open
	onOpenChange={(next) => {
		// Every open starts on the list.
		if (next) {
			editing = null;
			confirmingRemoveId = null;
		}
	}}
>
	<Dialog.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="ghost" size="sm" aria-label="Party" title="Party">
				<Users class="h-4 w-4" />
			</Button>
		{/snippet}
	</Dialog.Trigger>
	<!-- Capped to the window with only the middle scrolling, the token
	     dialogs' layout: the member form carries the art picker, which is
	     tall enough on its own to push the buttons off a laptop screen. -->
	<Dialog.Content class="flex max-h-[calc(100dvh-2rem)] flex-col sm:max-w-lg">
		{#if editing}
			<Dialog.Header>
				<Dialog.Title>{editingMember ? `Edit ${editingMember.name}` : 'New member'}</Dialog.Title>
				<Dialog.Description>Changes apply the next time the party is added.</Dialog.Description>
			</Dialog.Header>
			<form class="flex min-h-0 flex-1 flex-col gap-4" onsubmit={handleSubmit}>
				<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto">
					<div class="flex flex-col gap-2">
						<Label for="party-name">Name</Label>
						<Input id="party-name" bind:value={name} required />
					</div>
					<TokenSizePicker bind:squares idPrefix="party" />
					<TokenOwnerPicker
						bind:ownerId
						options={partyOwnerOptions(room.connectedParticipants, room.participants, ownerId)}
						idPrefix="party"
					/>
					<div class="flex flex-col gap-2">
						<Label>Image (optional)</Label>
						<AssetPicker
							{roomSlug}
							{sessionToken}
							idPrefix="party"
							kind="token"
							lockKind
							bind:selectedId={imageAssetId}
							emptyHint="Nothing in the library yet — add art on the assets page, or leave blank for a plain marker."
						/>
					</div>
				</div>
				<Dialog.Footer class="justify-between">
					<Button type="button" variant="ghost" onclick={() => (editing = null)}>Cancel</Button>
					<Button type="submit">{editing.id ? 'Save changes' : 'Add to party'}</Button>
				</Dialog.Footer>
			</form>
		{:else}
			<Dialog.Header>
				<Dialog.Title>Party</Dialog.Title>
				<Dialog.Description>
					Only ticked members are added. Anyone already on this scene is skipped.
				</Dialog.Description>
			</Dialog.Header>
			<!-- First, because putting the party down is what the dialog is
			     opened for on most nights; configuring it is the rarer job. -->
			<Button onclick={addToScene} disabled={attendingCount === 0}>
				<Users class="h-4 w-4" /> Add to this scene
			</Button>
			<div class="min-h-0 flex-1 overflow-y-auto">
				{#if room.party.length === 0}
					<p class="text-sm text-muted-foreground">No one in the party yet.</p>
				{:else}
					<ul class="flex flex-col gap-2">
						{#each room.party as member (member.id)}
							{@const owner = ownerName(member.ownerParticipantId)}
							<li class="flex items-center gap-2 rounded-md border p-2">
								<!-- A native checkbox: there's no checkbox primitive in
								     ui/, and this is the one place that needs it. -->
								<input
									type="checkbox"
									class="h-4 w-4 shrink-0 accent-primary"
									checked={member.attending}
									aria-label="{member.name} is attending"
									title="Attending"
									onchange={(event) => setAttending(member, event.currentTarget.checked)}
								/>
								<!-- Dimmed while unticked, so who's coming tonight reads at a
								     glance down the list rather than only from the boxes. The
								     art and name only: dimming the whole row faded the edit and
								     remove buttons too, which then looked disabled and weren't. -->
								<span
									class={[
										'flex min-w-[6rem] flex-1 items-center gap-2',
										!member.attending && 'opacity-50'
									]}
								>
									{#if member.imageAssetId}
										<img
											src={assetUrl(member.imageAssetId)}
											alt=""
											class="h-8 w-8 shrink-0 rounded-full object-cover"
										/>
									{:else}
										<span
											class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-bold"
											aria-hidden="true"
										>
											{member.name.slice(0, 2).toUpperCase()}
										</span>
									{/if}
									<span class="flex min-w-0 flex-1 flex-col">
										<span class="truncate text-sm">{member.name}</span>
										<span class="truncate text-xs text-muted-foreground">
											{sizeForSquares(member.width).label}{owner ? ` · ${owner}` : ''}
										</span>
									</span>
								</span>
								<Button
									size="sm"
									variant="outline"
									title="Edit"
									aria-label="Edit {member.name}"
									onclick={() => startEditing(member.id, member)}
								>
									<SquarePen class="h-4 w-4" />
								</Button>
								<!-- Armed, it says so in words: colour alone is no signal
								     to someone who can't see it. Same as deleting a scene. -->
								<Button
									size="sm"
									variant={confirmingRemoveId === member.id ? 'destructive' : 'outline'}
									title="Remove from the party"
									aria-label={confirmingRemoveId === member.id
										? `Confirm removing ${member.name}`
										: `Remove ${member.name} from the party`}
									onclick={() => handleRemove(member)}
								>
									{#if confirmingRemoveId === member.id}
										Really remove?
									{:else}
										<Trash2 class="h-4 w-4" />
									{/if}
								</Button>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
			<Dialog.Footer class="flex-wrap gap-2 sm:justify-between">
				<!-- A native select rather than a menu: a popover inside a dialog
				     doesn't work here (seats-dialog.svelte has the details). It
				     acts on change and then resets, so it reads as a button that
				     happens to offer a list. -->
				{#if room.tokens.length > 0}
					<select
						aria-label="Add from this scene"
						class="h-9 min-w-0 rounded-md border border-input bg-transparent px-2.5 text-sm shadow-xs dark:bg-input/30"
						value=""
						onchange={(event) => {
							const id = event.currentTarget.value;
							event.currentTarget.value = '';
							if (id) startFromToken(id);
						}}
					>
						<option value="">Add from this scene…</option>
						{#each room.tokens as token (token.id)}
							<option value={token.id}>{token.name}</option>
						{/each}
					</select>
				{/if}
				<Button variant="outline" onclick={() => startEditing(null, {})}>
					<Plus class="h-4 w-4" /> New member
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
