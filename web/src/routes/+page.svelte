<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import { createRoom, listRooms, type RoomSummary } from '$lib/api';
	import { saveSession } from '$lib/session';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';

	let rooms = $state<RoomSummary[]>([]);

	let roomName = $state('');
	let gmName = $state('');
	let password = $state('');
	let creating = $state(false);

	onMount(async () => {
		try {
			rooms = await listRooms();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to load rooms');
		}
	});

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
			<Card.Title>Join a room</Card.Title>
		</Card.Header>
		<Card.Content>
			{#if rooms.length === 0}
				<p class="text-sm text-muted-foreground">No rooms yet — create one below.</p>
			{:else}
				<ul class="flex flex-col gap-2">
					{#each rooms as room (room.slug)}
						<li>
							<a
								href={resolve('/r/[slug]', { slug: room.slug })}
								class="text-primary underline-offset-4 hover:underline"
							>
								{room.name}
							</a>
						</li>
					{/each}
				</ul>
			{/if}
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
						play.
					</p>
				</div>
				<Button type="submit" disabled={creating} class="self-start">
					{creating ? 'Creating…' : 'Create room'}
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
