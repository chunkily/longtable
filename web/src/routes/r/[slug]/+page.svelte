<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { gmLogin, joinRoom, type Session } from '$lib/api';
	import { loadSession, saveSession } from '$lib/session';
	import { RoomClient } from '$lib/room.svelte';

	const slug = $derived(page.params.slug ?? '');

	let session = $state<Session | null>(null);
	let client = $state<RoomClient | null>(null);

	let mode = $state<'player' | 'gm'>('player');
	let displayName = $state('');
	let password = $state('');
	let joining = $state(false);
	let joinError = $state('');

	let chatText = $state('');

	onMount(() => {
		const existing = loadSession(slug);
		if (existing) startSession(existing);
	});

	onDestroy(() => {
		client?.disconnect();
	});

	function startSession(s: Session) {
		session = s;
		const c = new RoomClient();
		c.connect(s.roomSlug, s.sessionToken);
		client = c;
	}

	async function handleJoin(event: SubmitEvent) {
		event.preventDefault();
		joining = true;
		joinError = '';
		try {
			const s =
				mode === 'gm'
					? await gmLogin(slug, displayName, password)
					: await joinRoom(slug, { displayName });
			saveSession(s);
			startSession(s);
		} catch (err) {
			joinError = err instanceof Error ? err.message : 'failed to join';
		} finally {
			joining = false;
		}
	}

	function handleSendChat(event: SubmitEvent) {
		event.preventDefault();
		const text = chatText.trim();
		if (!text || !client) return;
		client.sendChat(text);
		chatText = '';
	}
</script>

{#if !session || !client}
	<h1>Join room</h1>
	<form onsubmit={handleJoin}>
		<fieldset>
			<label>
				<input type="radio" name="mode" value="player" bind:group={mode} />
				Player
			</label>
			<label>
				<input type="radio" name="mode" value="gm" bind:group={mode} />
				I'm the GM
			</label>
		</fieldset>
		<label>
			Your name
			<input type="text" bind:value={displayName} required />
		</label>
		{#if mode === 'gm'}
			<label>
				GM password
				<input type="password" bind:value={password} required />
			</label>
		{/if}
		{#if joinError}
			<p class="error">{joinError}</p>
		{/if}
		<button type="submit" disabled={joining}>{joining ? 'Joining…' : 'Join'}</button>
	</form>
{:else}
	<header>
		<h1>{client.roomName || slug}</h1>
		<p class="status">
			Playing as <strong>{client.you?.displayName}</strong> ({client.you?.role}) — {client.status}
		</p>
		{#if client.error}
			<p class="error">{client.error}</p>
		{/if}
	</header>

	<section class="chat">
		<ul class="log">
			{#each client.messages as msg (msg.id)}
				<li class={msg.kind}>
					<strong>{msg.participantName}:</strong>
					{#if msg.kind === 'roll'}
						<span class="roll-body">{msg.body}</span> → <strong>{msg.rollResult}</strong>
						<span class="breakdown">({msg.rollBreakdown})</span>
					{:else}
						{msg.body}
					{/if}
				</li>
			{/each}
		</ul>
		<form onsubmit={handleSendChat}>
			<input
				type="text"
				bind:value={chatText}
				placeholder="Say something, or /roll 2d6+3"
				autocomplete="off"
			/>
			<button type="submit">Send</button>
		</form>
	</section>
{/if}

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		max-width: 24rem;
	}
	fieldset {
		display: flex;
		gap: 1rem;
		border: none;
		padding: 0;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.status {
		color: #555;
	}
	.error {
		color: #b00020;
	}
	.chat {
		max-width: 40rem;
	}
	.log {
		list-style: none;
		margin: 0 0 1rem;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		max-height: 24rem;
		overflow-y: auto;
		border: 1px solid #ddd;
		border-radius: 4px;
		padding: 0.75rem;
	}
	.log li.roll {
		background: #f5f2ff;
	}
	.breakdown {
		color: #777;
		font-size: 0.85em;
	}
	.chat form {
		flex-direction: row;
	}
	.chat input[type='text'] {
		flex: 1;
	}
</style>
