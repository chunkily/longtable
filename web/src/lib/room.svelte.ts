// Reactive WebSocket client for a single room. Wraps the raw
// command/event protocol (see internal/ws/hub.go) in Svelte 5 runes
// state so components just read fields and let the UI update itself.

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

interface ErrorPayload {
	message: string;
	// Present when the failure is attributable to one drawing, which is
	// how an optimistically-rendered stroke knows to take itself back.
	drawingId?: string;
}

// How long a ping marker stays on screen before RoomClient removes it.
const PING_LIFETIME_MS = 1500;

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
	createDrawing(sceneId: string, kind: DrawingKind, points: DrawingPoint[], color: string) {
		const drawingId = crypto.randomUUID();
		if (!this.send('draw.create', { drawingId, sceneId, kind, points, color })) return;

		this.drawings = [
			...this.drawings,
			{
				id: drawingId,
				sceneId,
				kind,
				points,
				color,
				// Claimed locally so the eraser treats it as yours right
				// away; the server sets the real author on the echo, from
				// the connection rather than from anything sent here.
				createdByParticipantId: this.you?.participantId ?? null
			}
		];
	}

	// Also optimistic: the eraser highlights what a click will remove,
	// so leaving the stroke sitting there until the server agrees reads
	// as a missed click. A refusal (erasing what isn't yours, or what
	// someone else just erased) comes back naming the drawing, and puts
	// it back where it was.
	deleteDrawing(drawingId: string) {
		const index = this.drawings.findIndex((d) => d.id === drawingId);
		if (index === -1) return;
		if (!this.send('draw.delete', { drawingId })) return;

		this.pendingErases.set(drawingId, { drawing: this.drawings[index], index });
		this.drawings = this.drawings.filter((d) => d.id !== drawingId);
	}

	// Anything still in flight is moot once the server hands over a full
	// picture of the scene.
	private resetPending() {
		this.pendingErases.clear();
	}

	private restoreErased(drawingId: string) {
		const pending = this.pendingErases.get(drawingId);
		if (!pending) return;
		this.pendingErases.delete(drawingId);

		const restored = [...this.drawings];
		restored.splice(Math.min(pending.index, restored.length), 0, pending.drawing);
		this.drawings = restored;
	}

	sendPing(sceneId: string, x: number, y: number) {
		this.send('ping', { sceneId, x, y });
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
				this.resetPending();
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
				this.resetPending();
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
				this.drawings = this.drawings.filter((d) => d.id !== payload.drawingId);
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
