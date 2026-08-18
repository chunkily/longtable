<script lang="ts">
	// Who is at this table. Everyone's to open, the GM's to change — a
	// Player gets the same list without the form that adds a chair or the
	// bins that clear one away.
	//
	// Split out of Manage room, which kept the two things that are
	// genuinely a GM's alone: the movement lock and the room password.
	// Seats went the other way because reading the roster isn't a GM
	// power — ADR-0007 draws the line at role boundaries, and "who is at
	// the table" is on the near side of it. It is also where your own
	// colour is now picked, which is what makes the dialog worth a
	// Player's time rather than a list they can only look at.
	import { toast } from 'svelte-sonner';
	import { addSeat, listSeats, removeSeat, type Seat } from '$lib/api';
	import { identityHex, suggestedColor } from '$lib/identity-color';
	import type { RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import * as Dialog from '$lib/components/ui/dialog';
	import IdentityColorPicker from '$lib/components/identity-color-picker.svelte';
	import Trash2 from '@lucide/svelte/icons/trash-2';

	let {
		room,
		roomSlug,
		sessionToken,
		isGM,
		open = $bindable(false)
	}: {
		room: RoomClient;
		roomSlug: string;
		sessionToken: string;
		isGM: boolean;
		open?: boolean;
	} = $props();

	let seats = $state<Seat[]>([]);
	let newName = $state('');
	let busy = $state(false);
	// Removing a seat takes its tokens' owner with it, so it arms in
	// place rather than firing on one click — the same two-step deleting
	// a scene uses.
	let confirmingId = $state<string | null>(null);

	// Seats come from the REST endpoint rather than from the roster in
	// `state.sync`, because this needs the same *pre-join* view a
	// returning device gets: whether anyone is sitting in a chair right
	// now, which the roster deliberately doesn't say.
	async function refresh() {
		try {
			seats = (await listSeats(roomSlug)).seats;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to load seats');
		}
	}

	$effect(() => {
		if (open) {
			confirmingId = null;
			refresh();
		}
	});

	/**
	 * The colour to paint a seat's dot, live where the list is not: that
	 * list was fetched when the dialog opened, and `participant.setColor`
	 * is deliberately not optimistic, so your own dot would sit on the
	 * old colour until something refetched. The roster is the same answer
	 * kept current by the socket. It falls back to the fetched value for
	 * a seat the roster hasn't got — a chair a GM set out that nobody has
	 * taken yet, which arrives over REST and not in `state.sync`.
	 */
	function seatColor(seat: Seat): string {
		return room.colorOf(seat.participantId) || seat.color;
	}

	async function handleAdd(event: SubmitEvent) {
		event.preventDefault();
		const name = newName.trim();
		if (!name) return;
		busy = true;
		try {
			// The GM picks the chair's colour too, so whoever takes it
			// arrives with one rather than being the only person at the
			// table with no way to have chosen.
			await addSeat(roomSlug, sessionToken, name, suggestedColor(seats.map((s) => s.color)));
			newName = '';
			await refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to add a seat');
		} finally {
			busy = false;
		}
	}

	async function handleRemove(seat: Seat) {
		if (confirmingId !== seat.participantId) {
			confirmingId = seat.participantId;
			return;
		}
		busy = true;
		try {
			await removeSeat(roomSlug, sessionToken, seat.participantId);
			confirmingId = null;
			await refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to remove that seat');
		} finally {
			busy = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<!-- Bounded by the window and laid out as header plus a scrolling
	     body, the same shape manage-room-dialog.svelte and
	     token-detail-dialog.svelte settled on: a full table plus the add
	     form and the palette outruns a laptop window, and Dialog.Content
	     is `fixed` and centred by transform, so overflow leaves the ends
	     off the edges rather than scrolled. -->
	<Dialog.Content class="flex max-h-[calc(100dvh-2rem)] flex-col">
		<Dialog.Header>
			<Dialog.Title>Seats</Dialog.Title>
			<Dialog.Description>
				Seats are how people are known in this room. A seat outlives a browser, so someone on a new
				device takes theirs back rather than joining as a stranger.
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto">
			{#if isGM}
				<form class="flex items-end gap-2" onsubmit={handleAdd}>
					<div class="flex flex-1 flex-col gap-2">
						<Label for="new-seat">Add a seat</Label>
						<!-- A GM can set the table before anyone arrives: a named chair
					     with nobody signed into it, waiting to be claimed. -->
						<Input
							id="new-seat"
							bind:value={newName}
							placeholder="Player's name"
							autocomplete="off"
						/>
					</div>
					<Button type="submit" disabled={busy || !newName.trim()}>Add</Button>
				</form>
			{/if}

			<ul class="flex flex-col gap-2">
				{#each seats as seat (seat.participantId)}
					{@const isYou = seat.participantId === room.you?.participantId}
					<li class="flex flex-wrap items-center gap-2 rounded-md border p-2">
						{#if identityHex(seatColor(seat))}
							<span
								class="h-3 w-3 shrink-0 rounded-full"
								style="background-color: {identityHex(seatColor(seat))}"
							></span>
						{/if}
						<span class="min-w-0 flex-1 truncate text-sm">{seat.displayName}</span>
						{#if isYou}
							<Badge variant="outline">you</Badge>
						{/if}
						{#if seat.role === 'gm'}
							<Badge>GM</Badge>
						{/if}
						{#if seat.connected}
							<Badge variant="secondary">here now</Badge>
						{/if}
						<!-- The GM's own seat can't go: the room password signs you
					     into it, so removing it would strand the only role that
					     could undo the damage. The server refuses it too. -->
						{#if isGM && seat.role !== 'gm'}
							<Button
								size="sm"
								variant={confirmingId === seat.participantId ? 'destructive' : 'outline'}
								disabled={busy}
								aria-label={confirmingId === seat.participantId
									? `Confirm removing ${seat.displayName}`
									: `Remove ${seat.displayName}`}
								onclick={() => handleRemove(seat)}
							>
								{#if confirmingId === seat.participantId}
									Really remove?
								{:else}
									<Trash2 class="h-4 w-4" />
								{/if}
							</Button>
						{/if}
					</li>
				{:else}
					<li class="text-sm text-muted-foreground">No seats yet.</li>
				{/each}
			</ul>

			<!-- Only where it answers something: a Player has no bin to press,
		     so the paragraph explaining what pressing it costs would be a
		     warning about a button that isn't there. -->
			{#if isGM}
				<p class="text-xs text-muted-foreground">
					Removing a seat signs out every device on it, and any token it owned goes back to
					belonging to nobody. {room.roomName || roomSlug} keeps everything else.
				</p>
			{/if}

			<!-- The palette sits open at the foot of the list rather than on a
		     popover off your own row, and that is a bug fix rather than a
		     preference: a bits-ui popover opened *inside* a dialog comes out
		     `position: static` with `opacity: 0` — unpositioned, invisible
		     and behind the dialog's own overlay, which then eats every click
		     aimed at it. It stays in the accessibility tree throughout, so
		     `getByRole('radio')` finds it and a spec asserting on the picked
		     colour can pass while nobody could have clicked it. Anything
		     that wants to pop up inside a dialog has the same trap waiting.
		     Everything in the room that pops up over the *map* is still on
		     the popover primitive.

		     Sitting open costs nothing here: it is one row of the dialog,
		     and the seats above it are exactly the "who else is wearing
		     what" this is chosen against. -->
			<div class="flex flex-col gap-2 border-t pt-4">
				<Label>Your colour</Label>
				<IdentityColorPicker
					value={room.colorOf(room.you?.participantId)}
					taken={seats
						.filter((seat) => seat.participantId !== room.you?.participantId)
						.map((seat) => seatColor(seat))}
					onpick={(color) => room.setColor(color)}
				/>
			</div>
		</div>
	</Dialog.Content>
</Dialog.Root>
