<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { createRoom } from '$lib/api';
	import { parseRoomCode } from '$lib/room-code';
	import { clearSession, listSessions, saveSession, type StoredSession } from '$lib/session';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';

	/**
	 * Which question the page is asking.
	 *
	 * It used to ask all of them at once, and the first thing anyone ever
	 * saw was an empty list of their rooms with a one-line box tucked under
	 * it. The two things a newcomer can actually do were the smallest
	 * things on the page, and the largest was a report that they had
	 * nothing — so the page now asks one question at a time, the way the
	 * room's own join screen does.
	 */
	type HomeStep = 'welcome' | 'join' | 'create';
	let step = $state<HomeStep>('welcome');

	// The rooms this browser has been in. Read on mount rather than at
	// module scope because localStorage doesn't exist while prerendering.
	let rooms = $state<StoredSession[]>([]);

	let roomCode = $state('');

	let roomName = $state('');
	let gmName = $state('');
	let password = $state('');
	let creating = $state(false);

	onMount(() => {
		rooms = listSessions();
	});

	function handleJoin(event: SubmitEvent) {
		event.preventDefault();
		const slug = parseRoomCode(roomCode);
		if (!slug) {
			toast.error("That doesn't look like a room code.");
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

	<!-- Declared out here rather than inside a Card: a snippet written as a
	     component's child becomes one of its props. -->
	{#snippet back()}
		<div>
			<Button
				type="button"
				variant="ghost"
				size="sm"
				class="-ml-2"
				onclick={() => (step = 'welcome')}
			>
				<ChevronLeftIcon class="size-4" /> Back
			</Button>
		</div>
	{/snippet}

	{#if step === 'welcome'}
		<!-- Only rendered once there are rooms to put in it. An empty list is
		     a report that you have nothing, which is both true and useless to
		     the person it's addressed to — the two buttons below say what to
		     do instead, and on a fresh browser they are the whole page.

		     A landmark so this region can be addressed on its own. "GM"
		     appears both as a badge here and in the create form's "GM
		     password" label, and without something to scope to, the two are
		     indistinguishable to anything reading the page. -->
		{#if rooms.length > 0}
			<Card.Root>
				<Card.Header>
					<Card.Title>Your rooms</Card.Title>
				</Card.Header>
				<Card.Content role="region" aria-label="Your rooms">
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
				</Card.Content>
			</Card.Root>
		{/if}

		<!-- The two things anyone can do from here, at the same weight,
		     because which one you want depends entirely on whether somebody
		     already sent you a code — and the page has no way of knowing. -->
		<div class="grid gap-4 sm:grid-cols-2">
			<Button
				type="button"
				variant="outline"
				class="h-auto flex-col items-start gap-1.5 p-6 text-left whitespace-normal"
				onclick={() => (step = 'join')}
			>
				<span class="flex items-center gap-2 text-base font-semibold">
					<LogInIcon class="size-5" /> Join a room
				</span>
				<span class="text-sm font-normal text-muted-foreground">
					Someone at the table sent you a room code. This is where it goes.
				</span>
			</Button>
			<Button
				type="button"
				variant="outline"
				class="h-auto flex-col items-start gap-1.5 p-6 text-left whitespace-normal"
				onclick={() => (step = 'create')}
			>
				<span class="flex items-center gap-2 text-base font-semibold">
					<PlusIcon class="size-5" /> Create a room
				</span>
				<span class="text-sm font-normal text-muted-foreground">
					Start a new table. You'll be its GM, and you'll get a code to hand out.
				</span>
			</Button>
		</div>
	{:else if step === 'join'}
		<Card.Root>
			<Card.Header>
				<Card.Title>Join a room</Card.Title>
				<Card.Description>
					Rooms aren't listed anywhere, so the code is the only way in.
				</Card.Description>
			</Card.Header>
			<Card.Content class="flex flex-col gap-4">
				{@render back()}
				<form class="flex flex-col gap-2" onsubmit={handleJoin}>
					<Label for="room-code">Room code</Label>
					<div class="flex gap-2">
						<Input
							id="room-code"
							bind:value={roomCode}
							placeholder="7wdbtb"
							autocomplete="off"
							autocapitalize="off"
							spellcheck={false}
						/>
						<Button type="submit">Join room</Button>
					</div>
					<!-- A code reaches people as whatever their group already
					     pastes at each other, so saying a whole link works saves
					     the person who has one from trimming it by hand. -->
					<p class="text-xs text-muted-foreground">
						Six characters. A link to the room works too — paste the whole thing.
					</p>
				</form>
			</Card.Content>
		</Card.Root>
	{:else}
		<Card.Root>
			<Card.Header>
				<Card.Title>Create a room</Card.Title>
				<Card.Description>You'll be the GM of this one.</Card.Description>
			</Card.Header>
			<Card.Content class="flex flex-col gap-4">
				{@render back()}
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
							This password lets you reclaim GM control from another device later — it isn't needed
							to play. If you lose it, whoever runs the server can reset it.
						</p>
					</div>
					<Button type="submit" disabled={creating} class="self-start">
						{creating ? 'Creating…' : 'Create room'}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>
	{/if}
</div>
