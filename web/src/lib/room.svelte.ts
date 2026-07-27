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
}

interface ErrorPayload {
	message: string;
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

	private socket: WebSocket | null = null;

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

	private send(type: string, payload: unknown) {
		if (this.socket?.readyState !== WebSocket.OPEN) return;
		this.socket.send(JSON.stringify({ type, payload }));
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

	createToken(sceneId: string, name: string, imageAssetId: string | null, x: number, y: number) {
		this.send('token.create', { sceneId, name, imageAssetId, x, y });
	}

	revealFog(sceneId: string, cells: FogCell[]) {
		this.send('fog.reveal', { sceneId, cells });
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
				break;
			}

			case 'error':
				this.error = (env.payload as ErrorPayload).message;
				break;
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
