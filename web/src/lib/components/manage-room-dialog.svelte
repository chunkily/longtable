<script lang="ts">
	// GM-level settings for the room itself, rather than for this scene.
	//
	// Seats came first and the movement lock is the second thing here.
	// Room privacy and deleting the room are each still their own open
	// backlog item.
	import { toast } from 'svelte-sonner';
	import { addSeat, listSeats, removeSeat, type Seat } from '$lib/api';
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
		open = $bindable(false)
	}: { room: RoomClient; roomSlug: string; sessionToken: string; open?: boolean } = $props();

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
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Manage room</Dialog.Title>
			<Dialog.Description>
				Seats are how people are known in this room. A seat outlives a browser, so someone on a new
				device takes theirs back rather than joining as a stranger.
			</Dialog.Description>
		</Dialog.Header>

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

		<form class="flex items-end gap-2" onsubmit={handleAdd}>
			<div class="flex flex-1 flex-col gap-2">
				<Label for="new-seat">Add a seat</Label>
				<!-- A GM can set the table before anyone arrives: a named chair
				     with nobody signed into it, waiting to be claimed. -->
				<Input id="new-seat" bind:value={newName} placeholder="Player's name" autocomplete="off" />
			</div>
			<Button type="submit" disabled={busy || !newName.trim()}>Add</Button>
		</form>

		<ul class="flex max-h-72 flex-col gap-2 overflow-y-auto">
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
			Removing a seat signs out every device on it, and any token it owned goes back to belonging to
			nobody. {room.roomName || roomSlug} keeps everything else.
		</p>
	</Dialog.Content>
</Dialog.Root>
