<script lang="ts">
	// GM-level settings for the room itself, rather than for this scene.
	//
	// Seats used to be the bulk of this and are their own dialog now
	// (`seats-dialog.svelte`), because reading the roster isn't a GM power
	// and this dialog is. What's left is what only a GM can do: the
	// movement lock, the room's own password, and deleting the room. Room
	// privacy is still its own open backlog item.
	import { toast } from 'svelte-sonner';
	import { deleteRoom, setGMPassword } from '$lib/api';
	import type { RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';

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
	// The same arm-then-fire the seat bins use, and the only one in the
	// app protecting something with no undo at all.
	let confirmingDelete = $state(false);

	$effect(() => {
		if (open) {
			confirmingDelete = false;
			// Half a password left in the box from last time is a trap: it
			// looks like the current one and it is not.
			newPassword = '';
			repeatPassword = '';
		}
	});

	async function handlePassword(event: SubmitEvent) {
		event.preventDefault();
		if (!passwordReady) return;
		busy = true;
		try {
			await setGMPassword(roomSlug, sessionToken, newPassword);
			newPassword = '';
			repeatPassword = '';
			// The only toast in this dialog that isn't an error, because this
			// is the only thing in it that changes nothing you can see: the
			// movement buttons swap over, a deleted room takes you out of it,
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
			<Dialog.Description>These apply to the whole room, not just this scene.</Dialog.Description>
		</Dialog.Header>

		<!-- Three sections, each announced by a heading over a hairline. It
		     read as one long column of controls before that: the password
		     boxes carried field labels and nothing said where the password
		     *began*, so the form under "Moving tokens" looked like more of the
		     same setting. The headings are also where the next GM-only switch
		     goes — Player token creation is still open in the backlog, and it
		     belongs under Token permissions rather than at the end of a list. -->
		<div class="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto">
			<section class="flex flex-col gap-2">
				<h3 class="text-sm font-medium">Token permissions</h3>
				<!-- Two buttons rather than a switch: there is no switch component in
			     this project, and the pair says what each state *means* — which
			     matters more here than switch-ness, because "on" and "off" are
			     not obvious names for a rule about other people's tokens. -->
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
			</section>

			<!-- Rotating the room password from inside the room, rather than
		     asking whoever runs the server to do it. The current one isn't
		     asked for: the session proves the seat, the same as every other
		     control in here (ADR-0007). -->
			<section class="flex flex-col gap-2 border-t pt-4">
				<h3 class="text-sm font-medium">GM password</h3>
				<form class="flex flex-col gap-2" onsubmit={handlePassword}>
					<!-- `New password`, not `New GM password`: the heading above it
				     already said whose. -->
					<Label for="new-gm-password">New password</Label>
					<Input
						id="new-gm-password"
						type="password"
						bind:value={newPassword}
						minlength={MIN_PASSWORD}
						autocomplete="new-password"
					/>
					<Label for="repeat-gm-password">Confirm password</Label>
					<Input
						id="repeat-gm-password"
						type="password"
						bind:value={repeatPassword}
						minlength={MIN_PASSWORD}
						autocomplete="new-password"
					/>
					{#if repeatPassword && repeatPassword !== newPassword}
						<p class="text-xs font-medium text-destructive" role="alert">
							Both boxes have to match.
						</p>
					{/if}
					<p class="text-xs text-muted-foreground">
						Everyone stays signed in, including you. The next GM login needs the new password.
					</p>
					<Button type="submit" class="self-start" disabled={busy || !passwordReady}>Save</Button>
				</form>
			</section>

			<!-- Last, and set apart, because it is the one thing in this app
		     that can't be undone. Everything else destructive here is a
		     seat, a stroke or a token, and every one of those comes back. -->
			<section class="flex flex-col gap-2 border-t pt-4">
				<h3 class="text-sm font-medium">Delete room</h3>
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
			</section>
		</div>
	</Dialog.Content>
</Dialog.Root>
