<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import { gmLogin, joinRoom, type Session } from '$lib/api';
	import { loadSession, saveSession } from '$lib/session';
	import { RoomClient, type Token } from '$lib/room.svelte';
	import { DEFAULT_LINE_WIDTH_FEET, LINE_WIDTH_CHOICES_FEET, type SnapMode } from '$lib/aoe';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import ChevronUpIcon from '@lucide/svelte/icons/chevron-up';
	import GameCanvas, { type Tool } from '$lib/components/game-canvas.svelte';
	import CreateSceneDialog from '$lib/components/create-scene-dialog.svelte';
	import SceneManagerDialog from '$lib/components/scene-manager-dialog.svelte';
	import CreateTokenDialog from '$lib/components/create-token-dialog.svelte';
	import TokenDetailDialog from '$lib/components/token-detail-dialog.svelte';
	import Pen from '@lucide/svelte/icons/pen';
	import Slash from '@lucide/svelte/icons/slash';
	import RectangleHorizontal from '@lucide/svelte/icons/rectangle-horizontal';
	import Circle from '@lucide/svelte/icons/circle';
	import MapPin from '@lucide/svelte/icons/map-pin';
	import Eraser from '@lucide/svelte/icons/eraser';
	import Ruler from '@lucide/svelte/icons/ruler';
	import Undo from '@lucide/svelte/icons/undo';
	import Redo from '@lucide/svelte/icons/redo';
	import RefreshCw from '@lucide/svelte/icons/refresh-cw';
	import CircleDot from '@lucide/svelte/icons/circle-dot';
	import Cone from '@lucide/svelte/icons/cone';
	import Minus from '@lucide/svelte/icons/minus';
	import Square from '@lucide/svelte/icons/square';
	import Trash2 from '@lucide/svelte/icons/trash-2';

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
	// GM-only (gated in the template below); drawing, pinging, measuring
	// and erasing are open to everyone, since they're meant as a shared
	// pointer/annotation tool rather than GM-only map control. The
	// eraser is offered to everyone but reaches different drawings per
	// role: a GM can erase anyone's, a Player only their own.
	let activeTool = $state<Tool>('none');
	// Where an area template's points may land. A setting rather than a
	// rule because tables genuinely differ — some put a burst on a cell
	// centre, some on an intersection, some eyeball it — and it never
	// leaves this client: the points that go on the wire are already
	// snapped, so nobody else needs to know which convention made them.
	let snapMode = $state<SnapMode>('intersections');
	const SNAP_MODES: { value: SnapMode; label: string }[] = [
		{ value: 'intersections', label: 'Corners' },
		{ value: 'centres', label: 'Centres' },
		{ value: 'free', label: 'Free' }
	];
	// A Line is the one shape a single drag can't describe: the drag
	// gives length and direction, never width.
	let lineWidthFeet = $state(DEFAULT_LINE_WIDTH_FEET);
	const isTemplateTool = $derived(activeTool.startsWith('template-'));
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

	// Which token this client is looking at. Local to this browser and
	// nothing more: it never goes on the wire, so two people can have
	// different tokens selected, and a reload starts with none. Owned here
	// rather than inside the canvas because the details section below
	// chat reads it too — the canvas binds it so a click can set it.
	let selectedTokenId = $state<string | null>(null);
	// Derived rather than kept in step by an effect. A selection whose
	// token has gone — the scene changed under it, or it was removed —
	// simply reads as nothing selected, with no second copy of the truth
	// to go stale. The id itself is left alone, so a token that comes back
	// under the same id comes back selected.
	const selectedToken = $derived<Token | null>(
		client?.tokens.find((t) => t.id === selectedTokenId) ?? null
	);
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

	// Undo/redo shortcuts. Bound to the window rather than the canvas,
	// which never holds focus — but that means catching keystrokes meant
	// for whatever the user is actually typing in, so anything with a
	// text cursor keeps its own undo behaviour.
	function handleKeydown(event: KeyboardEvent) {
		if (!client || !(event.ctrlKey || event.metaKey)) return;

		const target = event.target as HTMLElement | null;
		if (
			target?.isContentEditable ||
			['INPUT', 'TEXTAREA', 'SELECT'].includes(target?.tagName ?? '')
		) {
			return;
		}

		const key = event.key.toLowerCase();
		if (key === 'z' && !event.shiftKey) {
			event.preventDefault();
			client.undo();
		} else if ((key === 'z' && event.shiftKey) || key === 'y') {
			event.preventDefault();
			client.redo();
		}
	}

	const statusVariant = $derived(
		client?.status === 'open'
			? 'secondary'
			: client?.status === 'closed'
				? 'destructive'
				: 'outline'
	);
	// A dropped socket used to be invisible beyond the small status badge,
	// while every command silently did nothing. The banner is the one
	// place that says so plainly.
	const showConnectionBanner = $derived(
		!!client && client.status !== 'open' && client.status !== 'connecting'
	);
	const isGM = $derived(client?.you?.role === 'gm');
</script>

<svelte:window onkeydown={handleKeydown} />

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

	<!-- Who is actually at the table right now, which is a different list
	     from everyone who has ever joined: someone who played last week
	     and isn't online doesn't appear. Fixed above the chat in both
	     layouts, so it stays put when the panel grows tabs. -->
	{#snippet whoIsHere(room: RoomClient)}
		<section aria-label="Who's connected" class="flex flex-wrap items-center gap-1">
			{#each room.connectedParticipants as participant (participant.id)}
				<Badge variant={participant.role === 'gm' ? 'default' : 'secondary'}>
					{participant.displayName}{participant.role === 'gm' ? ' (GM)' : ''}
				</Badge>
			{:else}
				<!-- Only reachable before the first sync lands: you are always
				     connected to your own room. -->
				<span class="text-xs text-muted-foreground">Nobody connected.</span>
			{/each}
		</section>
	{/snippet}

	<!-- Fixed above the message list in both layouts — the desktop sidebar
	     and the mobile sheet — rather than scrolling away with the chat. -->
	{#snippet tokenDetails(room: RoomClient, token: Token | null)}
		<section aria-label="Selected token" class="flex items-center gap-2 rounded-md border p-2">
			{#if token}
				<div class="min-w-0 flex-1">
					<p class="truncate text-sm font-medium">{token.name}</p>
					<p class="text-xs text-muted-foreground">
						{token.width}×{token.height} squares{token.visibility === 'hidden'
							? ' · hidden from players'
							: ''}
					</p>
				</div>
				{#if isGM && session}
					<TokenDetailDialog
						{room}
						{token}
						roomSlug={session.roomSlug}
						sessionToken={session.sessionToken}
					/>
					<!-- Not behind a confirmation, unlike deleting a scene: the
					     deletion is undoable, which is the cheaper answer to a
					     misclick than a dialog on every deliberate one. -->
					<Button
						variant="outline"
						size="sm"
						aria-label="Delete token"
						title="Delete this token (Ctrl+Z to bring it back)"
						onclick={() => room.deleteToken(token.id)}
					>
						<Trash2 class="h-4 w-4" />
					</Button>
				{/if}
			{:else}
				<p class="text-sm text-muted-foreground">No token selected — click one on the map.</p>
			{/if}
		</section>
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

		{#if showConnectionBanner}
			<div
				role="status"
				class="flex flex-wrap items-center gap-3 rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm"
			>
				{#if client.sessionExpired}
					<span>
						This session is no longer valid — the room may have been reset. Rejoin to carry on.
					</span>
					<Button variant="outline" size="sm" onclick={() => location.reload()}>Rejoin</Button>
				{:else if client.reconnectExhausted}
					<span>Can't reach the table. Nothing you do will reach the others until it's back.</span>
					<Button variant="outline" size="sm" onclick={() => client?.reconnect()}>Try again</Button>
				{:else}
					<span>Reconnecting to the table…</span>
				{/if}
			</div>
		{/if}

		<div class="flex flex-col gap-4 lg:flex-row lg:items-start">
			<div class="flex min-w-0 flex-1 flex-col gap-2">
				{#if isGM}
					<div class="flex flex-wrap gap-2">
						<CreateSceneDialog
							room={client}
							roomSlug={session.roomSlug}
							sessionToken={session.sessionToken}
						/>
						<SceneManagerDialog
							room={client}
							roomSlug={session.roomSlug}
							sessionToken={session.sessionToken}
						/>
						{#if client.scene}
							<!-- Captured here because the handlers below read it inside a
							     closure, where the {#if}'s narrowing doesn't reach. -->
							{@const sceneId = client.scene.id}
							<CreateTokenDialog
								room={client}
								{sceneId}
								roomSlug={session.roomSlug}
								sessionToken={session.sessionToken}
								spawnCell={() => canvasRef?.viewCenterCell() ?? { x: 0, y: 0 }}
							/>
							<Button
								variant={activeTool === 'fog-reveal' ? 'default' : 'outline'}
								onclick={() => selectTool('fog-reveal')}
							>
								{activeTool === 'fog-reveal' ? 'Painting fog…' : 'Reveal fog'}
							</Button>
							<Button
								variant={activeTool === 'fog-hide' ? 'default' : 'outline'}
								onclick={() => selectTool('fog-hide')}
								title="Drag over revealed squares to cover them again"
							>
								{activeTool === 'fog-hide' ? 'Hiding fog…' : 'Hide fog'}
							</Button>
							<!-- The two bulk actions are one-shot buttons rather than
							     tools: neither has a gesture to make, so making them
							     modes would mean arming something that fires on the
							     next click anywhere on the map. -->
							<Button variant="outline" onclick={() => client?.revealAllFog(sceneId)}>
								Reveal all
							</Button>
							<!-- Deliberately not behind a confirmation. The story asks
							     for a single action, and the room has no confirm dialog
							     to borrow; the cost is that a misclick here drops a
							     session's worth of revealed fog with no undo, which is
							     worth fixing the first time someone actually hits it. -->
							<Button variant="outline" onclick={() => client?.resetFog(sceneId)}>Reset fog</Button>
						{/if}
					</div>
				{/if}
				{#if client.scene}
					<div class="flex flex-wrap items-center justify-between gap-2">
						<div class="flex flex-wrap gap-2">
							<Button
								variant={activeTool === 'freehand' ? 'default' : 'outline'}
								size="sm"
								aria-label="Freehand"
								onclick={() => selectTool('freehand')}
								title="Freehand drawing"
							>
								<Pen class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'line' ? 'default' : 'outline'}
								size="sm"
								aria-label="Line"
								onclick={() => selectTool('line')}
								title="Straight line drawing"
							>
								<Slash class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'rect' ? 'default' : 'outline'}
								size="sm"
								aria-label="Rectangle"
								onclick={() => selectTool('rect')}
								title="Rectangle drawing"
							>
								<RectangleHorizontal class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'ellipse' ? 'default' : 'outline'}
								size="sm"
								aria-label="Ellipse"
								onclick={() => selectTool('ellipse')}
								title="Ellipse drawing"
							>
								<Circle class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'ping' ? 'default' : 'outline'}
								size="sm"
								aria-label="Ping"
								onclick={() => selectTool('ping')}
								title="Ping the map"
							>
								<MapPin class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'measure' ? 'default' : 'outline'}
								size="sm"
								aria-label="Measure"
								onclick={() => selectTool('measure')}
								title="Drag to measure a distance in feet"
							>
								<Ruler class="h-4 w-4" />
							</Button>
							<!-- The four area templates. Six shapes in the rules, but
							     Sphere, Cylinder and Emanation are all a circle seen
							     from above, so they share one tool. -->
							<Button
								variant={activeTool === 'template-circle' ? 'default' : 'outline'}
								size="sm"
								aria-label="Circle template"
								onclick={() => selectTool('template-circle')}
								title="Sphere, cylinder or emanation — drag from the centre"
							>
								<CircleDot class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'template-cone' ? 'default' : 'outline'}
								size="sm"
								aria-label="Cone template"
								onclick={() => selectTool('template-cone')}
								title="Cone — drag from the point of origin"
							>
								<Cone class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'template-line' ? 'default' : 'outline'}
								size="sm"
								aria-label="Line template"
								onclick={() => selectTool('template-line')}
								title="Line — drag its length; set its width below"
							>
								<Minus class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'template-cube' ? 'default' : 'outline'}
								size="sm"
								aria-label="Cube template"
								onclick={() => selectTool('template-cube')}
								title="Cube — drag one corner to the opposite corner"
							>
								<Square class="h-4 w-4" />
							</Button>
							<Button
								variant={activeTool === 'eraser' ? 'default' : 'outline'}
								size="sm"
								title={isGM
									? 'Click a drawing to erase it'
									: 'Click one of your own drawings to erase it'}
								aria-label="Erase"
								onclick={() => selectTool('eraser')}
							>
								<Eraser class="h-4 w-4" />
							</Button>
							<!-- The selected swatch is ringed rather than bordered: an
							     outline is drawn outside the element, so it never covers
							     the colour it is marking and can't sit between the swatch
							     and the pointer. Light blue reads against every colour in
							     the palette, black included, and against the toolbar. -->
							<div class="flex items-center gap-2 px-2">
								{#each STROKE_COLORS as opt (opt.value)}
									<button
										type="button"
										aria-label={opt.label}
										aria-pressed={strokeColor === opt.value}
										title={opt.label}
										class={[
											'h-6 w-6 rounded-full',
											strokeColor === opt.value && 'outline-2 outline-offset-2 outline-sky-400'
										]}
										style="background-color: {opt.value}"
										onclick={() => (strokeColor = opt.value)}
									></button>
								{/each}
							</div>
						</div>
						<div class="flex flex-wrap gap-2">
							<Button
								variant="outline"
								size="sm"
								disabled={!client.canUndo}
								title="Undo your last drawing, erase or token deletion (Ctrl+Z)"
								aria-label="Undo"
								onclick={() => client?.undo()}
							>
								<Undo class="h-4 w-4" />
							</Button>
							<Button
								variant="outline"
								size="sm"
								disabled={!client.canRedo}
								title="Redo (Ctrl+Shift+Z)"
								aria-label="Redo"
								onclick={() => client?.redo()}
							>
								<Redo class="h-4 w-4" />
							</Button>
							<Button
								variant="outline"
								size="sm"
								aria-label="Reset view"
								onclick={() => canvasRef?.resetView()}
								title="Reset view"
							>
								<RefreshCw class="h-4 w-4" />
							</Button>
						</div>
					</div>
					<!-- Template options appear only while a template tool is
					     active: they mean nothing otherwise, and the tool row is
					     long enough already. -->
					{#if isTemplateTool}
						<div class="flex flex-wrap items-center gap-3 rounded-md border p-2">
							<div class="flex items-center gap-2">
								<span class="text-xs text-muted-foreground">Snap to</span>
								{#each SNAP_MODES as mode (mode.value)}
									<Button
										variant={snapMode === mode.value ? 'default' : 'outline'}
										size="sm"
										aria-pressed={snapMode === mode.value}
										onclick={() => (snapMode = mode.value)}
									>
										{mode.label}
									</Button>
								{/each}
							</div>
							{#if activeTool === 'template-line'}
								<div class="flex items-center gap-2">
									<span class="text-xs text-muted-foreground">Line width</span>
									{#each LINE_WIDTH_CHOICES_FEET as feet (feet)}
										<Button
											variant={lineWidthFeet === feet ? 'default' : 'outline'}
											size="sm"
											aria-label="{feet} foot wide line"
											aria-pressed={lineWidthFeet === feet}
											onclick={() => (lineWidthFeet = feet)}
										>
											{feet} ft
										</Button>
									{/each}
								</div>
							{/if}
						</div>
					{/if}
					<GameCanvas
						room={client}
						{activeTool}
						{strokeColor}
						{snapMode}
						{lineWidthFeet}
						bind:selectedTokenId
						bind:this={canvasRef}
					/>
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
					{@render whoIsHere(client)}
					{@render tokenDetails(client, selectedToken)}
					{@render chatMessages(client, 'max-h-96')}
					{@render chatForm()}
				</Card.Content>
			</Card.Root>
		</div>
	</div>

	<div class="fixed inset-x-0 bottom-0 lg:hidden">
		{#if mobileChatOpen}
			<div class="flex max-h-[60vh] flex-col gap-3 border-t bg-background p-4 shadow-lg">
				{@render whoIsHere(client)}
				{@render tokenDetails(client, selectedToken)}
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
