<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createRoom, listRooms, type RoomSummary } from '$lib/api';
	import { saveSession } from '$lib/session';

	let rooms = $state<RoomSummary[]>([]);
	let loadError = $state('');

	let roomName = $state('');
	let gmName = $state('');
	let password = $state('');
	let creating = $state(false);
	let createError = $state('');

	onMount(async () => {
		try {
			rooms = await listRooms();
		} catch (err) {
			loadError = err instanceof Error ? err.message : 'failed to load rooms';
		}
	});

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		creating = true;
		createError = '';
		try {
			const session = await createRoom(roomName, gmName, password);
			saveSession(session);
			await goto(resolve('/r/[slug]', { slug: session.roomSlug }));
		} catch (err) {
			createError = err instanceof Error ? err.message : 'failed to create room';
		} finally {
			creating = false;
		}
	}
</script>

<h1>Longtable</h1>
<p>Open source virtual tabletop for Dungeons and Dragons.</p>

<section>
	<h2>Join a room</h2>
	{#if loadError}
		<p class="error">{loadError}</p>
	{:else if rooms.length === 0}
		<p>No rooms yet — create one below.</p>
	{:else}
		<ul>
			{#each rooms as room (room.slug)}
				<li>
					<a href={resolve('/r/[slug]', { slug: room.slug })}>{room.name}</a>
				</li>
			{/each}
		</ul>
	{/if}
</section>

<section>
	<h2>Create a room</h2>
	<form onsubmit={handleCreate}>
		<label>
			Room name
			<input type="text" bind:value={roomName} required />
		</label>
		<label>
			Your name (GM)
			<input type="text" bind:value={gmName} required />
		</label>
		<label>
			GM password
			<input type="password" bind:value={password} minlength="4" required />
		</label>
		<p class="hint">
			This password lets you reclaim GM control from another device later — it isn't needed to play.
		</p>
		{#if createError}
			<p class="error">{createError}</p>
		{/if}
		<button type="submit" disabled={creating}>
			{creating ? 'Creating…' : 'Create room'}
		</button>
	</form>
</section>

<style>
	section {
		margin-top: 2rem;
	}
	form {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		max-width: 24rem;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.hint {
		font-size: 0.85rem;
		color: #666;
		margin: 0;
	}
	.error {
		color: #b00020;
	}
</style>
