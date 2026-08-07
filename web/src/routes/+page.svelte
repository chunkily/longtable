<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { createRoom } from '$lib/api';
	import { parseInvite } from '$lib/invite';
	import { clearSession, listSessions, saveSession, type StoredSession } from '$lib/session';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';

	// The rooms this browser has been in. Read on mount rather than at
	// module scope because localStorage doesn't exist while prerendering.
	let rooms = $state<StoredSession[]>([]);

	let invite = $state('');

	let roomName = $state('');
	let gmName = $state('');
	let password = $state('');
	let creating = $state(false);

	onMount(() => {
		rooms = listSessions();
	});

	function handleInvite(event: SubmitEvent) {
		event.preventDefault();
		const slug = parseInvite(invite);
		if (!slug) {
			toast.error("That doesn't look like an invite link or code.");
			return;
		}
		void goto(resolve('/r/[slug]', { slug }));
	}

	// Only forgets the room here. The room itself, and everyone else's
	// list, are untouched — this is the browser's own record.
	function forget(slug: string) {
		clearSession(slug);
		rooms = listSessions();
	}

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		creating = true;
		try {
			const session = await createRoom(roomName, gmName, password);
			saveSession(session);
			await goto(resolve('/r/[slug]', { slug: session.roomSlug }));
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to create room');
		} finally {
			creating = false;
		}
	}
</script>

<div class="mx-auto flex max-w-3xl flex-col gap-8 p-6">
	<header>
		<h1 class="text-3xl font-bold tracking-tight">Longtable</h1>
		<p class="text-muted-foreground">Open source virtual tabletop for Dungeons and Dragons.</p>
	</header>

	<Card.Root>
		<Card.Header>
			<Card.Title>Your rooms</Card.Title>
		</Card.Header>
		<!-- A landmark so this region can be addressed on its own. "GM"
		     appears both as a badge here and in the create form's "GM
		     password" label below, and without something to scope to, the
		     two are indistinguishable to anything reading the page. -->
		<Card.Content class="flex flex-col gap-4" role="region" aria-label="Your rooms">
			{#if rooms.length === 0}
				<!-- The first thing anyone sees on a fresh browser, so it says
				     how to get into a room rather than reporting that there are
				     none. Nobody arrives here expecting a list; they arrive
				     having been sent a link, or wanting to start a table. -->
				<div class="rounded-md border border-dashed p-6 text-center">
					<p class="text-sm font-medium">Nothing here yet</p>
					<p class="mx-auto mt-1 max-w-sm text-sm text-muted-foreground">
						Rooms you join show up here. Ask your GM for the invite link, or start your own table
						below.
					</p>
				</div>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each rooms as room (room.roomSlug)}
						<li class="flex items-center gap-3 rounded-md border p-3">
							<a
								href={resolve('/r/[slug]', { slug: room.roomSlug })}
								class="min-w-0 flex-1 text-sm font-medium underline-offset-4 hover:underline"
							>
								<span class="block truncate">{room.roomName}</span>
								<span class="block truncate text-xs font-normal text-muted-foreground">
									as {room.displayName}
								</span>
							</a>
							<Badge variant={room.role === 'gm' ? 'default' : 'secondary'}>
								{room.role === 'gm' ? 'GM' : 'Player'}
							</Badge>
							<Button
								variant="ghost"
								size="sm"
								aria-label="Forget {room.roomName}"
								onclick={() => forget(room.roomSlug)}
							>
								Forget
							</Button>
						</li>
					{/each}
				</ul>
			{/if}

			<form class="flex flex-col gap-2 border-t pt-4" onsubmit={handleInvite}>
				<Label for="invite">Have an invite?</Label>
				<div class="flex gap-2">
					<Input
						id="invite"
						bind:value={invite}
						placeholder="Link or code"
						autocomplete="off"
						autocapitalize="off"
						spellcheck={false}
					/>
					<Button type="submit" variant="outline">Join</Button>
				</div>
			</form>
		</Card.Content>
	</Card.Root>

	<Card.Root>
		<Card.Header>
			<Card.Title>Create a room</Card.Title>
		</Card.Header>
		<Card.Content>
			<form class="flex flex-col gap-4" onsubmit={handleCreate}>
				<div class="flex flex-col gap-2">
					<Label for="room-name">Room name</Label>
					<Input id="room-name" bind:value={roomName} required />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="gm-name">Your name (GM)</Label>
					<Input id="gm-name" bind:value={gmName} required />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="gm-password">GM password</Label>
					<Input id="gm-password" type="password" bind:value={password} minlength={4} required />
					<p class="text-xs text-muted-foreground">
						This password lets you reclaim GM control from another device later — it isn't needed to
						play. If you lose it, whoever runs the server can reset it.
					</p>
				</div>
				<Button type="submit" disabled={creating} class="self-start">
					{creating ? 'Creating…' : 'Create room'}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
