// Reactive WebSocket client for a single room. Wraps the raw
// command/event protocol (see internal/ws/hub.go) in Svelte 5 runes
// state so components just read fields and let the UI update itself.

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

export interface You {
	participantId: string;
	displayName: string;
	role: 'gm' | 'player';
}

type ConnectionStatus = 'connecting' | 'open' | 'closed';

interface Envelope {
	type: string;
	payload: unknown;
}

interface StateSyncPayload {
	room: { slug: string; name: string };
	you: You;
	messages?: ChatMessage[];
	scene?: Scene | null;
	tokens?: Token[];
	fogCells?: FogCell[];
	drawings?: Drawing[];
}

interface TokenMovedPayload {
	tokenId: string;
	x: number;
	y: number;
}

interface FogRevealedPayload {
	sceneId: string;
	cells: FogCell[];
}

interface SceneActivatedPayload {
	scene: Scene;
	tokens?: Token[];
	fogCells?: FogCell[];
	drawings?: Drawing[];
}

interface DrawingDeletedPayload {
	drawingId: string;
}

interface PingPayload {
	sceneId: string;
	x: number;
	y: number;
	participantName: string;
}

// One reversible thing this session did to the map. Both directions
// carry the whole drawing, because undoing an erase has to put it back
// exactly as it was, and redoing a drawing has to recreate it under the
// same id.
//
// A caveat that comes with restoring: the server takes a drawing's
// author from the connection that sent it and ignores anything claimed
// in the payload, so a GM who erases a Player's stroke and then undoes
// gets the stroke back under their own name. It looks identical, but
// the Player can no longer erase it themselves.
interface DrawingAction {
	kind: 'create' | 'erase';
	drawing: Drawing;
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

	roomName = $state('');
	you = $state<You | null>(null);

	messages = $state<ChatMessage[]>([]);
	scene = $state<Scene | null>(null);
	tokens = $state<Token[]>([]);
	fogCells = $state<FogCell[]>([]);
	drawings = $state<Drawing[]>([]);
	pings = $state<Ping[]>([]);

	private socket: WebSocket | null = null;

	// Drawings taken off the map before the server confirmed the erase,
	// kept with the position they held so a refusal can put them back
	// where they were rather than on top of everything. Bookkeeping only
	// — nothing reactive reads it, so a plain Map is right here.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	private pendingErases = new Map<string, { drawing: Drawing; index: number }>();

	private lastPingSentAt = 0;

	// This session's own drawing actions, oldest first. Reactive so the
	// toolbar can tell whether there is anything left to undo or redo.
	private undoable = $state<DrawingAction[]>([]);
	private redoable = $state<DrawingAction[]>([]);

	connect(slug: string, sessionToken: string) {
		this.disconnect();
		this.status = 'connecting';
		this.error = '';

		const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${proto}//${window.location.host}/ws?room=${encodeURIComponent(slug)}&token=${encodeURIComponent(sessionToken)}`;

		const socket = new WebSocket(url);
		this.socket = socket;

		socket.onopen = () => {
			this.status = 'open';
		};
		socket.onclose = () => {
			this.status = 'closed';
		};
		socket.onerror = () => {
			this.error = 'connection error';
		};
		socket.onmessage = (event) => {
			this.handleEnvelope(JSON.parse(event.data));
		};
	}

	disconnect() {
		this.socket?.close();
		this.socket = null;
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

	moveToken(tokenId: string, x: number, y: number) {
		this.send('token.move', { tokenId, x, y });
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

	createToken(
		sceneId: string,
		name: string,
		imageAssetId: string | null,
		x: number,
		y: number,
		visibility: 'visible' | 'hidden' = 'visible'
	) {
		this.send('token.create', { sceneId, name, imageAssetId, x, y, visibility });
	}

	revealFog(sceneId: string, cells: FogCell[]) {
		this.send('fog.reveal', { sceneId, cells });
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

	// --- undo/redo ---
	//
	// The history is this session's own actions and nothing else, which
	// is what makes undo safe on a shared map: it can only ever reach
	// strokes you drew or erased yourself, never someone else's work
	// that happens to be more recent. It needs no server support either
	// — undoing a drawing is an erase and undoing an erase is a drawing,
	// both of which already exist, already check permission, and already
	// render optimistically.

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
		read: () => DrawingAction[],
		write: (next: DrawingAction[]) => void,
		push: (action: DrawingAction) => void,
		attempt: (action: DrawingAction) => boolean
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

	private reverse(action: DrawingAction): boolean {
		return action.kind === 'create'
			? this.sendErase(action.drawing.id) !== null
			: this.sendCreate(action.drawing);
	}

	private apply(action: DrawingAction): boolean {
		return action.kind === 'create'
			? this.sendCreate(action.drawing)
			: this.sendErase(action.drawing.id) !== null;
	}

	// Doing something new abandons the branch you had undone your way
	// out of, so there is nothing left to redo.
	private record(action: DrawingAction) {
		this.undoable = [...this.undoable, action];
		this.redoable = [];
	}

	// Anything in flight or on the history is moot once the server hands
	// over a full picture: the actions refer to drawings in a scene that
	// may not even be the one now on screen, and re-creating one there
	// would drop a stroke into a map it never belonged to.
	private resetAfterSync() {
		this.pendingErases.clear();
		this.undoable = [];
		this.redoable = [];
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

	private handleEnvelope(env: Envelope) {
		switch (env.type) {
			case 'state.sync': {
				const payload = env.payload as StateSyncPayload;
				this.roomName = payload.room.name;
				this.you = payload.you;
				// server returns newest-first; the log reads top-to-bottom.
				this.messages = [...(payload.messages ?? [])].reverse();
				this.scene = payload.scene ?? null;
				this.tokens = payload.tokens ?? [];
				this.fogCells = payload.fogCells ?? [];
				this.drawings = payload.drawings ?? [];
				this.resetAfterSync();
				break;
			}

			case 'chat.posted':
				this.messages = [...this.messages, env.payload as ChatMessage];
				break;

			case 'token.created':
				this.tokens = [...this.tokens, env.payload as Token];
				break;

			case 'token.moved': {
				const payload = env.payload as TokenMovedPayload;
				this.tokens = this.tokens.map((t) =>
					t.id === payload.tokenId ? { ...t, x: payload.x, y: payload.y } : t
				);
				break;
			}

			case 'fog.revealed': {
				const payload = env.payload as FogRevealedPayload;
				if (this.scene?.id === payload.sceneId) {
					this.fogCells = mergeFogCells(this.fogCells, payload.cells);
				}
				break;
			}

			case 'scene.activated': {
				const payload = env.payload as SceneActivatedPayload;
				this.scene = payload.scene;
				this.tokens = payload.tokens ?? [];
				this.fogCells = payload.fogCells ?? [];
				this.drawings = payload.drawings ?? [];
				this.resetAfterSync();
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
