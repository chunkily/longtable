<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { toast } from 'svelte-sonner';
	import {
		endSession,
		gmLogin,
		isNotFound,
		joinRoom,
		listSeats,
		type Seat,
		type Session
	} from '$lib/api';
	import { clearSession, loadSession, saveSession, touchSession } from '$lib/session';
	import { hostNotice } from '$lib/host-notice.svelte';
	import { loadFogOpacity, saveFogOpacity } from '$lib/fog-opacity';
	import { RoomClient, type Token } from '$lib/room.svelte';
	import { DEFAULT_LINE_WIDTH_FEET, type SnapMode } from '$lib/aoe';
	import { familyHasStrip, familyOf, type Tool } from '$lib/tool-family';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Badge } from '$lib/components/ui/badge';
	import { Separator } from '$lib/components/ui/separator';
	import * as Card from '$lib/components/ui/card';
	import GameCanvas from '$lib/components/game-canvas.svelte';
	import MapToolbar from '$lib/components/map-toolbar.svelte';
	import ToolStrip from '$lib/components/tool-strip.svelte';
	import RoomMenu from '$lib/components/room-menu.svelte';
	import ManageRoomDialog from '$lib/components/manage-room-dialog.svelte';
	import RoomCodeDialog from '$lib/components/room-code-dialog.svelte';
	import SceneManagerDialog from '$lib/components/scene-manager-dialog.svelte';
	import CreateTokenDialog from '$lib/components/create-token-dialog.svelte';
	import TokenDetailDialog from '$lib/components/token-detail-dialog.svelte';
	import TokenTrackerStrip from '$lib/components/token-tracker-strip.svelte';
	import InitiativePanel from '$lib/components/initiative-panel.svelte';
	import MessageSquare from '@lucide/svelte/icons/message-square';
	import Swords from '@lucide/svelte/icons/swords';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import ChevronUpIcon from '@lucide/svelte/icons/chevron-up';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';

	const slug = $derived(page.params.slug ?? '');

	let session = $state<Session | null>(null);
	let client = $state<RoomClient | null>(null);
	let canvasRef:
		{ resetView: () => void; viewCenterCell: () => { x: number; y: number } } | undefined =
		$state();

	// The pre-join screen asks one question at a time, and the first one
	// is which side of the screen you're on. A GM proves the room password
	// and a Player picks a chair, which share nothing but a name box — so
	// showing both at once meant every arrival read a form with two thirds
	// of it addressed to somebody else.
	//
	//   role ─┬─ gm
	//         └─ seats ── name   ("I'm new here")
	//
	// Every step but the first can go back, so a wrong turn costs a click
	// rather than a reload.
	type JoinStep = 'role' | 'gm' | 'seats' | 'name';
	let step = $state<JoinStep>('role');
	let displayName = $state('');
	let password = $state('');
	let joining = $state(false);

	// The room's seats, for a device with no stored session. Only the
	// player seats are offered: the GM's is a role boundary and goes
	// through the password on the GM step instead.
	let seats = $state<Seat[]>([]);
	let seatsRoomName = $state('');
	// Whether the seat list has come back yet — which the seats step has
	// to be able to tell apart from "this table has no seats". Offering
	// "I'm new here" on its own while the answer is still in flight is
	// exactly how someone ends up on a fresh seat beside the one holding
	// their tokens, which is the mistake seats exist to prevent.
	let seatsLoaded = $state(false);
	// Set only when the server has said there is no such room — never by a
	// failure that merely might mean that. See the catch in onMount.
	let roomMissing = $state(false);
	const playerSeats = $derived(seats.filter((s) => s.role !== 'gm'));

	let chatText = $state('');

	// The active map tool. 'none' is the hand — pan and token selection.
	// The toolbar groups these into five families for display; the
	// grouping is derived from this value rather than stored beside it,
	// so the two can't disagree. See $lib/tool-family.
	let activeTool = $state<Tool>('none');
	// Where an area template's points may land. A setting rather than a
	// rule because tables genuinely differ — some put a burst on a cell
	// centre, some on an intersection, some eyeball it — and it never
	// leaves this client: the points that go on the wire are already
	// snapped, so nobody else needs to know which convention made them.
	let snapMode = $state<SnapMode>('intersections');
	// A Line is the one shape a single drag can't describe: the drag
	// gives length and direction, never width.
	let lineWidthFeet = $state(DEFAULT_LINE_WIDTH_FEET);
	let strokeColor = $state('#000000');
	// The GM's own preference, same reasoning as the theme control: it's
	// how fog looks on this screen, not room state, so it's read from
	// localStorage up front and written back on every change rather than
	// carried on the wire.
	let fogOpacity = $state(loadFogOpacity());
	$effect(() => saveFogOpacity(fogOpacity));

	// Which of the two switchable panels the side rail is showing. The
	// menu is the third foot icon but isn't a panel — it opens over
	// whichever of these is on screen.
	let panel = $state<'chat' | 'initiative'>('chat');
	// Below lg the rail is a bottom sheet, shut by default so the map gets
	// the whole screen until something is wanted from it.
	let sheetOpen = $state(false);

	let scenesOpen = $state(false);
	let manageRoomOpen = $state(false);
	let roomCodeOpen = $state(false);

	// Which token this client is looking at. Local to this browser and
	// nothing more: it never goes on the wire, so two people can have
	// different tokens selected, and a reload starts with none. Owned here
	// rather than inside the canvas because the side panel reads it too —
	// the canvas binds it so a click can set it.
	let selectedTokenId = $state<string | null>(null);
	// Derived rather than kept in step by an effect. A selection whose
	// token has gone — the scene changed under it, or it was removed —
	// simply reads as nothing selected, with no second copy of the truth
	// to go stale. The id itself is left alone, so a token that comes back
	// under the same id comes back selected.
	const selectedToken = $derived<Token | null>(
		client?.tokens.find((t) => t.id === selectedTokenId) ?? null
	);

	/**
	 * The owner's display name for the details panel. Falls back to a
	 * neutral phrase rather than the raw id: the roster holds everyone who
	 * has ever joined, so a miss should be impossible, and an id on screen
	 * would be a worse answer than admitting we don't know.
	 */
	function ownerName(room: RoomClient, participantId: string): string {
		const owner = room.participants.find((p) => p.id === participantId);
		return owner ? `${owner.displayName}'s token` : 'Owned by someone no longer listed';
	}

	onMount(() => {
		const existing = loadSession(slug);
		if (existing) {
			// Sitting back down at this table puts it at the top of the home
			// page's list, which is what makes that list order by the game
			// someone is actually playing. A device that still has its
			// session never sees the seat picker at all.
			touchSession(slug);
			startSession(existing);
			return;
		}

		// No session: this is the pre-join screen, so find out what chairs
		// are at the table.
		listSeats(slug)
			.then((res) => {
				seatsRoomName = res.roomName;
				seats = res.seats;
			})
			.catch((err) => {
				// A 404 here is the one failure worth stopping for: there is no
				// room behind this code, and every question the join screen is
				// about to ask has no answer. Someone who mistyped a character
				// used to find that out four steps later, after picking a role,
				// saying they were new, and typing a name into a form that
				// could only ever fail.
				//
				// Anything else stays non-fatal, which is what this catch used
				// to do for every failure alike: a blip or a 500 says nothing
				// about whether the room exists, and telling someone their
				// room is gone when it isn't is the worse mistake of the two.
				if (isNotFound(err)) roomMissing = true;
			})
			.finally(() => (seatsLoaded = true));
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

	// Taking a seat someone already sat in. No name to type and no
	// password: the seat carries the name, and claiming is open by
	// design (ADR-0007) — what you get back is a session of your own on
	// a seat that already owns tokens.
	async function handleClaimSeat(seat: Seat) {
		joining = true;
		try {
			const s = await joinRoom(slug, { participantId: seat.participantId });
			saveSession(s);
			startSession(s);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to take that seat');
		} finally {
			joining = false;
		}
	}

	async function handleJoin(event: SubmitEvent) {
		event.preventDefault();
		joining = true;
		try {
			const s =
				step === 'gm'
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

	// Leaving spends a *session*, not an identity. Since seats, the seat
	// survives with the tokens it owns still attached, so coming back
	// means picking it off the list rather than starting over — and any
	// other device signed into the same seat stays signed in.
	//
	// It still doesn't remove anyone from the roster: that list is
	// everyone who has ever joined, and a seat is exactly the thing meant
	// to outlive a browser. A GM who wants a seat gone removes it from
	// Manage room.
	//
	// Disconnecting first so the others see the presence drop straight
	// away rather than when the socket eventually times out.
	async function handleLeave() {
		client?.disconnect();
		if (session) await endSession(slug, session.sessionToken);
		clearSession(slug);
		goto(resolve('/'));
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
	// place that says so plainly — and it stays a banner across the top of
	// the map rather than becoming the status dot in session info, because
	// it is the one thing a Room Member must not miss and a dot in a
	// corner is missable.
	const showConnectionBanner = $derived(
		!!client && client.status !== 'open' && client.status !== 'connecting'
	);
	const isGM = $derived(client?.you?.role === 'gm');
	// Who may open the edit dialog on the selected token: a GM on anything,
	// and a Player on one they own — where all they get is the trackers and
	// conditions. Mirrors the per-field check in handleTokenUpdate, which is
	// what actually enforces it.
	const canEditSelected = $derived(
		!!selectedToken &&
			(isGM || (!!client?.you && selectedToken.ownerParticipantId === client.you.participantId))
	);
	// Deletion follows ownership too, now that a Player can create tokens
	// — clearing away your own summons is the other half of conjuring
	// them. Same rule as editing, and the same rule handleTokenDelete
	// enforces; kept as its own name because the two were different for
	// most of this file's life and will read as a mistake otherwise.
	const canDeleteSelected = $derived(canEditSelected);
	const showStrip = $derived(familyHasStrip(familyOf(activeTool)));
</script>

<svelte:head><title>{session?.roomName ?? seatsRoomName ?? slug} — Longtable</title></svelte:head>

<svelte:window onkeydown={handleKeydown} />

{#if !session || !client}
	<!-- Declared out here rather than inside Card.Content: a snippet
	     written as a component's child becomes one of its props. -->
	{#snippet back(to: JoinStep)}
		<div>
			<Button
				type="button"
				variant="ghost"
				size="sm"
				class="-ml-2"
				disabled={joining}
				onclick={() => (step = to)}
			>
				<ChevronLeftIcon class="h-4 w-4" /> Back
			</Button>
		</div>
	{/snippet}

	<div class="mx-auto max-w-md p-6">
		{#if roomMissing}
			<!-- The server has said there is no room here, so the join screen
			     is replaced rather than annotated: every question it asks —
			     which side of the table you're on, which chair is yours — has
			     no answer, and leaving them on screen invites someone to keep
			     answering them. A mistyped character is by far the likeliest
			     way to arrive here, so the code is repeated back for checking
			     against whatever it was copied from. -->
			<Card.Root>
				<Card.Header>
					<Card.Title>Room not found!</Card.Title>
					<Card.Description>Room code {slug} does not exist.</Card.Description>
				</Card.Header>
				<Card.Content class="flex flex-col gap-4">
					<p class="text-sm text-muted-foreground">We couldn't find a room with that code.</p>
					<Button href={resolve('/')} class="self-start">Click here to go back home</Button>
				</Card.Content>
			</Card.Root>
		{:else}
			<Card.Root>
				<Card.Header>
					<Card.Title>{seatsRoomName || 'Join room'}</Card.Title>
					<!-- Named rather than left bare: this string is what someone
					     reads back to whoever sent it when they end up in the wrong
					     room, and "room code" is what it's called everywhere else. -->
					<Card.Description>Room code {slug}</Card.Description>
				</Card.Header>
				<Card.Content class="flex flex-col gap-4">
					{#if step === 'role'}
						<p class="text-sm text-muted-foreground">How are you joining this table?</p>
						<div class="flex gap-2">
							<Button type="button" class="flex-1" onclick={() => (step = 'seats')}>Player</Button>
							<!-- The GM seat is the one exception to open-claim: it's a
						     role boundary, so it goes through the room password
						     rather than being a chair anyone can sit in. -->
							<Button type="button" variant="outline" class="flex-1" onclick={() => (step = 'gm')}>
								I'm the GM
							</Button>
						</div>
					{:else if step === 'seats'}
						{@render back('role')}
						<!-- Seats come before the name box, because on a device that
					     doesn't remember you taking one is almost always what you
					     want: it brings back the tokens you own and your name,
					     where joining fresh would leave them behind on a chair
					     nobody is sitting in. Open-claim, no password — see
					     ADR-0008 and ADR-0007. -->
						{#if !seatsLoaded}
							<p class="text-sm text-muted-foreground">Looking for the seats at this table…</p>
						{:else}
							<div class="flex flex-col gap-2">
								<p class="text-sm font-medium">
									{playerSeats.length > 0
										? 'Take your seat'
										: 'Nobody has taken a seat at this table yet.'}
								</p>
								{#each playerSeats as seat (seat.participantId)}
									<!-- Named rather than left to its contents: "Bob" alone is
								     also what a token is called, and this is the one
								     control on the page whose whole job is to say whose
								     chair it is. -->
									<Button
										type="button"
										variant="outline"
										class="justify-between"
										aria-label="Take {seat.displayName}'s seat"
										disabled={joining}
										onclick={() => handleClaimSeat(seat)}
									>
										<span class="truncate">{seat.displayName}</span>
										{#if seat.connected}
											<!-- Someone is on it right now. Still claimable — two
										     devices on one seat is one person, which is the
										     whole point — but worth saying so nobody takes a
										     chair thinking it's spare. -->
											<Badge variant="secondary">here now</Badge>
										{/if}
									</Button>
								{/each}
								{#if playerSeats.length > 0}
									<Separator class="my-1" />
								{/if}
								<!-- The last slot in the same list rather than a link
							     underneath it, and dashed so it reads as the empty
							     chair it is: being new is one of the answers to
							     "which of these is you", not a way out of the
							     question. It's also the only slot on a table nobody
							     has sat down at yet. -->
								<Button
									type="button"
									variant="outline"
									class="justify-start border-dashed"
									disabled={joining}
									onclick={() => (step = 'name')}
								>
									I'm new here
								</Button>
							</div>
						{/if}
					{:else}
						{@render back(step === 'gm' ? 'role' : 'seats')}
						<form class="flex flex-col gap-4" onsubmit={handleJoin}>
							<div class="flex flex-col gap-2">
								<Label for="display-name">Your name</Label>
								<Input id="display-name" bind:value={displayName} required />
							</div>
							{#if step === 'gm'}
								<div class="flex flex-col gap-2">
									<Label for="gm-password">GM password</Label>
									<Input id="gm-password" type="password" bind:value={password} required />
								</div>
							{/if}
							<Button type="submit" disabled={joining}>{joining ? 'Joining…' : 'Join'}</Button>
						</form>
					{/if}
				</Card.Content>
			</Card.Root>
		{/if}
	</div>
{:else}
	{#snippet chatPanel(room: RoomClient)}
		<div class="flex min-h-0 flex-1 flex-col gap-2">
			<ul class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto">
				{#each room.messages as msg (msg.id)}
					{@const canDelete =
						!!room.you && (room.you.role === 'gm' || msg.participantId === room.you.participantId)}
					<!-- The server redacts per recipient, not this component: an
					     empty body on a deleted message means this client is a
					     bystander, and anything else means it's the author or the
					     one who deleted it, still allowed to see what they wrote
					     or removed — struck through rather than hidden outright. -->
					{@const isRedacted = msg.deleted && !msg.body && !msg.rollExpression}
					<li
						class={[
							'flex items-start gap-1 rounded-md px-2 py-1 text-sm',
							msg.kind === 'roll' && !isRedacted && 'bg-accent text-accent-foreground'
						]}
					>
						<div class="min-w-0 flex-1">
							{#if isRedacted}
								<span class="text-muted-foreground italic">This message has been deleted.</span>
							{:else}
								<span class={msg.deleted ? 'line-through opacity-60' : undefined}>
									<strong>{msg.participantName}:</strong>
									{#if msg.kind === 'roll'}
										{msg.body} → <strong>{msg.rollResult}</strong>
										<span class="text-xs text-muted-foreground">({msg.rollBreakdown})</span>
									{:else}
										{msg.body}
									{/if}
								</span>
							{/if}
						</div>
						<!-- chat.delete folds both stages into one command — the hub
						     decides from the message's current state whether this
						     click leaves a placeholder or purges it, so the button
						     never has to track which stage a message is on. -->
						{#if canDelete}
							<Button
								variant="ghost"
								size="sm"
								class="h-5 w-5 shrink-0 p-0"
								aria-label={msg.deleted ? 'Remove message permanently' : 'Delete message'}
								title={msg.deleted
									? 'Remove this message permanently'
									: 'Delete this message (click again to remove it permanently)'}
								onclick={() => room.deleteMessage(msg.id)}
							>
								<Trash2 class="h-3 w-3" />
							</Button>
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
		</div>
	{/snippet}

	<!-- Rendered twice — once in the rail, once in the mobile sheet — and
	     only one is ever on screen, so the duplicate ids inside it never
	     both reach the accessibility tree. Same arrangement as the chat
	     panel above. -->
	{#snippet initiativePanel(room: RoomClient)}
		<InitiativePanel {room} {isGM} bind:selectedTokenId />
	{/snippet}

	<!-- Room name, who you are, and whether the socket is up. There is no
	     page header any more, so this is the only place these live. -->
	{#snippet sessionInfo(room: RoomClient)}
		<section aria-label="Session info" class="flex flex-col gap-2 border-b p-3">
			<div class="flex flex-wrap items-center gap-2">
				<h1 class="min-w-0 flex-1 truncate text-sm font-semibold">{room.roomName || slug}</h1>
				<Badge variant={statusVariant}>{room.status}</Badge>
			</div>
			<p class="text-xs text-muted-foreground">
				playing as <strong>{room.you?.displayName}</strong>
				<Badge variant="outline" class="ml-1">{room.you?.role}</Badge>
			</p>
			<!-- The room code used to sit here. It lives at the top of the
			     room menu now, where it shows itself and opens a dialog
			     holding both the code and this browser's address. The rail
			     is the thing on screen the whole session, so what earns a
			     place in it is what changes — who is connected, whether the
			     socket is up — not a constant six characters. -->
			<!-- Who is actually at the table right now, which is a different
			     list from everyone who has ever joined: someone who played last
			     week and isn't online doesn't appear.

			     A <section>, not a <div>: the accessible name only gives it a
			     landmark role on a sectioning element, and that role is how
			     the presence specs find it. -->
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
		</section>
	{/snippet}

	{#snippet tokenDetails(room: RoomClient, token: Token)}
		<div class="flex items-start gap-2">
			<div class="min-w-0 flex-1">
				<!-- The size used to be spelled out under the name and isn't any
				     more: a token's footprint is drawn on the map at the size it
				     is, so the line said in words what the reader was already
				     looking at. Being hidden is the opposite — it is precisely
				     what the map can't show a GM at a glance — so that half
				     stayed, as a badge. -->
				<div class="flex items-center gap-1.5">
					<p class="truncate text-sm font-medium">{token.name}</p>
					{#if token.visibility === 'hidden'}
						<Badge variant="outline" class="shrink-0">hidden from players</Badge>
					{/if}
				</div>
				<!-- Shown to everyone, not just the GM: whose token is whose is
				     the point of an owner, and it's the roster that turns the
				     stored id back into a name. Silent when nobody owns it —
				     most tokens are monsters, and "Owner: nobody" on every one
				     of them is noise. -->
				{#if token.ownerParticipantId}
					<p class="truncate text-xs text-muted-foreground">
						{ownerName(room, token.ownerParticipantId)}
					</p>
				{/if}
				<!-- Values editable in place for whoever may edit the token at
				     all — the same rule the Edit button follows. Labels and
				     conditions stay in the dialog; damage is what changes
				     every round and what shouldn't cost a dialog to change. -->
				<TokenTrackerStrip {room} {token} editable={canEditSelected} />
			</div>
			{#if canEditSelected && session}
				<TokenDetailDialog
					{room}
					{token}
					roomSlug={session.roomSlug}
					sessionToken={session.sessionToken}
					canEditAll={isGM}
				/>
			{/if}
			<!-- A GM on anything, and anyone else on a token they own — the
			     same rule as the Edit button beside it. Deletion used to be
			     GM-only because creation was; now that a Player can conjure
			     eight monkeys, leaving the clearing-up to the GM would be the
			     busywork this was meant to remove. token.delete enforces it.

			     Not behind a confirmation, unlike deleting a scene: the
			     deletion is undoable, which is the cheaper answer to a
			     misclick than a dialog on every deliberate one. -->
			{#if canDeleteSelected}
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
		</div>
	{/snippet}

	<!-- The three icons at the foot of the panel. Rendered in the rail on a
	     desktop and in the pinned bottom bar on a phone; only one of the
	     two is ever displayed, so the duplicate accessible names never
	     both reach the accessibility tree. -->
	{#snippet panelIcons(room: RoomClient)}
		<div class="flex items-center justify-around border-t p-1">
			<Button
				variant={panel === 'chat' ? 'default' : 'ghost'}
				size="sm"
				aria-label="Chat"
				aria-pressed={panel === 'chat'}
				title="Chat"
				onclick={() => {
					panel = 'chat';
					sheetOpen = true;
				}}
			>
				<MessageSquare class="h-4 w-4" />
			</Button>
			<Button
				variant={panel === 'initiative' ? 'default' : 'ghost'}
				size="sm"
				aria-label="Initiative"
				aria-pressed={panel === 'initiative'}
				title="Initiative tracker"
				onclick={() => {
					panel = 'initiative';
					sheetOpen = true;
				}}
			>
				<Swords class="h-4 w-4" />
			</Button>
			<RoomMenu
				{room}
				{slug}
				{isGM}
				onOpenScenes={() => (scenesOpen = true)}
				onOpenManageRoom={() => (manageRoomOpen = true)}
				onOpenRoomCode={() => (roomCodeOpen = true)}
				onLeave={handleLeave}
				onResetView={() => canvasRef?.resetView()}
			/>
		</div>
	{/snippet}

	<!-- The dialogs are opened from the menu rather than from a trigger of
	     their own, so they live out here at the top level rather than
	     inside whichever copy of the icon bar is on screen. -->

	<!-- Outside the isGM block below: everyone at the table can hand the
	     room to someone else, and only a GM can manage it. -->
	<RoomCodeDialog {slug} bind:open={roomCodeOpen} />

	{#if isGM && session}
		<SceneManagerDialog
			room={client}
			roomSlug={session.roomSlug}
			sessionToken={session.sessionToken}
			bind:open={scenesOpen}
			trigger={false}
		/>
		<ManageRoomDialog
			room={client}
			roomSlug={session.roomSlug}
			sessionToken={session.sessionToken}
			bind:open={manageRoomOpen}
		/>
	{/if}

	<!-- Starts under the Host's banner when there is one. This container
	     is `fixed`, so the layout's padding doesn't reach it — the map
	     would otherwise run under the banner and take the toolbar with
	     it. Read from the banner's own height rather than a constant,
	     since a long message wraps. -->
	<div
		class="fixed inset-x-0 bottom-0 flex flex-col lg:flex-row"
		style="top: {hostNotice.height}px"
	>
		<!-- The map. Everything else either floats over it or sits in the
		     rail beside it; there is no padding, no card and no header. -->
		<div class="relative min-h-0 min-w-0 flex-1">
			{#if client.scene}
				<GameCanvas
					room={client}
					{activeTool}
					{strokeColor}
					{snapMode}
					{lineWidthFeet}
					{fogOpacity}
					bind:selectedTokenId
					bind:this={canvasRef}
				/>
			{:else}
				<div class="flex h-full items-center justify-center bg-muted p-6">
					<p class="text-sm text-muted-foreground">
						{isGM
							? 'Create a scene to get started — Scenes, in the menu.'
							: 'Waiting for the GM to start a scene…'}
					</p>
				</div>
			{/if}

			{#if showConnectionBanner}
				<!-- An opaque surface, not a tint. `bg-destructive/10` reads as a
				     pale red wash on the white page this was designed against
				     and as very nearly nothing over a dark battle map — which
				     is precisely where it matters, since this is the one thing
				     a Room Member must not miss. The toolbar floating beside it
				     already solved this: carry your own background. -->
				<div
					role="status"
					class="absolute inset-x-0 top-0 z-30 flex flex-wrap items-center gap-3 border-b-2 border-destructive bg-background p-3 text-sm font-medium text-destructive shadow-md"
				>
					{#if client.sessionExpired}
						<span>
							This session is no longer valid — the room may have been reset. Rejoin to carry on.
						</span>
						<Button variant="outline" size="sm" onclick={() => location.reload()}>Rejoin</Button>
					{:else if client.reconnectExhausted}
						<span>Can't reach the table. Nothing you do will reach the others until it's back.</span
						>
						<Button variant="outline" size="sm" onclick={() => client?.reconnect()}
							>Try again</Button
						>
					{:else}
						<span>Reconnecting to the table…</span>
					{/if}
				</div>
			{/if}

			{#if client.scene}
				<!-- Both captured here because the snippet below reads them
				     inside a closure, where the {#if}'s narrowing doesn't
				     reach. -->
				{@const sceneId = client.scene.id}
				{@const room = client}
				{@const joined = session}
				<div
					class={[
						'absolute left-2 z-20 flex flex-col items-start gap-2',
						showConnectionBanner ? 'top-16' : 'top-2'
					]}
				>
					<MapToolbar {room} bind:activeTool {isGM} onResetView={() => canvasRef?.resetView()}>
						{#snippet newToken()}
							<!-- Everyone's, not just the GM's: a Player's summons and
							     familiars were the GM's paperwork mid-fight. The dialog
							     itself is what differs by role. -->
							{#if joined}
								<CreateTokenDialog
									{room}
									{sceneId}
									{isGM}
									roomSlug={joined.roomSlug}
									sessionToken={joined.sessionToken}
									spawnCell={() => canvasRef?.viewCenterCell() ?? { x: 0, y: 0 }}
								/>
							{/if}
						{/snippet}
					</MapToolbar>
					<!-- Floats under the toolbar on a desktop; below lg the same
					     strip is rendered inside the sheet instead. -->
					<div class="hidden lg:block">
						<ToolStrip
							room={client}
							{sceneId}
							bind:activeTool
							bind:strokeColor
							bind:snapMode
							bind:lineWidthFeet
							bind:fogOpacity
							{isGM}
						/>
					</div>
				</div>
			{/if}
		</div>

		<!-- The rail. Static: the panel switcher lives inside it, so a
		     collapsible one would need something left behind on the map to
		     bring it back, and the map is still most of the screen without
		     that complication. -->
		<aside class="hidden w-[368px] shrink-0 flex-col border-l bg-background lg:flex">
			<!-- Holds its height with nothing selected, so the rest of the rail
			     doesn't jump every time you click empty map. -->
			<!-- Empty, it's a plain muted block rather than a sentence. The
			     instruction was read once and then sat there for the rest of
			     the session saying nothing, and the shaded panel already says
			     "nothing here" without asking to be read. It still holds its
			     height, which is the point of the section existing at all.

			     A *floor* and a ceiling rather than one fixed height, and the
			     content laid out from the top. It was `h-28 items-center`,
			     which meant a token carrying condition tags overflowed a box
			     that centred it — and overflow on a centred flex child spills
			     off the *top*, taking the token's name with it, out of the
			     panel and out of reach. Growing to `max-h-44` fits the usual
			     case; anything past that scrolls, with the name staying put. -->
			<section
				aria-label="Selected token"
				class={[
					'flex max-h-44 min-h-28 shrink-0 flex-col overflow-y-auto border-b p-3',
					!selectedToken && 'bg-muted'
				]}
			>
				{#if selectedToken}
					<div class="w-full">{@render tokenDetails(client, selectedToken)}</div>
				{/if}
			</section>

			{@render sessionInfo(client)}

			<!-- Both panels stay mounted and are hidden with CSS rather than
			     swapped out: switching to the tracker and back must not lose a
			     half-typed message or the scroll position in the log. -->
			<div class="flex min-h-0 flex-1 flex-col p-3">
				<div class={['flex min-h-0 flex-1 flex-col', panel !== 'chat' && 'hidden']}>
					{@render chatPanel(client)}
				</div>
				<div class={['flex min-h-0 flex-1 flex-col', panel !== 'initiative' && 'hidden']}>
					{@render initiativePanel(client)}
				</div>
			</div>

			{@render panelIcons(client)}
		</aside>
	</div>

	<!-- Below lg the rail becomes a sheet coming up from the bottom, with
	     the icons pinned along the bottom edge whether it's open or shut.
	     A right-hand drawer sliding over the map was the alternative and
	     was rejected: it buys one sidebar implementation by covering the
	     thing you're looking at. -->
	<div class="fixed inset-x-0 bottom-0 z-30 lg:hidden">
		{#if sheetOpen}
			<div class="flex max-h-[60vh] flex-col border-t bg-background shadow-lg">
				<div class="flex items-center justify-end p-1">
					<Button
						variant="ghost"
						size="sm"
						aria-label="Close panel"
						title="Give the screen back to the map"
						onclick={() => (sheetOpen = false)}
					>
						<ChevronUpIcon class="h-4 w-4 rotate-180" />
					</Button>
				</div>
				{@render sessionInfo(client)}
				<div class="flex min-h-0 flex-1 flex-col p-3">
					<div class={['flex min-h-0 flex-1 flex-col', panel !== 'chat' && 'hidden']}>
						{@render chatPanel(client)}
					</div>
					<div class={['flex min-h-0 flex-1 flex-col', panel !== 'initiative' && 'hidden']}>
						{@render initiativePanel(client)}
					</div>
				</div>
			</div>
		{/if}

		<!-- The contextual strip docks here rather than floating over the
		     map: draw's is borderline at 375px and measure's doesn't fit,
		     and a horizontally scrolling bar over a pannable canvas is a
		     gesture conflict. Flagged in the design as the decision most
		     likely to be revisited once a real table has used it. -->
		{#if client.scene && showStrip}
			{@const sceneId = client.scene.id}
			<div class="overflow-x-auto border-t bg-background p-1">
				<ToolStrip
					room={client}
					{sceneId}
					bind:activeTool
					bind:strokeColor
					bind:snapMode
					bind:lineWidthFeet
					bind:fogOpacity
					{isGM}
				/>
			</div>
		{/if}

		<!-- The selected token becomes a bar pinned above the icons, shown
		     only when something is selected — it leaves the sheet entirely,
		     and unlike the rail it holds no space when nothing is selected.
		     Anchoring it to the token itself was the first idea and was
		     rejected: it tracks a target you're dragging under your own
		     thumb, covers the squares around it, and needs world→screen
		     conversion every frame because the card is DOM and the token is
		     Konva. The selection ring already says *which* token; this only
		     has to say *what*. -->
		{#if selectedToken}
			<section aria-label="Selected token" class="border-t bg-background p-2">
				{@render tokenDetails(client, selectedToken)}
			</section>
		{/if}

		<div class="bg-background">
			{@render panelIcons(client)}
		</div>
	</div>
{/if}
