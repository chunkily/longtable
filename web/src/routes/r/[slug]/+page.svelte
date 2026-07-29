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
	import ChevronUpIcon from '@lucide/svelte/icons/chevron-up';
	import GameCanvas, { type Tool } from '$lib/components/game-canvas.svelte';
	import CreateSceneDialog from '$lib/components/create-scene-dialog.svelte';
	import CreateTokenDialog from '$lib/components/create-token-dialog.svelte';

	const slug = $derived(page.params.slug ?? '');

	let session = $state<Session | null>(null);
	let client = $state<RoomClient | null>(null);
	let canvasRef:
		{ resetView: () => void; viewCenterCell: () => { x: number; y: number } } | undefined =
		$state();

	let mode = $state<'player' | 'gm'>('player');
	let displayName = $state('');
	let password = $state('');
	let joining = $state(false);

	let chatText = $state('');

	// The map toolbar: 'none' is plain pan/token-drag mode. Fog stays
	// GM-only (gated in the template below); drawing, pinging, and
	// erasing are open to everyone, since they're meant as a shared
	// pointer/annotation tool rather than GM-only map control. The
	// eraser is offered to everyone but reaches different drawings per
	// role: a GM can erase anyone's, a Player only their own.
	let activeTool = $state<Tool>('none');
	const STROKE_COLORS = [
		{ label: 'Black', value: '#000000' },
		{ label: 'Red', value: '#cc0000' },
		{ label: 'Green', value: '#008000' },
		{ label: 'Blue', value: '#0033cc' }
	];
	let strokeColor = $state(STROKE_COLORS[0].value);

	function selectTool(tool: Tool) {
		activeTool = activeTool === tool ? 'none' : tool;
	}
	// Below the lg breakpoint the chat panel isn't shown inline — it's a
	// bottom sheet toggled by the "Chat" bar, since there isn't room for
	// canvas + chat side by side there (see viewport-layout discussion).
	let mobileChatOpen = $state(false);

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
	{#snippet chatMessages(room: RoomClient, maxHeightClass: string)}
		<ul class={['flex flex-col gap-2 overflow-y-auto', maxHeightClass]}>
			{#each room.messages as msg (msg.id)}
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
	{/snippet}

	{#snippet chatForm()}
		<form class="flex gap-2" onsubmit={handleSendChat}>
			<Input
				bind:value={chatText}
				placeholder="Say something, or /roll 2d6+3"
				autocomplete="off"
				class="flex-1"
			/>
			<Button type="submit">Send</Button>
		</form>
	{/snippet}

	<div class="flex flex-col gap-4 p-6 pb-16 lg:pb-6">
		<header class="flex flex-wrap items-center gap-2">
			<h1 class="text-2xl font-bold tracking-tight">{client.roomName || slug}</h1>
			<Badge variant="outline">{client.you?.role}</Badge>
			<Badge variant={statusVariant}>{client.status}</Badge>
			<span class="text-sm text-muted-foreground">
				playing as <strong>{client.you?.displayName}</strong>
			</span>
		</header>

		<div class="flex flex-col gap-4 lg:flex-row lg:items-start">
			<div class="flex min-w-0 flex-1 flex-col gap-2">
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
								spawnCell={() => canvasRef?.viewCenterCell() ?? { x: 0, y: 0 }}
							/>
							<Button
								variant={activeTool === 'fog' ? 'default' : 'outline'}
								onclick={() => selectTool('fog')}
							>
								{activeTool === 'fog' ? 'Painting fog…' : 'Reveal fog'}
							</Button>
						{/if}
					</div>
				{/if}
				{#if client.scene}
					<div class="flex flex-wrap items-center justify-between gap-2">
						<div class="flex flex-wrap gap-2">
							<Button
								variant={activeTool === 'freehand' ? 'default' : 'outline'}
								size="sm"
								onclick={() => selectTool('freehand')}
							>
								Freehand
							</Button>
							<Button
								variant={activeTool === 'line' ? 'default' : 'outline'}
								size="sm"
								onclick={() => selectTool('line')}
							>
								Line
							</Button>
							<Button
								variant={activeTool === 'rect' ? 'default' : 'outline'}
								size="sm"
								onclick={() => selectTool('rect')}
							>
								Rectangle
							</Button>
							<Button
								variant={activeTool === 'ellipse' ? 'default' : 'outline'}
								size="sm"
								onclick={() => selectTool('ellipse')}
							>
								Ellipse
							</Button>
							<Button
								variant={activeTool === 'ping' ? 'default' : 'outline'}
								size="sm"
								onclick={() => selectTool('ping')}
							>
								Ping
							</Button>
							<Button
								variant={activeTool === 'eraser' ? 'default' : 'outline'}
								size="sm"
								title={isGM
									? 'Click a drawing to erase it'
									: 'Click one of your own drawings to erase it'}
								onclick={() => selectTool('eraser')}
							>
								Erase
							</Button>
							<div class="flex items-center gap-1 px-1">
								{#each STROKE_COLORS as opt (opt.value)}
									<button
										type="button"
										aria-label={opt.label}
										title={opt.label}
										class={[
											'h-6 w-6 rounded-full border-2',
											strokeColor === opt.value ? 'border-foreground' : 'border-transparent'
										]}
										style="background-color: {opt.value}"
										onclick={() => (strokeColor = opt.value)}
									></button>
								{/each}
							</div>
						</div>
						<Button variant="outline" size="sm" onclick={() => canvasRef?.resetView()}>
							Reset view
						</Button>
					</div>
					<GameCanvas room={client} {activeTool} {strokeColor} bind:this={canvasRef} />
				{:else}
					<Card.Root class="flex h-64 w-full items-center justify-center">
						<p class="text-sm text-muted-foreground">
							{isGM ? 'Create a scene to get started.' : 'Waiting for the GM to start a scene…'}
						</p>
					</Card.Root>
				{/if}
			</div>

			<Card.Root class="hidden w-full lg:flex lg:max-w-sm">
				<Card.Content class="flex flex-col gap-3">
					{@render chatMessages(client, 'max-h-96')}
					{@render chatForm()}
				</Card.Content>
			</Card.Root>
		</div>
	</div>

	<div class="fixed inset-x-0 bottom-0 lg:hidden">
		{#if mobileChatOpen}
			<div class="flex max-h-[60vh] flex-col gap-3 border-t bg-background p-4 shadow-lg">
				{@render chatMessages(client, 'flex-1')}
				{@render chatForm()}
			</div>
		{/if}
		<button
			type="button"
			class="flex w-full items-center justify-center gap-2 border-t bg-background py-2 text-sm font-medium"
			onclick={() => (mobileChatOpen = !mobileChatOpen)}
		>
			<ChevronUpIcon class={mobileChatOpen ? 'rotate-180' : ''} />
			Chat
		</button>
	</div>
{/if}
