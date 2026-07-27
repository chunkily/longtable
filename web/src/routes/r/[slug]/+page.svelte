<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import { gmLogin, joinRoom, type Session } from '$lib/api';
	import { loadSession, saveSession } from '$lib/session';
	import { RoomClient } from '$lib/room.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import GameCanvas from '$lib/components/game-canvas.svelte';
	import CreateSceneDialog from '$lib/components/create-scene-dialog.svelte';
	import CreateTokenDialog from '$lib/components/create-token-dialog.svelte';

	const slug = $derived(page.params.slug ?? '');

	let session = $state<Session | null>(null);
	let client = $state<RoomClient | null>(null);

	let mode = $state<'player' | 'gm'>('player');
	let displayName = $state('');
	let password = $state('');
	let joining = $state(false);

	let chatText = $state('');
	let fogToolActive = $state(false);

	onMount(() => {
		const existing = loadSession(slug);
		if (existing) startSession(existing);
	});

	onDestroy(() => {
		client?.disconnect();
	});

	$effect(() => {
		if (client?.error) toast.error(client.error);
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
		try {
			const s =
				mode === 'gm'
					? await gmLogin(slug, displayName, password)
					: await joinRoom(slug, { displayName });
			saveSession(s);
			startSession(s);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to join');
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

	const statusVariant = $derived(
		client?.status === 'open'
			? 'secondary'
			: client?.status === 'closed'
				? 'destructive'
				: 'outline'
	);
	const isGM = $derived(client?.you?.role === 'gm');
</script>

{#if !session || !client}
	<div class="mx-auto max-w-md p-6">
		<Card.Root>
			<Card.Header>
				<Card.Title>Join room</Card.Title>
				<Card.Description>{slug}</Card.Description>
			</Card.Header>
			<Card.Content>
				<form class="flex flex-col gap-4" onsubmit={handleJoin}>
					<div class="flex gap-2">
						<Button
							type="button"
							variant={mode === 'player' ? 'default' : 'outline'}
							onclick={() => (mode = 'player')}
							class="flex-1"
						>
							Player
						</Button>
						<Button
							type="button"
							variant={mode === 'gm' ? 'default' : 'outline'}
							onclick={() => (mode = 'gm')}
							class="flex-1"
						>
							I'm the GM
						</Button>
					</div>
					<div class="flex flex-col gap-2">
						<Label for="display-name">Your name</Label>
						<Input id="display-name" bind:value={displayName} required />
					</div>
					{#if mode === 'gm'}
						<div class="flex flex-col gap-2">
							<Label for="gm-password">GM password</Label>
							<Input id="gm-password" type="password" bind:value={password} required />
						</div>
					{/if}
					<Button type="submit" disabled={joining}>{joining ? 'Joining…' : 'Join'}</Button>
				</form>
			</Card.Content>
		</Card.Root>
	</div>
{:else}
	<div class="mx-auto flex max-w-7xl flex-col gap-4 p-6">
		<header class="flex flex-wrap items-center gap-2">
			<h1 class="text-2xl font-bold tracking-tight">{client.roomName || slug}</h1>
			<Badge variant="outline">{client.you?.role}</Badge>
			<Badge variant={statusVariant}>{client.status}</Badge>
			<span class="text-sm text-muted-foreground">
				playing as <strong>{client.you?.displayName}</strong>
			</span>
		</header>

		<div class="flex flex-wrap items-start gap-4">
			<div class="flex flex-col gap-2">
				{#if isGM}
					<div class="flex flex-wrap gap-2">
						<CreateSceneDialog
							room={client}
							roomSlug={session.roomSlug}
							sessionToken={session.sessionToken}
						/>
						{#if client.scene}
							<CreateTokenDialog
								room={client}
								sceneId={client.scene.id}
								roomSlug={session.roomSlug}
								sessionToken={session.sessionToken}
							/>
							<Button
								variant={fogToolActive ? 'default' : 'outline'}
								onclick={() => (fogToolActive = !fogToolActive)}
							>
								{fogToolActive ? 'Painting fog…' : 'Reveal fog'}
							</Button>
						{/if}
					</div>
				{/if}
				{#if client.scene}
					<GameCanvas room={client} {fogToolActive} />
				{:else}
					<Card.Root class="flex h-64 w-[800px] max-w-full items-center justify-center">
						<p class="text-sm text-muted-foreground">
							{isGM ? 'Create a scene to get started.' : 'Waiting for the GM to start a scene…'}
						</p>
					</Card.Root>
				{/if}
			</div>

			<Card.Root class="w-full max-w-sm">
				<Card.Content class="flex flex-col gap-3">
					<ul class="flex max-h-96 flex-col gap-2 overflow-y-auto">
						{#each client.messages as msg (msg.id)}
							<li
								class={[
									'rounded-md px-2 py-1 text-sm',
									msg.kind === 'roll' && 'bg-accent text-accent-foreground'
								]}
							>
								<strong>{msg.participantName}:</strong>
								{#if msg.kind === 'roll'}
									{msg.body} → <strong>{msg.rollResult}</strong>
									<span class="text-xs text-muted-foreground">({msg.rollBreakdown})</span>
								{:else}
									{msg.body}
								{/if}
							</li>
						{/each}
					</ul>
					<form class="flex gap-2" onsubmit={handleSendChat}>
						<Input
							bind:value={chatText}
							placeholder="Say something, or /roll 2d6+3"
							autocomplete="off"
							class="flex-1"
						/>
						<Button type="submit">Send</Button>
					</form>
				</Card.Content>
			</Card.Root>
		</div>
	</div>
{/if}
