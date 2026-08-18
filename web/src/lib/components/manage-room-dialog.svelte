<script lang="ts">
	// GM-level settings for the room itself, rather than for this scene.
	//
	// Seats came first, then the movement lock, the room's own password,
	// and deleting the room. Room privacy is still its own open backlog
	// item.
	import { toast } from 'svelte-sonner';
	import { addSeat, deleteRoom, listSeats, removeSeat, setGMPassword, type Seat } from '$lib/api';
	import { identityHex, suggestedColor } from '$lib/identity-color';
	import type { RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import * as Dialog from '$lib/components/ui/dialog';
	import Trash2 from '@lucide/svelte/icons/trash-2';

	let {
		room,
		roomSlug,
		sessionToken,
		ondeleted,
		open = $bindable(false)
	}: {
		room: RoomClient;
		roomSlug: string;
		sessionToken: string;
		/**
		 * What to do once the room is gone. The page owns that, because
		 * everyone else in the room is told over the socket and ends up on
		 * the same path — this callback only covers the one person whose
		 * own click did it, who shouldn't be left staring at a map of a
		 * room that no longer exists if their socket happens to be down.
		 */
		ondeleted?: () => void;
		open?: boolean;
	} = $props();

	let seats = $state<Seat[]>([]);
	let newName = $state('');
	let busy = $state(false);
	// Typed twice, because getting it wrong is not recoverable from
	// inside the room: the next GM login would need the password that was
	// actually saved, and nobody knows what that was. The Host can reset
	// it from the command line, which is a worse afternoon than a second
	// box.
	let newPassword = $state('');
	let repeatPassword = $state('');
	// The server's own rule, mirrored so the button says no before the
	// round trip does (`minGMPasswordLength` in internal/api/rooms.go).
	const MIN_PASSWORD = 4;
	const passwordReady = $derived(
		newPassword.length >= MIN_PASSWORD && newPassword === repeatPassword
	);
	// Removing a seat takes its tokens' owner with it, so it arms in
	// place rather than firing on one click — the same two-step deleting
	// a scene uses.
	let confirmingId = $state<string | null>(null);
	// The same arm-then-fire the seat bins use, and the only one in the
	// app protecting something with no undo at all.
	let confirmingDelete = $state(false);

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
			confirmingDelete = false;
			// Half a password left in the box from last time is a trap: it
			// looks like the current one and it is not.
			newPassword = '';
			repeatPassword = '';
			refresh();
		}
	});

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

	async function handlePassword(event: SubmitEvent) {
		event.preventDefault();
		if (!passwordReady) return;
		busy = true;
		try {
			await setGMPassword(roomSlug, sessionToken, newPassword);
			newPassword = '';
			repeatPassword = '';
			// The only toast in this dialog that isn't an error, because this
			// is the only thing in it that changes nothing you can see: a
			// seat added shows up in the list, the movement buttons swap over,
			// and a password does neither.
			toast.success('Password changed');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to change the password');
		} finally {
			busy = false;
		}
	}

	async function handleDelete() {
		if (!confirmingDelete) {
			confirmingDelete = true;
			return;
		}
		busy = true;
		try {
			await deleteRoom(roomSlug, sessionToken);
			open = false;
			ondeleted?.();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to delete the room');
		} finally {
			busy = false;
			confirmingDelete = false;
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
	     body, the same shape token-detail-dialog.svelte settled on. This
	     dialog grew a password form and a delete section, and on a laptop
	     the last control in it sat below the bottom edge of the screen
	     with nothing to say so — Playwright found it before a person did,
	     reporting the button as "outside of the viewport". -->
	<Dialog.Content class="flex max-h-[calc(100dvh-2rem)] flex-col">
		<Dialog.Header>
			<Dialog.Title>Manage room</Dialog.Title>
			<Dialog.Description>
				Seats are how people are known in this room. A seat outlives a browser, so someone on a new
				device takes theirs back rather than joining as a stranger.
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto">
			<!-- Two buttons rather than a switch: there is no switch component in
		     this project, and the pair says what each state *means* — which
		     matters more here than switch-ness, because "on" and "off" are
		     not obvious names for a rule about other people's tokens. -->
			<div class="flex flex-col gap-2">
				<Label id="movement-label">Moving tokens</Label>
				<div class="flex gap-2" role="group" aria-labelledby="movement-label">
					<Button
						type="button"
						class="flex-1"
						variant={room.ownerOnlyMovement ? 'outline' : 'default'}
						aria-pressed={!room.ownerOnlyMovement}
						onclick={() => room.setOwnerOnlyMovement(false)}
					>
						Anyone moves anything
					</Button>
					<Button
						type="button"
						class="flex-1"
						variant={room.ownerOnlyMovement ? 'default' : 'outline'}
						aria-pressed={room.ownerOnlyMovement}
						onclick={() => room.setOwnerOnlyMovement(true)}
					>
						Only the owner
					</Button>
				</div>
				<p class="text-xs text-muted-foreground">
					{#if room.ownerOnlyMovement}
						A Player can drag only the tokens they own. You can still move everything.
					{:else}
						Anyone at the table can drag any token, including yours.
					{/if}
				</p>
			</div>

			<!-- Rotating the room password from inside the room, rather than
		     asking whoever runs the server to do it. The current one isn't
		     asked for: the session proves the seat, the same as every other
		     control in here (ADR-0007). -->
			<form class="flex flex-col gap-2" onsubmit={handlePassword}>
				<Label for="new-gm-password">New GM password</Label>
				<Input
					id="new-gm-password"
					type="password"
					bind:value={newPassword}
					minlength={MIN_PASSWORD}
					autocomplete="new-password"
				/>
				<Label for="repeat-gm-password">Type it again</Label>
				<Input
					id="repeat-gm-password"
					type="password"
					bind:value={repeatPassword}
					minlength={MIN_PASSWORD}
					autocomplete="new-password"
				/>
				{#if repeatPassword && repeatPassword !== newPassword}
					<p class="text-xs font-medium text-destructive" role="alert">Both boxes have to match.</p>
				{/if}
				<p class="text-xs text-muted-foreground">
					Everyone stays signed in, including you. The next GM login needs the new password.
				</p>
				<Button type="submit" class="self-start" disabled={busy || !passwordReady}>Save</Button>
			</form>

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

			<ul class="flex flex-col gap-2">
				{#each seats as seat (seat.participantId)}
					<li class="flex flex-wrap items-center gap-2 rounded-md border p-2">
						{#if identityHex(seat.color)}
							<span
								class="h-3 w-3 shrink-0 rounded-full"
								style="background-color: {identityHex(seat.color)}"
							></span>
						{/if}
						<span class="min-w-0 flex-1 truncate text-sm">{seat.displayName}</span>
						{#if seat.role === 'gm'}
							<Badge>GM</Badge>
						{/if}
						{#if seat.connected}
							<Badge variant="secondary">here now</Badge>
						{/if}
						<!-- The GM's own seat can't go: the room password signs you
					     into it, so removing it would strand the only role that
					     could undo the damage. The server refuses it too. -->
						{#if seat.role !== 'gm'}
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

			<p class="text-xs text-muted-foreground">
				Removing a seat signs out every device on it, and any token it owned goes back to belonging
				to nobody. {room.roomName || roomSlug} keeps everything else.
			</p>

			<!-- Last, and set apart, because it is the one thing in this app
		     that can't be undone. Everything else destructive here is a
		     seat, a stroke or a token, and every one of those comes back. -->
			<div class="flex flex-col gap-2 border-t pt-4">
				<p class="text-xs text-muted-foreground">
					{room.roomName || roomSlug} goes for everyone, with its scenes, tokens, chat and seats. Images
					you uploaded stay on the server. There's no undo.
				</p>
				<Button
					type="button"
					variant="destructive"
					class="self-start"
					disabled={busy}
					onclick={handleDelete}
				>
					{#if confirmingDelete}
						Really delete this room?
					{:else}
						Delete room
					{/if}
				</Button>
			</div>
		</div>
	</Dialog.Content>
</Dialog.Root>
