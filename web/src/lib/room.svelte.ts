// Reactive WebSocket client for a single room. Wraps the raw
// command/event protocol (see internal/ws/hub.go) in Svelte 5 runes
// state so components just read fields and let the UI update itself.

import { checkSession } from './api';
import type { TemplateKind } from './aoe';
import { MEASURE_SEND_INTERVAL_MS } from './measure';
import { PING_COOLDOWN_MS, PING_LIFETIME_MS } from './ping';

export interface ChatMessage {
	id: string;
	participantId: string | null;
	participantName: string;
	kind: 'text' | 'roll';
	body: string;
	rollExpression: string | null;
	rollResult: number | null;
	rollBreakdown: string | null;
	createdAt: string;
}

export interface Token {
	id: string;
	sceneId: string;
	name: string;
	imageAssetId: string | null;
	x: number;
	y: number;
	width: number;
	height: number;
	ownerParticipantId: string | null;
	visibility: 'visible' | 'hidden';
}

export interface Scene {
	id: string;
	roomId: string;
	name: string;
	mapAssetId: string | null;
	gridSize: number;
	gridOffsetX: number;
	gridOffsetY: number;
	width: number;
	height: number;
}

export interface FogCell {
	x: number;
	y: number;
}

export type DrawingKind = 'freehand' | 'line' | 'rect' | 'ellipse';

export interface DrawingPoint {
	x: number;
	y: number;
}

export interface Drawing {
	id: string;
	sceneId: string;
	kind: DrawingKind;
	// One point per vertex for freehand; for the other kinds exactly
	// two — a line's start and end, or two opposite corners of the box a
	// rect or ellipse is drawn in.
	points: DrawingPoint[];
	color: string;
	// Who drew it — null for drawings made before authorship was
	// tracked, or whose author has left the room. Compare against
	// `you.participantId` to tell your own drawings from other people's.
	createdByParticipantId: string | null;
}

// A transient pointer-ping. id is generated client-side on arrival
// (the server doesn't assign one, since pings are fire-and-forget) so
// the canvas can track and remove each marker independently.
export interface Ping {
	id: string;
	sceneId: string;
	x: number;
	y: number;
	participantName: string;
}

// What someone is dragging out right now: a plain distance line, or one
// of the four area templates. Two world-space points either way, with
// the shape left to the reader (see $lib/measure and $lib/aoe). Never
// persisted and never in state.sync: it exists only for as long as the
// drag does. Keyed by participantId, since each person has one at a
// time and every update replaces their last.
export type MeasurementKind = 'distance' | TemplateKind;

export interface Measurement {
	participantId: string;
	participantName: string;
	sceneId: string;
	kind: MeasurementKind;
	from: DrawingPoint;
	to: DrawingPoint;
	/** Only a Line has one; a drag can't express a width on its own. */
	widthFeet?: number;
}

// Someone who has joined the room at some point. The roster is everyone
// who ever has; whether they're *connected* is a separate list, because
// it has no row behind it — it lives in the server's memory only. Never
// carries a session token: that's a credential, and this is broadcast to
// the whole room.
export interface Participant {
	id: string;
	displayName: string;
	role: 'gm' | 'player';
}

export interface You {
	participantId: string;
	displayName: string;
	role: 'gm' | 'player';
}

type ConnectionStatus = 'connecting' | 'open' | 'closed' | 'reconnecting';

// Retry pacing. Doubling from half a second to a fifteen-second ceiling,
// giving up after eight tries — by then roughly a minute has passed, and
// a banner offering a manual retry is more honest than a spinner that
// never stops. The jitter spreads a table's worth of clients out so a
// restarted server isn't hit by all of them at once.
const RECONNECT_BASE_MS = 500;
const RECONNECT_MAX_MS = 15_000;
const RECONNECT_MAX_ATTEMPTS = 8;
const RECONNECT_JITTER = 0.25;

interface Envelope {
	type: string;
	payload: unknown;
}

interface StateSyncPayload {
	room: { slug: string; name: string };
	you: You;
	messages?: ChatMessage[];
	scenes?: Scene[];
	scene?: Scene | null;
	tokens?: Token[];
	fogCells?: FogCell[];
	drawings?: Drawing[];
	participants?: Participant[];
	connectedParticipantIds?: string[];
}

interface TokenMovedPayload {
	tokenId: string;
	x: number;
	y: number;
}

interface FogCellsPayload {
	sceneId: string;
	cells: FogCell[];
}

interface FogResetPayload {
	sceneId: string;
}

interface SceneActivatedPayload {
	scene: Scene;
	tokens?: Token[];
	fogCells?: FogCell[];
	drawings?: Drawing[];
}

// scene.created and scene.updated both carry just the scene, deliberately
// without the tokens/fog/drawings that scene.activated brings — see the
// note on the scene.updated case below.
interface ScenePayload {
	scene: Scene;
}

interface SceneDeletedPayload {
	sceneId: string;
}

interface DrawingDeletedPayload {
	drawingId: string;
}

interface TokenDeletedPayload {
	tokenId: string;
}

interface ParticipantDisconnectedPayload {
	participantId: string;
}

interface PingPayload {
	sceneId: string;
	x: number;
	y: number;
	participantName: string;
}

interface MeasureEndedPayload {
	participantId: string;
}

// One reversible thing this session did to the map. Every variant
// carries the whole object rather than just its id, because undoing a
// removal has to put it back exactly as it was — same id, same place,
// same properties — and redoing has to recreate it the same way.
//
// Deleting a token rides the same stack as drawing and erasing rather
// than getting a mechanism of its own: undoing a delete is a
// `token.create` under the same id, exactly as undoing an erase is a
// `draw.create` under the same id.
//
// `moveToken` is the one variant that holds less than the whole object,
// because a move is the one action where nothing was removed — the token
// is still there, so only the square it came from has been lost. It
// carries where it went as well as where it was, and both directions are
// used: undo needs the destination to check the token is still where
// this session put it (see sendMoveToken).
//
// A caveat that comes with restoring: the server takes a drawing's
// author from the connection that sent it and ignores anything claimed
// in the payload, so a GM who erases a Player's stroke and then undoes
// gets the stroke back under their own name. It looks identical, but
// the Player can no longer erase it themselves. Tokens are unaffected —
// they have no author, and their owner travels in the payload.
type HistoryAction =
	| { kind: 'create'; drawing: Drawing }
	| { kind: 'erase'; drawing: Drawing }
	| { kind: 'deleteToken'; token: Token }
	| { kind: 'moveToken'; tokenId: string; from: TokenPosition; to: TokenPosition };

interface TokenPosition {
	x: number;
	y: number;
}

interface ErrorPayload {
	message: string;
	// Present when the failure is attributable to one drawing, which is
	// how an optimistically-rendered stroke knows to take itself back.
	drawingId?: string;
}

export class RoomClient {
	status = $state<ConnectionStatus>('connecting');
	error = $state('');
	/**
	 * Set when the server says the session itself is the problem, not the
	 * connection. Retrying can't fix it — the only way out is rejoining.
	 */
	sessionExpired = $state(false);

	roomName = $state('');
	you = $state<You | null>(null);

	// Two lists rather than an "online" flag per row, because they come
	// from different places: the roster is a table, and who is connected
	// is the server's memory. Folding them together would make the
	// offline half unrepresentable — and the offline half is exactly who
	// a GM assigns tokens to while prepping before a session.
	participants = $state<Participant[]>([]);
	connectedParticipantIds = $state<string[]>([]);

	messages = $state<ChatMessage[]>([]);
	// Every scene in the room, for the scene picker. `scene` below is the
	// one actually on screen — the two are kept in step, but only `scene`
	// carries the tokens/fog/drawings that go with it.
	scenes = $state<Scene[]>([]);
	scene = $state<Scene | null>(null);
	tokens = $state<Token[]>([]);
	fogCells = $state<FogCell[]>([]);
	drawings = $state<Drawing[]>([]);
	pings = $state<Ping[]>([]);
	measurements = $state<Measurement[]>([]);

	private socket: WebSocket | null = null;

	// Drawings taken off the map before the server confirmed the erase,
	// kept with the position they held so a refusal can put them back
	// where they were rather than on top of everything. Bookkeeping only
	// — nothing reactive reads it, so a plain Map is right here.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	private pendingErases = new Map<string, { drawing: Drawing; index: number }>();

	// Where this session's own moves are headed, for tokens whose
	// broadcast hasn't come back yet. State can't answer that: nothing
	// about a move is applied optimistically, so a token dragged a moment
	// ago is still recorded on the square it left. Undo needs the real
	// answer or a Ctrl+Z pressed inside the round trip would decide the
	// token never moved, skip the entry and undo whatever came before it.
	// Bookkeeping only — nothing reactive reads it, so a plain Map is
	// right here.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	private pendingMoves = new Map<string, TokenPosition>();

	private lastPingSentAt = 0;

	// What a reconnect needs to re-open the same socket, and where the
	// backoff has got to. attempt is 0 whenever the connection is healthy.
	private slug = '';
	private sessionToken = '';
	private attempt = $state(0);
	private reconnectTimer = $state<ReturnType<typeof setTimeout> | null>(null);

	// The latest measurement position not yet put on the wire, and the
	// timer holding the wire until the interval is up. Both null when no
	// measurement is in progress.
	private unsentMeasure: Measurement | null = null;
	private measureTimer: ReturnType<typeof setTimeout> | null = null;

	// This session's own reversible actions, oldest first. Reactive so the
	// toolbar can tell whether there is anything left to undo or redo.
	private undoable = $state<HistoryAction[]>([]);
	private redoable = $state<HistoryAction[]>([]);

	connect(slug: string, sessionToken: string) {
		this.disconnect();
		// Remembered so a retry can re-open the socket rather than re-join:
		// the participant already exists, and joining again would make a
		// second one with a new identity and a new name in the roster.
		this.slug = slug;
		this.sessionToken = sessionToken;
		this.attempt = 0;
		this.sessionExpired = false;
		this.openSocket();
	}

	private openSocket() {
		this.status = this.attempt === 0 ? 'connecting' : 'reconnecting';
		this.error = '';

		const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${proto}//${window.location.host}/ws?room=${encodeURIComponent(this.slug)}&token=${encodeURIComponent(this.sessionToken)}`;

		const socket = new WebSocket(url);
		this.socket = socket;

		socket.onopen = () => {
			this.status = 'open';
			// Reset here rather than on the first envelope: the backoff is
			// about reaching the server, and we have.
			this.attempt = 0;
		};
		socket.onclose = () => {
			// A close we asked for is not a failure to recover from.
			if (this.socket !== socket) return;
			this.socket = null;
			this.status = 'closed';
			void this.scheduleReconnect();
		};
		socket.onerror = () => {
			this.error = 'connection error';
		};
		socket.onmessage = (event) => {
			this.handleEnvelope(JSON.parse(event.data));
		};
	}

	/**
	 * Backoff between attempts: doubling from RECONNECT_BASE_MS, capped,
	 * and jittered so a `longtable` restart doesn't bring every client at
	 * the table back in the same millisecond.
	 */
	private retryDelay(): number {
		const capped = Math.min(RECONNECT_BASE_MS * 2 ** this.attempt, RECONNECT_MAX_MS);
		return capped * (1 - RECONNECT_JITTER + Math.random() * RECONNECT_JITTER * 2);
	}

	private async scheduleReconnect() {
		if (this.attempt >= RECONNECT_MAX_ATTEMPTS) return;

		// Ask REST whether the problem is us before spending another
		// attempt. A refused upgrade arrives as a bare onclose with no
		// status, so the socket alone can't tell a restarting server from
		// a session the server no longer knows — and retrying a dead
		// session forever is the worse of the two failures, because
		// nothing on screen ever explains it.
		const session = await checkSession(this.slug, this.sessionToken);
		if (session === 'invalid') {
			this.sessionExpired = true;
			this.error = 'your session is no longer valid — rejoin the room';
			return;
		}

		// Delay from the attempt just made, then count this one — so the
		// first retry waits the base delay rather than already doubling it.
		const delay = this.retryDelay();
		this.attempt += 1;
		this.status = 'reconnecting';
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			this.openSocket();
		}, delay);
	}

	/** Starts the retry sequence again from the top, for a manual button. */
	reconnect() {
		if (this.socket) return;
		this.attempt = 0;
		this.sessionExpired = false;
		this.openSocket();
	}

	/** True once retrying has stopped and only a manual attempt is left. */
	get reconnectExhausted(): boolean {
		return (
			this.status === 'closed' &&
			this.reconnectTimer === null &&
			(this.attempt >= RECONNECT_MAX_ATTEMPTS || this.sessionExpired)
		);
	}

	disconnect() {
		this.cancelPendingMeasure();
		if (this.reconnectTimer !== null) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		const socket = this.socket;
		// Cleared first so onclose can tell a close we asked for from one
		// that happened to us, and doesn't start reconnecting to a room
		// the page is leaving.
		this.socket = null;
		socket?.close();
	}

	// Returns whether the command actually went out — anything drawn
	// optimistically has to be able to tell, or a stroke made while the
	// connection is down would sit on the map forever with no server
	// round trip coming to confirm or reject it.
	private send(type: string, payload: unknown): boolean {
		if (this.socket?.readyState !== WebSocket.OPEN) return false;
		this.socket.send(JSON.stringify({ type, payload }));
		return true;
	}

	sendChat(text: string) {
		this.send('chat.send', { text });
	}

	// The square the token came from is worked out here rather than passed
	// in, so the canvas doesn't have to carry history's requirements
	// through a drag gesture. Everything the undo entry needs is already
	// known: `settledPosition` is where the room will see the token once
	// what this session has sent has landed.
	moveToken(tokenId: string, x: number, y: number) {
		const from = this.settledPosition(tokenId);
		if (!this.send('token.move', { tokenId, x, y })) return;
		this.pendingMoves.set(tokenId, { x, y });
		// A drop back onto the square it started from moved nothing, so
		// there is nothing to undo — and an entry with from === to would
		// swallow a whole press of Ctrl+Z doing nothing visible. The
		// command still goes out: the hub answers every move, and a refusal
		// is how the canvas learns a drag it already drew was rejected.
		if (!from || (from.x === x && from.y === y)) return;
		this.record({ kind: 'moveToken', tokenId, from, to: { x, y } });
	}

	// Where the token stands once every move this session has sent has
	// been answered — the pending destination if one is in flight, and
	// otherwise what state holds. Null when the token isn't in the scene
	// at all, which is the one case an in-flight move can't paper over.
	private settledPosition(tokenId: string): TokenPosition | null {
		const token = this.tokens.find((t) => t.id === tokenId);
		if (!token) return null;
		return this.pendingMoves.get(tokenId) ?? { x: token.x, y: token.y };
	}

	createScene(
		name: string,
		mapAssetId: string | null,
		gridSize: number,
		width: number,
		height: number
	) {
		this.send('scene.create', { name, mapAssetId, gridSize, width, height });
	}

	setActiveScene(sceneId: string) {
		this.send('scene.setActive', { sceneId });
	}

	deleteScene(sceneId: string) {
		this.send('scene.delete', { sceneId });
	}

	// Bounds travel with the image the way they do at creation: they
	// describe the map, and keeping the old ones would stretch the new
	// art to the shape of the art it replaced.
	setSceneMap(sceneId: string, mapAssetId: string | null, width: number, height: number) {
		this.send('scene.setMap', { sceneId, mapAssetId, width, height });
	}

	// Size and owner are optional here in the same way they're optional on
	// the wire: the server reads a missing width as one square and a
	// missing owner as nobody, which is what most tokens are.
	createToken(
		sceneId: string,
		name: string,
		imageAssetId: string | null,
		x: number,
		y: number,
		visibility: 'visible' | 'hidden' = 'visible',
		options: { squares?: number; ownerParticipantId?: string | null } = {}
	) {
		this.send('token.create', {
			sceneId,
			name,
			imageAssetId,
			x,
			y,
			visibility,
			width: options.squares ?? 1,
			height: options.squares ?? 1,
			ownerParticipantId: options.ownerParticipantId ?? null
		});
	}

	revealFog(sceneId: string, cells: FogCell[]) {
		this.send('fog.reveal', { sceneId, cells });
	}

	hideFog(sceneId: string, cells: FogCell[]) {
		this.send('fog.hide', { sceneId, cells });
	}

	// The bulk pair. Neither renders optimistically: they're one click
	// rather than a drag with a preview to keep in sync, so waiting for
	// the round trip costs nothing and keeps the whole room's fog coming
	// from one place.
	revealAllFog(sceneId: string) {
		this.send('fog.revealAll', { sceneId });
	}

	resetFog(sceneId: string) {
		this.send('fog.reset', { sceneId });
	}

	// Drawn locally straight away rather than after the round trip: at
	// the end of a stroke the preview shape is thrown away, so waiting
	// for the server would blink the line off the map and back on. The
	// id is minted here so the echo can be recognised as this same
	// stroke — and so it can be erased in the meantime, since the server
	// handles one connection's commands in the order they were sent.
	private sendCreate(drawing: Drawing): boolean {
		if (this.drawings.some((d) => d.id === drawing.id)) return false;
		if (
			!this.send('draw.create', {
				drawingId: drawing.id,
				sceneId: drawing.sceneId,
				kind: drawing.kind,
				points: drawing.points,
				color: drawing.color
			})
		) {
			return false;
		}

		this.drawings = [...this.drawings, drawing];
		return true;
	}

	// Also optimistic: the eraser highlights what a click will remove,
	// so leaving the stroke sitting there until the server agrees reads
	// as a missed click. A refusal (erasing what isn't yours, or what
	// someone else just erased) comes back naming the drawing, and puts
	// it back where it was. Returns what was taken off the map, so an
	// undoable action can hold on to it.
	private sendErase(drawingId: string): Drawing | null {
		const index = this.drawings.findIndex((d) => d.id === drawingId);
		if (index === -1) return null;

		const drawing = this.drawings[index];
		if (!this.send('draw.delete', { drawingId })) return null;

		this.pendingErases.set(drawingId, { drawing, index });
		this.drawings = this.drawings.filter((d) => d.id !== drawingId);
		return drawing;
	}

	createDrawing(sceneId: string, kind: DrawingKind, points: DrawingPoint[], color: string) {
		const drawing: Drawing = {
			id: crypto.randomUUID(),
			sceneId,
			kind,
			points,
			color,
			// Claimed locally so the eraser treats it as yours right
			// away; the server sets the real author on the echo, from
			// the connection rather than from anything sent here.
			createdByParticipantId: this.you?.participantId ?? null
		};
		if (this.sendCreate(drawing)) this.record({ kind: 'create', drawing });
	}

	deleteDrawing(drawingId: string) {
		const drawing = this.sendErase(drawingId);
		if (drawing) this.record({ kind: 'erase', drawing });
	}

	// Deliberately *not* optimistic, unlike the eraser. A token is deleted
	// from a button rather than a gesture, so there is no preview shape
	// that would blink off and back on while the round trip happens —
	// which is the whole reason drawings render ahead of the server.
	// token.move already waits for its broadcast the same way. Returns
	// what was on the map so an undo entry can hold the whole token.
	private sendDeleteToken(tokenId: string): Token | null {
		const token = this.tokens.find((t) => t.id === tokenId);
		if (!token) return null;
		if (!this.send('token.delete', { tokenId })) return null;
		return token;
	}

	// Puts a deleted token back under its own id, so it returns as the
	// same token to everyone who still has it rather than as a new one
	// that merely looks the same. Every property goes back on the wire:
	// the server rebuilds the row from this payload alone.
	private sendCreateToken(token: Token): boolean {
		if (this.tokens.some((t) => t.id === token.id)) return false;
		return this.send('token.create', {
			tokenId: token.id,
			sceneId: token.sceneId,
			name: token.name,
			imageAssetId: token.imageAssetId,
			x: token.x,
			y: token.y,
			width: token.width,
			height: token.height,
			ownerParticipantId: token.ownerParticipantId,
			visibility: token.visibility
		});
	}

	// Every editable field goes every time, not only the changed ones: the
	// wire can't tell "left alone" from "cleared", and clearing a token's
	// art — or taking it back off a Player — is a real edit. Position
	// isn't here: that's moveToken's, so an edit dialog opened before a
	// drag can't undo the drag on submit.
	updateToken(
		tokenId: string,
		fields: {
			name: string;
			imageAssetId: string | null;
			width: number;
			height: number;
			ownerParticipantId: string | null;
			visibility: 'visible' | 'hidden';
		}
	) {
		this.send('token.update', { tokenId, ...fields });
	}

	deleteToken(tokenId: string) {
		const token = this.sendDeleteToken(tokenId);
		if (token) this.record({ kind: 'deleteToken', token });
	}

	// Moves a token only if it is still standing where this session's own
	// move left it. That check is what keeps undo to your own moves: the
	// history can't tell who dragged a token last, but the position can —
	// if someone else has moved it since, it is no longer sitting at
	// `expected`, and dragging it back to a square from this session's
	// past would be undoing their move, not ours. Skipping instead is the
	// same treatment a stroke someone else erased already gets.
	private sendMoveToken(tokenId: string, expected: TokenPosition, to: TokenPosition): boolean {
		const at = this.settledPosition(tokenId);
		if (!at || at.x !== expected.x || at.y !== expected.y) return false;
		if (!this.send('token.move', { tokenId, x: to.x, y: to.y })) return false;
		this.pendingMoves.set(tokenId, to);
		return true;
	}

	// --- undo/redo ---
	//
	// The history is this session's own actions and nothing else, which
	// is what makes undo safe on a shared map: it can only ever reach
	// strokes you drew or erased yourself, never someone else's work
	// that happens to be more recent. It needs no server support either
	// — undoing a drawing is an erase and undoing an erase is a drawing,
	// undoing a move is a move back — all of which already exist, already
	// check permission, and (for drawings) already render optimistically.

	/** Who's at the table right now, in the order they first joined. */
	get connectedParticipants(): Participant[] {
		// Thrown away before this getter returns, and rebuilt from $state
		// each time the getter runs — so a plain Set is right here, not a
		// SvelteSet. Nothing ever reads it reactively; it exists to make
		// the filter below a lookup rather than a scan.
		// eslint-disable-next-line svelte/prefer-svelte-reactivity
		const connected = new Set(this.connectedParticipantIds);
		return this.participants.filter((p) => connected.has(p.id));
	}

	get canUndo(): boolean {
		return this.undoable.length > 0;
	}

	get canRedo(): boolean {
		return this.redoable.length > 0;
	}

	undo() {
		this.step(
			() => this.undoable,
			(next) => (this.undoable = next),
			(action) => (this.redoable = [...this.redoable, action]),
			(action) => this.reverse(action)
		);
	}

	redo() {
		this.step(
			() => this.redoable,
			(next) => (this.redoable = next),
			(action) => (this.undoable = [...this.undoable, action]),
			(action) => this.apply(action)
		);
	}

	// Walks back through the stack until something actually applies.
	// An entry can be unusable by the time it's reached — a GM may have
	// erased the stroke you were about to undo, or the scene may no
	// longer hold it — and the useful behaviour there is to skip it and
	// undo the next thing you did, not to fail on the whole gesture.
	private step(
		read: () => HistoryAction[],
		write: (next: HistoryAction[]) => void,
		push: (action: HistoryAction) => void,
		attempt: (action: HistoryAction) => boolean
	) {
		while (read().length > 0) {
			const stack = read();
			const action = stack[stack.length - 1];
			write(stack.slice(0, -1));

			if (attempt(action)) {
				push(action);
				return;
			}
		}
	}

	private reverse(action: HistoryAction): boolean {
		switch (action.kind) {
			case 'create':
				return this.sendErase(action.drawing.id) !== null;
			case 'erase':
				return this.sendCreate(action.drawing);
			case 'deleteToken':
				return this.sendCreateToken(action.token);
			case 'moveToken':
				return this.sendMoveToken(action.tokenId, action.to, action.from);
		}
	}

	private apply(action: HistoryAction): boolean {
		switch (action.kind) {
			case 'create':
				return this.sendCreate(action.drawing);
			case 'erase':
				return this.sendErase(action.drawing.id) !== null;
			case 'deleteToken':
				return this.sendDeleteToken(action.token.id) !== null;
			case 'moveToken':
				return this.sendMoveToken(action.tokenId, action.from, action.to);
		}
	}

	// Doing something new abandons the branch you had undone your way
	// out of, so there is nothing left to redo.
	private record(action: HistoryAction) {
		this.undoable = [...this.undoable, action];
		this.redoable = [];
	}

	// Anything in flight or on the history is moot once the server hands
	// over a full picture: the actions refer to drawings in a scene that
	// may not even be the one now on screen, and re-creating one there
	// would drop a stroke into a map it never belonged to.
	private resetAfterSync() {
		this.pendingErases.clear();
		this.pendingMoves.clear();
		this.undoable = [];
		this.redoable = [];
		// Measurements belong to a scene that may no longer be the one on
		// screen, and there's no end event coming for them.
		this.cancelPendingMeasure();
		this.measurements = [];
	}

	private restoreErased(drawingId: string) {
		const pending = this.pendingErases.get(drawingId);
		if (!pending) return;
		this.pendingErases.delete(drawingId);

		const restored = [...this.drawings];
		restored.splice(Math.min(pending.index, restored.length), 0, pending.drawing);
		this.drawings = restored;
	}

	// Rate limited on this side only: a ping now pulses for a few
	// seconds, so a rapid double-click would drop a second marker over
	// the spot the first one is trying to draw attention to. Nothing
	// stops a client that doesn't want to be limited — if that ever
	// matters, the check belongs in the hub's handlePing instead.
	sendPing(sceneId: string, x: number, y: number) {
		const now = Date.now();
		if (now - this.lastPingSentAt < PING_COOLDOWN_MS) return;
		if (this.send('ping', { sceneId, x, y })) {
			this.lastPingSentAt = now;
		}
	}

	// --- measuring ---
	//
	// The local measurement goes into state immediately and the wire is
	// paced separately: the person dragging gets a line that tracks their
	// pointer exactly, and everyone else gets an update at most every
	// MEASURE_SEND_INTERVAL_MS. Their own echo is ignored on arrival
	// (see handleEnvelope), so a throttled update can't arrive late and
	// drag their own line back to where the pointer used to be.

	// Area templates ride this same path rather than a channel of their
	// own: one gesture per participant, the same throttle, the same
	// cleanup on disconnect. Only the shape differs.
	updateMeasure(
		sceneId: string,
		from: DrawingPoint,
		to: DrawingPoint,
		kind: MeasurementKind = 'distance',
		widthFeet?: number
	) {
		const you = this.you;
		if (!you) return;

		const measurement: Measurement = {
			participantId: you.participantId,
			participantName: you.displayName,
			sceneId,
			kind,
			from,
			to,
			widthFeet
		};
		this.measurements = upsertMeasurement(this.measurements, measurement);

		this.unsentMeasure = measurement;
		// No timer running means nothing has gone out recently, so this
		// one leaves straight away and starts the interval.
		if (this.measureTimer === null) this.flushMeasure();
	}

	endMeasure() {
		const you = this.you;
		if (!you) return;

		// Anything still waiting to be sent is dropped rather than raced
		// against the end: it would only put the line back on maps that
		// are about to lose it anyway.
		this.cancelPendingMeasure();
		this.measurements = this.measurements.filter((m) => m.participantId !== you.participantId);
		this.send('measure.end', {});
	}

	// Trailing-edge throttle: sends whatever the latest position was, then
	// holds the wire for the interval. If nothing new arrived by the time
	// it comes back round, the timer stops rather than idling — so a
	// stationary pointer costs nothing, and the last position of a drag
	// always goes out even if it landed mid-interval.
	private flushMeasure() {
		const measurement = this.unsentMeasure;
		if (!measurement) {
			this.measureTimer = null;
			return;
		}

		this.unsentMeasure = null;
		this.send('measure.update', {
			sceneId: measurement.sceneId,
			kind: measurement.kind,
			from: measurement.from,
			to: measurement.to,
			widthFeet: measurement.widthFeet ?? 0
		});
		this.measureTimer = setTimeout(() => this.flushMeasure(), MEASURE_SEND_INTERVAL_MS);
	}

	private cancelPendingMeasure() {
		if (this.measureTimer !== null) clearTimeout(this.measureTimer);
		this.measureTimer = null;
		this.unsentMeasure = null;
	}

	private handleEnvelope(env: Envelope) {
		switch (env.type) {
			case 'state.sync': {
				const payload = env.payload as StateSyncPayload;
				this.roomName = payload.room.name;
				this.you = payload.you;
				// server returns newest-first; the log reads top-to-bottom.
				this.messages = [...(payload.messages ?? [])].reverse();
				this.scenes = payload.scenes ?? [];
				this.scene = payload.scene ?? null;
				this.tokens = payload.tokens ?? [];
				this.fogCells = payload.fogCells ?? [];
				this.drawings = payload.drawings ?? [];
				this.participants = payload.participants ?? [];
				this.connectedParticipantIds = payload.connectedParticipantIds ?? [];
				this.resetAfterSync();
				break;
			}

			// Carries the whole participant, not just an id: someone joining
			// for the first time isn't on anyone else's roster yet, so this
			// upserts. There is no echo of your own arrival — your state.sync
			// already listed you among the connected.
			case 'participant.connected': {
				const participant = env.payload as Participant;
				const existing = this.participants.findIndex((p) => p.id === participant.id);
				this.participants =
					existing === -1
						? [...this.participants, participant]
						: this.participants.map((p) => (p.id === participant.id ? participant : p));
				if (!this.connectedParticipantIds.includes(participant.id)) {
					this.connectedParticipantIds = [...this.connectedParticipantIds, participant.id];
				}
				break;
			}

			// Only the connected list. They stay on the roster, which is
			// everyone who has ever joined — what changed is whether they're
			// at the table, not whether they exist.
			case 'participant.disconnected': {
				const payload = env.payload as ParticipantDisconnectedPayload;
				this.connectedParticipantIds = this.connectedParticipantIds.filter(
					(id) => id !== payload.participantId
				);
				break;
			}

			case 'chat.posted':
				this.messages = [...this.messages, env.payload as ChatMessage];
				break;

			case 'token.created':
				this.tokens = [...this.tokens, env.payload as Token];
				break;

			// An upsert, not a replace. A token that was hidden and has just
			// been revealed arrives here at a client that has never held it —
			// the server sends the whole token precisely so this case needs
			// no separate event. Going the other way (revealed to hidden) a
			// Player gets token.deleted instead, since an event withheld from
			// them can't tell them to stop looking at something.
			case 'token.updated': {
				const token = env.payload as Token;
				const existing = this.tokens.findIndex((t) => t.id === token.id);
				this.tokens =
					existing === -1
						? [...this.tokens, token]
						: this.tokens.map((t) => (t.id === token.id ? token : t));
				break;
			}

			case 'token.deleted': {
				const payload = env.payload as TokenDeletedPayload;
				// No broadcast is coming for a move of a token that no longer
				// exists, so the entry would sit here for the rest of the
				// session — and answer for the id if the token ever came back.
				this.pendingMoves.delete(payload.tokenId);
				this.tokens = this.tokens.filter((t) => t.id !== payload.tokenId);
				break;
			}

			case 'token.moved': {
				const payload = env.payload as TokenMovedPayload;
				// Our own move has landed once the room is told about the
				// square we asked for; anything else is someone else's move
				// arriving first, and ours is still on its way. Matching on
				// the coordinates rather than just the id is what tells the
				// two apart — the hub's events carry no sender.
				const pending = this.pendingMoves.get(payload.tokenId);
				if (pending && pending.x === payload.x && pending.y === payload.y) {
					this.pendingMoves.delete(payload.tokenId);
				}
				const current = this.tokens.find((t) => t.id === payload.tokenId);
				// Dropping a token back on the cell it started from still
				// broadcasts, and the hub only ever broadcasts coordinates it
				// accepted — a refusal comes back as an error instead. So an
				// event that matches what we already hold changes nothing, and
				// reassigning would rebuild every token group for it, briefly
				// destroying the one under the pointer if a second drag has
				// already begun.
				if (!current || (current.x === payload.x && current.y === payload.y)) break;
				this.tokens = this.tokens.map((t) =>
					t.id === payload.tokenId ? { ...t, x: payload.x, y: payload.y } : t
				);
				break;
			}

			case 'fog.revealed': {
				const payload = env.payload as FogCellsPayload;
				if (this.scene?.id === payload.sceneId) {
					this.fogCells = mergeFogCells(this.fogCells, payload.cells);
				}
				break;
			}

			case 'fog.hidden': {
				const payload = env.payload as FogCellsPayload;
				if (this.scene?.id === payload.sceneId) {
					this.fogCells = removeFogCells(this.fogCells, payload.cells);
				}
				break;
			}

			case 'fog.reset': {
				const payload = env.payload as FogResetPayload;
				if (this.scene?.id === payload.sceneId) {
					this.fogCells = [];
				}
				break;
			}

			case 'scene.activated': {
				const payload = env.payload as SceneActivatedPayload;
				this.scene = payload.scene;
				this.scenes = upsertScene(this.scenes, payload.scene);
				this.tokens = payload.tokens ?? [];
				this.fogCells = payload.fogCells ?? [];
				this.drawings = payload.drawings ?? [];
				this.resetAfterSync();
				break;
			}

			case 'scene.created': {
				const payload = env.payload as ScenePayload;
				this.scenes = upsertScene(this.scenes, payload.scene);
				break;
			}

			// Only the scene itself, never the tokens/fog/drawings on it.
			// A map swap leaves all of that in place, and folding it in
			// through scene.activated instead would make the client treat a
			// change of backdrop as a change of scene — throwing away undo
			// history and any gesture in flight.
			case 'scene.updated': {
				const payload = env.payload as ScenePayload;
				this.scenes = upsertScene(this.scenes, payload.scene);
				if (this.scene?.id === payload.scene.id) this.scene = payload.scene;
				break;
			}

			case 'scene.deleted': {
				const payload = env.payload as SceneDeletedPayload;
				this.scenes = this.scenes.filter((s) => s.id !== payload.sceneId);
				break;
			}

			case 'drawing.created': {
				const drawing = env.payload as Drawing;
				// Replace rather than append when this is the echo of a
				// stroke already drawn locally: same id, but the server's
				// copy is the authoritative one.
				const existing = this.drawings.findIndex((d) => d.id === drawing.id);
				if (existing === -1) {
					this.drawings = [...this.drawings, drawing];
				} else {
					this.drawings = this.drawings.map((d) => (d.id === drawing.id ? drawing : d));
				}
				break;
			}

			case 'drawing.deleted': {
				const payload = env.payload as DrawingDeletedPayload;
				this.pendingErases.delete(payload.drawingId);
				// Usually this is the server confirming an erase already
				// applied optimistically, so the drawing has been gone since
				// the click. Reassigning anyway would be a second identical
				// list, and a second re-render of the canvas for nothing.
				if (this.drawings.some((d) => d.id === payload.drawingId)) {
					this.drawings = this.drawings.filter((d) => d.id !== payload.drawingId);
				}
				break;
			}

			case 'ping': {
				const payload = env.payload as PingPayload;
				const ping: Ping = { id: crypto.randomUUID(), ...payload };
				this.pings = [...this.pings, ping];
				setTimeout(() => {
					this.pings = this.pings.filter((p) => p.id !== ping.id);
				}, PING_LIFETIME_MS);
				break;
			}

			case 'measure.updated': {
				const measurement = env.payload as Measurement;
				// Own echo: the local copy is already ahead of it.
				if (measurement.participantId === this.you?.participantId) break;
				if (this.scene?.id !== measurement.sceneId) break;
				this.measurements = upsertMeasurement(this.measurements, measurement);
				break;
			}

			case 'measure.ended': {
				const payload = env.payload as MeasureEndedPayload;
				if (this.measurements.some((m) => m.participantId === payload.participantId)) {
					this.measurements = this.measurements.filter(
						(m) => m.participantId !== payload.participantId
					);
				}
				break;
			}

			case 'error': {
				const payload = env.payload as ErrorPayload;
				this.error = payload.message;
				if (payload.drawingId) {
					// Either an erase was refused, so the stroke goes back, or
					// a drawing was, so the one shown locally comes off.
					if (this.pendingErases.has(payload.drawingId)) {
						this.restoreErased(payload.drawingId);
					} else {
						this.drawings = this.drawings.filter((d) => d.id !== payload.drawingId);
					}
				}
				break;
			}
		}
	}
}

// One measurement per participant: a new position for someone already
// measuring replaces theirs in place, keeping the list stable rather
// than growing one entry per pointer move.
function upsertMeasurement(existing: Measurement[], measurement: Measurement): Measurement[] {
	if (!existing.some((m) => m.participantId === measurement.participantId)) {
		return [...existing, measurement];
	}
	return existing.map((m) => (m.participantId === measurement.participantId ? measurement : m));
}

function mergeFogCells(existing: FogCell[], added: FogCell[]): FogCell[] {
	// Local, throwaway dedup set — never touches component state, so a
	// plain Set (not SvelteSet) is correct here.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	const seen = new Set(existing.map((c) => `${c.x},${c.y}`));
	const merged = [...existing];
	for (const c of added) {
		const key = `${c.x},${c.y}`;
		if (!seen.has(key)) {
			seen.add(key);
			merged.push(c);
		}
	}
	return merged;
}

// Replaces a scene in the list, or appends it if it's new. Both cases
// arrive: scene.created is always new, scene.updated never is, and
// scene.activated can be either — a client that connected before a scene
// existed still gets activated onto it.
function upsertScene(existing: Scene[], scene: Scene): Scene[] {
	if (!existing.some((s) => s.id === scene.id)) return [...existing, scene];
	return existing.map((s) => (s.id === scene.id ? scene : s));
}

function removeFogCells(existing: FogCell[], removed: FogCell[]): FogCell[] {
	// Same throwaway-Set reasoning as mergeFogCells above — no disable
	// comment needed here, though: the lint rule only fires on a Set that
	// is written to after construction.
	const drop = new Set(removed.map((c) => `${c.x},${c.y}`));
	return existing.filter((c) => !drop.has(`${c.x},${c.y}`));
}
