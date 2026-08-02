import { describe, expect, it, vi } from 'vitest';
import { RoomClient } from './room.svelte';
import { MEASURE_SEND_INTERVAL_MS } from './measure';
import { PING_COOLDOWN_MS, PING_LIFETIME_MS } from './ping';

// A minimal stand-in for the browser WebSocket, driven entirely from
// the test so we can assert on RoomClient's reaction to specific
// envelopes without reaching into its private internals.
class FakeWebSocket {
	static OPEN = 1;
	static instances: FakeWebSocket[] = [];

	readyState = FakeWebSocket.OPEN;
	sent: string[] = [];
	onopen: (() => void) | null = null;
	onclose: (() => void) | null = null;
	onerror: (() => void) | null = null;
	onmessage: ((event: { data: string }) => void) | null = null;

	constructor(public url: string) {
		FakeWebSocket.instances.push(this);
	}

	send(data: string) {
		this.sent.push(data);
	}

	close() {
		this.onclose?.();
	}

	emit(envelope: { type: string; payload: unknown }) {
		this.onmessage?.({ data: JSON.stringify(envelope) });
	}
}

function connectedClient() {
	FakeWebSocket.instances = [];
	vi.stubGlobal('WebSocket', FakeWebSocket);

	const client = new RoomClient();
	client.connect('abc123', 'session-token');
	const socket = FakeWebSocket.instances.at(-1)!;
	return { client, socket };
}

describe('RoomClient', () => {
	it('builds the ws URL from the room slug and session token', () => {
		const { socket } = connectedClient();
		expect(socket.url).toContain('room=abc123');
		expect(socket.url).toContain('token=session-token');
	});

	it('reverses state.sync messages so the log reads oldest-first', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Test Room' },
				you: { participantId: 'p1', displayName: 'Alice', role: 'gm' },
				messages: [
					{ id: 'm2', createdAt: '2', body: 'second' },
					{ id: 'm1', createdAt: '1', body: 'first' }
				]
			}
		});

		expect(client.roomName).toBe('Test Room');
		expect(client.you?.participantId).toBe('p1');
		expect(client.messages.map((m) => m.id)).toEqual(['m1', 'm2']);
	});

	it('appends chat.posted messages to the existing log', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' }
			}
		});
		socket.emit({ type: 'chat.posted', payload: { id: 'm1', body: 'hi' } });

		expect(client.messages).toHaveLength(1);
		expect(client.messages[0].body).toBe('hi');
	});

	it('patches only the moved token on token.moved', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				tokens: [
					{ id: 't1', x: 0, y: 0, name: 'A' },
					{ id: 't2', x: 5, y: 5, name: 'B' }
				]
			}
		});

		socket.emit({ type: 'token.moved', payload: { tokenId: 't1', x: 10, y: 20 } });

		const t1 = client.tokens.find((t) => t.id === 't1')!;
		const t2 = client.tokens.find((t) => t.id === 't2')!;
		expect(t1.x).toBe(10);
		expect(t1.y).toBe(20);
		expect(t2.x).toBe(5); // untouched
	});

	// Identity is the assertion rather than contents because identity is
	// what the canvas re-renders on: a token dropped back where it started
	// still broadcasts, and holding the same array is what stops that from
	// rebuilding every token group for a move of nowhere.
	it('holds the same tokens array when token.moved changes nothing', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				tokens: [{ id: 't1', x: 3, y: 4, name: 'A' }]
			}
		});
		const before = client.tokens;

		socket.emit({ type: 'token.moved', payload: { tokenId: 't1', x: 3, y: 4 } });
		expect(client.tokens).toBe(before);

		// An id the client isn't holding — a token on a scene it isn't
		// showing — leaves the list alone too.
		socket.emit({ type: 'token.moved', payload: { tokenId: 'nope', x: 1, y: 1 } });
		expect(client.tokens).toBe(before);

		// ...but a real move still lands.
		socket.emit({ type: 'token.moved', payload: { tokenId: 't1', x: 3, y: 5 } });
		expect(client.tokens).not.toBe(before);
		expect(client.tokens[0].y).toBe(5);
	});

	it('merges fog.revealed cells only when they belong to the active scene', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scene: { id: 'scene1', name: 'Scene' },
				fogCells: [{ x: 0, y: 0 }]
			}
		});

		socket.emit({ type: 'fog.revealed', payload: { sceneId: 'scene1', cells: [{ x: 1, y: 1 }] } });
		expect(client.fogCells).toEqual([
			{ x: 0, y: 0 },
			{ x: 1, y: 1 }
		]);

		// a reveal for a scene the client isn't currently viewing must be ignored
		socket.emit({
			type: 'fog.revealed',
			payload: { sceneId: 'other-scene', cells: [{ x: 9, y: 9 }] }
		});
		expect(client.fogCells).toEqual([
			{ x: 0, y: 0 },
			{ x: 1, y: 1 }
		]);
	});

	it('drops fog.hidden cells only when they belong to the active scene', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scene: { id: 'scene1', name: 'Scene' },
				fogCells: [
					{ x: 0, y: 0 },
					{ x: 1, y: 1 }
				]
			}
		});

		socket.emit({ type: 'fog.hidden', payload: { sceneId: 'scene1', cells: [{ x: 0, y: 0 }] } });
		expect(client.fogCells).toEqual([{ x: 1, y: 1 }]);

		// Hiding a cell that isn't revealed is a no-op rather than an error —
		// the hide tool sweeps across cells it never revealed.
		socket.emit({ type: 'fog.hidden', payload: { sceneId: 'scene1', cells: [{ x: 7, y: 7 }] } });
		expect(client.fogCells).toEqual([{ x: 1, y: 1 }]);

		socket.emit({
			type: 'fog.hidden',
			payload: { sceneId: 'other-scene', cells: [{ x: 1, y: 1 }] }
		});
		expect(client.fogCells).toEqual([{ x: 1, y: 1 }]);
	});

	it('clears every cell on fog.reset, but only for the active scene', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scene: { id: 'scene1', name: 'Scene' },
				fogCells: [
					{ x: 0, y: 0 },
					{ x: 1, y: 1 }
				]
			}
		});

		socket.emit({ type: 'fog.reset', payload: { sceneId: 'other-scene' } });
		expect(client.fogCells).toHaveLength(2);

		socket.emit({ type: 'fog.reset', payload: { sceneId: 'scene1' } });
		expect(client.fogCells).toEqual([]);
	});

	// Reveal-all deliberately reuses fog.revealed rather than an event of
	// its own, so there is nothing extra for the client to handle — this
	// pins that, since a future server change growing a fog.revealedAll
	// event would silently do nothing here.
	it('takes a whole-scene reveal through the same fog.revealed case', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scene: { id: 'scene1', name: 'Scene' },
				fogCells: [{ x: 0, y: 0 }]
			}
		});

		client.revealAllFog('scene1');
		expect(JSON.parse(socket.sent.at(-1)!)).toEqual({
			type: 'fog.revealAll',
			payload: { sceneId: 'scene1' }
		});

		socket.emit({
			type: 'fog.revealed',
			payload: {
				sceneId: 'scene1',
				cells: [
					{ x: 0, y: 0 },
					{ x: 1, y: 0 }
				]
			}
		});
		// (0,0) was already revealed and must not double up.
		expect(client.fogCells).toEqual([
			{ x: 0, y: 0 },
			{ x: 1, y: 0 }
		]);
	});

	it('keeps the scene list in step with created, updated and deleted', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scenes: [{ id: 's1', name: 'Tavern' }],
				scene: { id: 's1', name: 'Tavern' }
			}
		});
		expect(client.scenes.map((s) => s.id)).toEqual(['s1']);

		socket.emit({ type: 'scene.created', payload: { scene: { id: 's2', name: 'Dungeon' } } });
		expect(client.scenes.map((s) => s.id)).toEqual(['s1', 's2']);
		// Creating a scene must not drag anyone onto it.
		expect(client.scene?.id).toBe('s1');

		socket.emit({ type: 'scene.deleted', payload: { sceneId: 's2' } });
		expect(client.scenes.map((s) => s.id)).toEqual(['s1']);
	});

	// A map swap changes the backdrop and nothing else. Anything that
	// reset tokens/fog/drawings here would be discarding a session's
	// progress for a change of art.
	it('applies scene.updated to the active scene without clearing what is on it', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scenes: [{ id: 's1', name: 'Tavern', mapAssetId: 'old' }],
				scene: { id: 's1', name: 'Tavern', mapAssetId: 'old' },
				tokens: [{ id: 't1', x: 1, y: 1, name: 'Goblin' }],
				fogCells: [{ x: 0, y: 0 }],
				drawings: [{ id: 'd1', sceneId: 's1', kind: 'line', points: [], color: '#000' }]
			}
		});

		socket.emit({
			type: 'scene.updated',
			payload: { scene: { id: 's1', name: 'Tavern', mapAssetId: 'new' } }
		});

		expect(client.scene?.mapAssetId).toBe('new');
		expect(client.scenes[0].mapAssetId).toBe('new');
		expect(client.tokens).toHaveLength(1);
		expect(client.fogCells).toEqual([{ x: 0, y: 0 }]);
		expect(client.drawings).toHaveLength(1);
	});

	it('ignores scene.updated for a scene that is not the one on screen', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scenes: [
					{ id: 's1', name: 'Tavern' },
					{ id: 's2', name: 'Dungeon', mapAssetId: 'old' }
				],
				scene: { id: 's1', name: 'Tavern' }
			}
		});

		socket.emit({
			type: 'scene.updated',
			payload: { scene: { id: 's2', name: 'Dungeon', mapAssetId: 'new' } }
		});

		// The list learns about it; the canvas doesn't change scene.
		expect(client.scenes[1].mapAssetId).toBe('new');
		expect(client.scene?.id).toBe('s1');
	});

	it('replaces scene/tokens/fog wholesale on scene.activated', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'scene.activated',
			payload: { scene: { id: 's2', name: 'New Scene' }, tokens: [{ id: 't9' }] }
		});

		expect(client.scene?.id).toBe('s2');
		expect(client.tokens.map((t) => t.id)).toEqual(['t9']);
		expect(client.fogCells).toEqual([]);
	});

	it('surfaces error envelopes on the error field', () => {
		const { client, socket } = connectedClient();
		socket.emit({ type: 'error', payload: { message: 'that action is GM-only' } });
		expect(client.error).toBe('that action is GM-only');
	});

	it('does not send while the socket is not open', () => {
		const { client, socket } = connectedClient();
		socket.readyState = 0; // connecting, not open
		client.sendChat('hello');
		expect(socket.sent).toHaveLength(0);
	});

	it('sends chat.send with the given text once open', () => {
		const { client, socket } = connectedClient();
		client.sendChat('hello');
		expect(socket.sent).toHaveLength(1);
		expect(JSON.parse(socket.sent[0])).toEqual({ type: 'chat.send', payload: { text: 'hello' } });
	});

	it('loads drawings from state.sync and appends drawing.created events', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings: [
					{
						id: 'd1',
						sceneId: 's1',
						kind: 'line',
						points: [],
						color: '#cc0000',
						createdByParticipantId: 'p2'
					}
				]
			}
		});
		expect(client.drawings.map((d) => d.id)).toEqual(['d1']);

		socket.emit({
			type: 'drawing.created',
			payload: {
				id: 'd2',
				sceneId: 's1',
				kind: 'rect',
				points: [],
				color: '#0033cc',
				createdByParticipantId: 'p1'
			}
		});
		expect(client.drawings.map((d) => d.id)).toEqual(['d1', 'd2']);
	});

	it('keeps the author of each drawing, so your own can be told from other people', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings: [
					{
						id: 'd1',
						sceneId: 's1',
						kind: 'line',
						points: [],
						color: '#cc0000',
						createdByParticipantId: 'p2'
					},
					// Drawings predating authorship tracking, or whose author
					// has left the room, arrive with a null author.
					{
						id: 'd2',
						sceneId: 's1',
						kind: 'line',
						points: [],
						color: '#cc0000',
						createdByParticipantId: null
					}
				]
			}
		});

		socket.emit({
			type: 'drawing.created',
			payload: {
				id: 'd3',
				sceneId: 's1',
				kind: 'rect',
				points: [],
				color: '#0033cc',
				createdByParticipantId: 'p1'
			}
		});

		expect(client.drawings.map((d) => d.createdByParticipantId)).toEqual(['p2', null, 'p1']);
	});

	it('removes a drawing on drawing.deleted', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings: [
					{
						id: 'd1',
						sceneId: 's1',
						kind: 'line',
						points: [],
						color: '#cc0000',
						createdByParticipantId: 'p1'
					},
					{
						id: 'd2',
						sceneId: 's1',
						kind: 'rect',
						points: [],
						color: '#0033cc',
						createdByParticipantId: 'p2'
					}
				]
			}
		});

		socket.emit({ type: 'drawing.deleted', payload: { drawingId: 'd1' } });
		expect(client.drawings.map((d) => d.id)).toEqual(['d2']);

		// An id that isn't on screen (already erased, or from a scene the
		// client isn't showing) leaves the list alone.
		socket.emit({ type: 'drawing.deleted', payload: { drawingId: 'd1' } });
		expect(client.drawings.map((d) => d.id)).toEqual(['d2']);
	});

	it('sends draw.delete with the drawing id', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings: [{ id: 'd1', sceneId: 's1', kind: 'line', points: [], color: '#cc0000' }]
			}
		});

		client.deleteDrawing('d1');
		expect(JSON.parse(socket.sent[0])).toEqual({
			type: 'draw.delete',
			payload: { drawingId: 'd1' }
		});

		// Nothing to erase, nothing to send: without a local copy there
		// would be nothing to put back if the server refused.
		client.deleteDrawing('unknown');
		expect(socket.sent).toHaveLength(1);
	});

	it('resets drawings on scene.activated', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings: [{ id: 'd1', sceneId: 's1', kind: 'line', points: [], color: '#cc0000' }]
			}
		});

		socket.emit({
			type: 'scene.activated',
			payload: {
				scene: { id: 's2' },
				drawings: [{ id: 'd3', sceneId: 's2', kind: 'ellipse', points: [], color: '#008000' }]
			}
		});
		expect(client.drawings.map((d) => d.id)).toEqual(['d3']);
	});

	it('sends draw.create with a generated id, sceneId, kind, points, and color', () => {
		const { client, socket } = connectedClient();
		const points = [
			{ x: 1, y: 2 },
			{ x: 3, y: 4 }
		];
		client.createDrawing('scene1', 'line', points, '#cc0000');

		const sent = JSON.parse(socket.sent[0]);
		expect(sent.type).toBe('draw.create');
		expect(sent.payload).toMatchObject({
			sceneId: 'scene1',
			kind: 'line',
			points,
			color: '#cc0000'
		});
		expect(sent.payload.drawingId).toMatch(
			/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
		);
	});

	it('renders a new drawing immediately, then reconciles the echo by id', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' }
			}
		});

		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		expect(client.drawings).toHaveLength(1);
		// Claimed locally, so the eraser treats it as yours before the
		// server has said anything.
		expect(client.drawings[0].createdByParticipantId).toBe('p1');

		const { drawingId } = JSON.parse(socket.sent[0]).payload;
		socket.emit({
			type: 'drawing.created',
			payload: {
				id: drawingId,
				sceneId: 's1',
				kind: 'line',
				points: [{ x: 0, y: 0 }],
				color: '#cc0000',
				createdByParticipantId: 'p1'
			}
		});

		// The echo replaces the local copy rather than doubling it up.
		expect(client.drawings.map((d) => d.id)).toEqual([drawingId]);
	});

	it('takes back an optimistic drawing the server rejects', () => {
		const { client, socket } = connectedClient();
		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		const { drawingId } = JSON.parse(socket.sent[0]).payload;
		expect(client.drawings).toHaveLength(1);

		socket.emit({
			type: 'error',
			payload: { message: 'unknown drawing kind', drawingId }
		});

		expect(client.drawings).toHaveLength(0);
		expect(client.error).toBe('unknown drawing kind');
	});

	it('does not render a drawing it could not send', () => {
		const { client, socket } = connectedClient();
		socket.readyState = 0; // still connecting

		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');

		// Nothing went out, so nothing may be shown — no echo is coming to
		// confirm or reject it.
		expect(socket.sent).toHaveLength(0);
		expect(client.drawings).toHaveLength(0);
	});

	it('erases immediately and puts the drawing back if the server refuses', () => {
		const { client, socket } = connectedClient();
		const drawings = [
			{ id: 'd1', sceneId: 's1', kind: 'line', points: [], color: '#cc0000' },
			{ id: 'd2', sceneId: 's1', kind: 'rect', points: [], color: '#0033cc' },
			{ id: 'd3', sceneId: 's1', kind: 'line', points: [], color: '#008000' }
		];
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'player' },
				drawings
			}
		});

		client.deleteDrawing('d2');
		expect(client.drawings.map((d) => d.id)).toEqual(['d1', 'd3']);

		socket.emit({
			type: 'error',
			payload: { message: 'you can only erase drawings you created', drawingId: 'd2' }
		});

		// Back in its old position, so it doesn't jump on top of d3.
		expect(client.drawings.map((d) => d.id)).toEqual(['d1', 'd2', 'd3']);
	});

	it('keeps an erase that the server confirms', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings: [{ id: 'd1', sceneId: 's1', kind: 'line', points: [], color: '#cc0000' }]
			}
		});

		client.deleteDrawing('d1');
		socket.emit({ type: 'drawing.deleted', payload: { drawingId: 'd1' } });
		expect(client.drawings).toHaveLength(0);

		// A later unrelated error must not resurrect it.
		socket.emit({ type: 'error', payload: { message: 'something else', drawingId: 'd1' } });
		expect(client.drawings).toHaveLength(0);
	});

	it('forgets in-flight erases when the server sends a full scene', () => {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings: [{ id: 'd1', sceneId: 's1', kind: 'line', points: [], color: '#cc0000' }]
			}
		});

		client.deleteDrawing('d1');
		socket.emit({
			type: 'scene.activated',
			payload: { scene: { id: 's2' }, drawings: [] }
		});

		// The scene the erase belonged to is gone; a late refusal must not
		// drop a stale stroke into the new one.
		socket.emit({ type: 'error', payload: { message: 'drawing not found', drawingId: 'd1' } });
		expect(client.drawings).toHaveLength(0);
	});

	// --- undo/redo ---

	function roomWithDrawings(drawings: unknown[] = []) {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				drawings
			}
		});
		return { client, socket };
	}

	function sentTypes(socket: { sent: string[] }) {
		return socket.sent.map((s) => JSON.parse(s).type);
	}

	it('has nothing to undo or redo until something happens', () => {
		const { client } = roomWithDrawings();
		expect(client.canUndo).toBe(false);
		expect(client.canRedo).toBe(false);

		client.undo();
		client.redo();
		expect(client.drawings).toHaveLength(0);
	});

	it('undoes drawings one at a time, newest first', () => {
		const { client } = roomWithDrawings();
		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		client.createDrawing('s1', 'rect', [{ x: 1, y: 1 }], '#0033cc');
		const [first, second] = client.drawings.map((d) => d.id);

		client.undo();
		expect(client.drawings.map((d) => d.id)).toEqual([first]);

		client.undo();
		expect(client.drawings).toHaveLength(0);
		expect(client.canUndo).toBe(false);

		// And back again, oldest first, under the same ids.
		client.redo();
		expect(client.drawings.map((d) => d.id)).toEqual([first]);
		client.redo();
		expect(client.drawings.map((d) => d.id)).toEqual([first, second]);
		expect(client.canRedo).toBe(false);
	});

	it('undoes an erase by putting the drawing back', () => {
		const { client, socket } = roomWithDrawings([
			{
				id: 'd1',
				sceneId: 's1',
				kind: 'line',
				points: [{ x: 0, y: 0 }],
				color: '#cc0000',
				createdByParticipantId: 'p1'
			}
		]);

		client.deleteDrawing('d1');
		expect(client.drawings).toHaveLength(0);

		client.undo();
		expect(client.drawings.map((d) => d.id)).toEqual(['d1']);
		expect(sentTypes(socket)).toEqual(['draw.delete', 'draw.create']);

		client.redo();
		expect(client.drawings).toHaveLength(0);
	});

	it('never reaches a drawing this session did not touch', () => {
		const { client } = roomWithDrawings([
			{
				id: 'theirs',
				sceneId: 's1',
				kind: 'line',
				points: [],
				color: '#cc0000',
				createdByParticipantId: 'p2'
			}
		]);

		// Nothing of our own has happened, so there is nothing to undo —
		// someone else's stroke being the most recent thing on the map
		// doesn't put it within reach.
		expect(client.canUndo).toBe(false);
		client.undo();
		expect(client.drawings.map((d) => d.id)).toEqual(['theirs']);
	});

	it('drops history entries whose drawing is already gone and undoes the next one', () => {
		const { client, socket } = roomWithDrawings();
		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		client.createDrawing('s1', 'rect', [{ x: 1, y: 1 }], '#0033cc');
		const [first, second] = client.drawings.map((d) => d.id);

		// Someone else erases the newest one out from under us.
		socket.emit({ type: 'drawing.deleted', payload: { drawingId: second } });

		// Undo skips the entry it can no longer act on rather than
		// failing, and takes back the previous drawing instead.
		client.undo();
		expect(client.drawings).toHaveLength(0);
		expect(client.canUndo).toBe(false);

		client.redo();
		expect(client.drawings.map((d) => d.id)).toEqual([first]);
	});

	it('abandons the redo branch once something new is drawn', () => {
		const { client } = roomWithDrawings();
		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		client.undo();
		expect(client.canRedo).toBe(true);

		client.createDrawing('s1', 'rect', [{ x: 1, y: 1 }], '#0033cc');
		expect(client.canRedo).toBe(false);

		client.redo();
		expect(client.drawings).toHaveLength(1);
	});

	it('forgets history when the server sends a full scene', () => {
		const { client, socket } = roomWithDrawings();
		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		expect(client.canUndo).toBe(true);

		socket.emit({
			type: 'scene.activated',
			payload: { scene: { id: 's2' }, drawings: [] }
		});

		// The stroke belonged to a scene that isn't on screen any more;
		// undoing into this one would drop it somewhere it never was.
		expect(client.canUndo).toBe(false);
		expect(client.canRedo).toBe(false);
	});

	it('records nothing for a drawing that could not be sent', () => {
		const { client, socket } = roomWithDrawings();
		socket.readyState = 0; // connecting

		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		expect(client.canUndo).toBe(false);
	});

	// --- deleting tokens ---

	function roomWithTokens(tokens: unknown[] = []) {
		const { client, socket } = connectedClient();
		socket.emit({
			type: 'state.sync',
			payload: {
				room: { slug: 'abc123', name: 'Room' },
				you: { participantId: 'p1', displayName: 'A', role: 'gm' },
				scene: { id: 's1', gridSize: 70 },
				tokens
			}
		});
		return { client, socket };
	}

	const goblin = {
		id: 't1',
		sceneId: 's1',
		name: 'Goblin',
		imageAssetId: null,
		x: 3,
		y: 4,
		width: 2,
		height: 2,
		ownerParticipantId: null,
		visibility: 'visible'
	};

	it('replaces a token it already holds on token.updated', () => {
		const { client, socket } = roomWithTokens([goblin, { ...goblin, id: 't2' }]);

		socket.emit({ type: 'token.updated', payload: { ...goblin, name: 'Hobgoblin', width: 2 } });

		expect(client.tokens.map((t) => t.id)).toEqual(['t1', 't2']);
		expect(client.tokens[0].name).toBe('Hobgoblin');
		expect(client.tokens[0].width).toBe(2);
	});

	// A hidden token being revealed reaches a Player who has never held
	// it, because they were never told it existed. The server sends the
	// whole token for exactly this case rather than a separate event, so
	// an update for an unknown id has to add it rather than be dropped.
	it('adds a token it has never seen on token.updated', () => {
		const { client, socket } = roomWithTokens([goblin]);

		socket.emit({
			type: 'token.updated',
			payload: { ...goblin, id: 'revealed', name: 'Ambusher' }
		});

		expect(client.tokens.map((t) => t.id)).toEqual(['t1', 'revealed']);
	});

	// The other direction: a Player watching a token get hidden is told
	// it was deleted, since an event withheld from them can't tell them
	// to stop looking at it.
	it('removes a token on token.deleted', () => {
		const { client, socket } = roomWithTokens([goblin, { ...goblin, id: 't2' }]);

		socket.emit({ type: 'token.deleted', payload: { tokenId: 't1' } });

		expect(client.tokens.map((t) => t.id)).toEqual(['t2']);
	});

	// Unlike the eraser, this waits for the broadcast: a token is deleted
	// from a button, so there is no preview shape that would blink.
	it('sends token.delete and leaves the token until the server confirms', () => {
		const { client, socket } = roomWithTokens([goblin]);

		client.deleteToken('t1');

		expect(sentTypes(socket)).toEqual(['token.delete']);
		expect(JSON.parse(socket.sent[0]).payload).toEqual({ tokenId: 't1' });
		expect(client.tokens.map((t) => t.id)).toEqual(['t1']);

		socket.emit({ type: 'token.deleted', payload: { tokenId: 't1' } });
		expect(client.tokens).toHaveLength(0);
	});

	it('undoes a deletion by recreating the token under the same id', () => {
		const { client, socket } = roomWithTokens([goblin]);

		client.deleteToken('t1');
		socket.emit({ type: 'token.deleted', payload: { tokenId: 't1' } });
		expect(client.canUndo).toBe(true);

		client.undo();

		expect(sentTypes(socket)).toEqual(['token.delete', 'token.create']);
		// Every property goes back, because the server rebuilds the row
		// from this payload alone — a token that came back 1×1 in the wrong
		// square would look like a different token to everyone.
		expect(JSON.parse(socket.sent[1]).payload).toEqual({
			tokenId: 't1',
			sceneId: 's1',
			name: 'Goblin',
			imageAssetId: null,
			x: 3,
			y: 4,
			width: 2,
			height: 2,
			ownerParticipantId: null,
			visibility: 'visible'
		});

		// And redo deletes it again, once the recreation has landed.
		socket.emit({ type: 'token.created', payload: goblin });
		client.redo();
		expect(sentTypes(socket)).toEqual(['token.delete', 'token.create', 'token.delete']);
	});

	it('skips a deletion whose token is already back and undoes the next thing instead', () => {
		const { client, socket } = roomWithTokens([goblin]);

		client.deleteToken('t1');
		socket.emit({ type: 'token.deleted', payload: { tokenId: 't1' } });

		// Someone else put a token back under the same id — recreating it
		// would be refused, so undo passes over the entry rather than
		// failing the whole gesture.
		socket.emit({ type: 'token.created', payload: goblin });

		client.undo();

		expect(sentTypes(socket)).toEqual(['token.delete']);
		expect(client.canUndo).toBe(false);
	});

	it('sends every editable field on token.update, so a cleared image stays cleared', () => {
		const { client, socket } = roomWithTokens([{ ...goblin, imageAssetId: 'art' }]);

		client.updateToken('t1', {
			name: 'Hobgoblin',
			imageAssetId: null,
			width: 2,
			height: 2,
			visibility: 'hidden'
		});

		expect(JSON.parse(socket.sent[0])).toEqual({
			type: 'token.update',
			payload: {
				tokenId: 't1',
				name: 'Hobgoblin',
				imageAssetId: null,
				width: 2,
				height: 2,
				visibility: 'hidden'
			}
		});
		// Nothing changes locally: unlike a drawing there is no preview
		// shape to blink, so this waits for the broadcast like token.move.
		expect(client.tokens[0].name).toBe('Goblin');
	});

	it('records nothing for a token deletion that could not be sent', () => {
		const { client, socket } = roomWithTokens([goblin]);
		socket.readyState = 0; // connecting

		client.deleteToken('t1');
		expect(client.canUndo).toBe(false);
		expect(client.tokens).toHaveLength(1);
	});

	it('shares one history with drawings, so undo walks back through both', () => {
		const { client, socket } = roomWithTokens([goblin]);

		client.createDrawing('s1', 'line', [{ x: 0, y: 0 }], '#cc0000');
		client.deleteToken('t1');
		socket.emit({ type: 'token.deleted', payload: { tokenId: 't1' } });

		// Newest first: the token comes back before the stroke goes away.
		client.undo();
		expect(sentTypes(socket).at(-1)).toBe('token.create');

		client.undo();
		expect(sentTypes(socket).at(-1)).toBe('draw.delete');
		expect(client.canUndo).toBe(false);
	});

	it('adds a ping with a generated id and removes it after it expires', () => {
		vi.useFakeTimers();
		try {
			const { client, socket } = connectedClient();
			socket.emit({
				type: 'ping',
				payload: { sceneId: 'scene1', x: 10, y: 20, participantName: 'Bob' }
			});

			expect(client.pings).toHaveLength(1);
			expect(client.pings[0]).toMatchObject({
				sceneId: 'scene1',
				x: 10,
				y: 20,
				participantName: 'Bob'
			});
			expect(client.pings[0].id).toBeTruthy();

			// The marker has to outlast every pulse of the animation, or
			// it would vanish part-way through the sequence.
			vi.advanceTimersByTime(PING_LIFETIME_MS - 1);
			expect(client.pings).toHaveLength(1);

			vi.advanceTimersByTime(1);
			expect(client.pings).toHaveLength(0);
		} finally {
			vi.useRealTimers();
		}
	});

	it('sends ping with sceneId, x, and y', () => {
		const { client, socket } = connectedClient();
		client.sendPing('scene1', 5, 6);
		expect(JSON.parse(socket.sent[0])).toEqual({
			type: 'ping',
			payload: { sceneId: 'scene1', x: 5, y: 6 }
		});
	});

	// A ping is on screen for a few seconds now, so an impatient second
	// click would drop a marker over the spot the first is pointing at.
	it('ignores a second ping inside the cooldown', () => {
		vi.useFakeTimers();
		try {
			const { client, socket } = connectedClient();

			client.sendPing('scene1', 5, 6);
			client.sendPing('scene1', 7, 8);
			expect(socket.sent).toHaveLength(1);

			vi.advanceTimersByTime(PING_COOLDOWN_MS - 1);
			client.sendPing('scene1', 9, 10);
			expect(socket.sent).toHaveLength(1);

			vi.advanceTimersByTime(1);
			client.sendPing('scene1', 11, 12);
			expect(socket.sent).toHaveLength(2);
			expect(JSON.parse(socket.sent[1]).payload).toMatchObject({ x: 11, y: 12 });
		} finally {
			vi.useRealTimers();
		}
	});

	it('does not start the cooldown on a ping that could not be sent', () => {
		vi.useFakeTimers();
		try {
			const { client, socket } = connectedClient();
			socket.readyState = 0; // connecting

			client.sendPing('scene1', 5, 6);
			expect(socket.sent).toHaveLength(0);

			// Nothing went out, so the next attempt must not be swallowed
			// as though one just had.
			socket.readyState = 1;
			client.sendPing('scene1', 5, 6);
			expect(socket.sent).toHaveLength(1);
		} finally {
			vi.useRealTimers();
		}
	});

	describe('measuring', () => {
		function measuringClient() {
			const { client, socket } = connectedClient();
			socket.emit({
				type: 'state.sync',
				payload: {
					room: { slug: 'abc123', name: 'Room' },
					you: { participantId: 'p1', displayName: 'Alice', role: 'player' },
					scene: { id: 's1', gridSize: 70 }
				}
			});
			socket.sent.length = 0;
			return { client, socket };
		}

		function measurePayloads(socket: { sent: string[] }) {
			return socket.sent
				.map((s) => JSON.parse(s))
				.filter((e) => e.type === 'measure.update')
				.map((e) => e.payload);
		}

		it('shows your own measurement immediately and names you as its owner', () => {
			const { client } = measuringClient();
			client.updateMeasure('s1', { x: 0, y: 0 }, { x: 100, y: 0 });

			expect(client.measurements).toEqual([
				{
					participantId: 'p1',
					participantName: 'Alice',
					sceneId: 's1',
					// A measurement with no shape named is the plain distance
					// line the tool started as; the area templates share this
					// same path and differ only in the kind.
					kind: 'distance',
					from: { x: 0, y: 0 },
					to: { x: 100, y: 0 },
					widthFeet: undefined
				}
			]);
		});

		// Templates ride the measuring gesture rather than a channel of
		// their own, so the shape and the line's width have to survive the
		// throttle and reach the wire.
		it('sends the template shape and width on an area measurement', () => {
			const { client, socket } = measuringClient();
			client.updateMeasure('s1', { x: 0, y: 0 }, { x: 140, y: 0 }, 'line', 10);

			expect(client.measurements[0].kind).toBe('line');
			expect(JSON.parse(socket.sent.at(-1)!)).toEqual({
				type: 'measure.update',
				payload: {
					sceneId: 's1',
					kind: 'line',
					from: { x: 0, y: 0 },
					to: { x: 140, y: 0 },
					widthFeet: 10
				}
			});
		});

		it('keeps one measurement per participant as the drag moves', () => {
			const { client, socket } = measuringClient();
			client.updateMeasure('s1', { x: 0, y: 0 }, { x: 100, y: 0 });
			client.updateMeasure('s1', { x: 0, y: 0 }, { x: 200, y: 0 });
			socket.emit({
				type: 'measure.updated',
				payload: {
					participantId: 'p2',
					participantName: 'Bob',
					sceneId: 's1',
					from: { x: 0, y: 0 },
					to: { x: 70, y: 70 }
				}
			});

			expect(client.measurements).toHaveLength(2);
			expect(client.measurements[0].to).toEqual({ x: 200, y: 0 });
		});

		it('paces updates on the wire and still sends the position the drag ended on', () => {
			vi.useFakeTimers();
			try {
				const { client, socket } = measuringClient();

				// First move goes out at once; the rest are held.
				client.updateMeasure('s1', { x: 0, y: 0 }, { x: 10, y: 0 });
				client.updateMeasure('s1', { x: 0, y: 0 }, { x: 20, y: 0 });
				client.updateMeasure('s1', { x: 0, y: 0 }, { x: 30, y: 0 });
				expect(measurePayloads(socket)).toHaveLength(1);

				vi.advanceTimersByTime(MEASURE_SEND_INTERVAL_MS);
				const sent = measurePayloads(socket);
				expect(sent).toHaveLength(2);
				expect(sent[1].to).toEqual({ x: 30, y: 0 });

				// Nothing new arrived, so the throttle stops rather than
				// resending the same position forever.
				vi.advanceTimersByTime(MEASURE_SEND_INTERVAL_MS * 5);
				expect(measurePayloads(socket)).toHaveLength(2);
			} finally {
				vi.useRealTimers();
			}
		});

		it('drops an unsent position rather than letting it follow the end', () => {
			vi.useFakeTimers();
			try {
				const { client, socket } = measuringClient();
				client.updateMeasure('s1', { x: 0, y: 0 }, { x: 10, y: 0 });
				client.updateMeasure('s1', { x: 0, y: 0 }, { x: 20, y: 0 });
				client.endMeasure();

				vi.advanceTimersByTime(MEASURE_SEND_INTERVAL_MS * 5);
				expect(sentTypes(socket)).toEqual(['measure.update', 'measure.end']);
				expect(client.measurements).toEqual([]);
			} finally {
				vi.useRealTimers();
			}
		});

		it('ignores the echo of your own measurement', () => {
			const { client, socket } = measuringClient();
			client.updateMeasure('s1', { x: 0, y: 0 }, { x: 200, y: 0 });

			// A throttled update arriving late must not drag your own line
			// back to where the pointer used to be.
			socket.emit({
				type: 'measure.updated',
				payload: {
					participantId: 'p1',
					participantName: 'Alice',
					sceneId: 's1',
					from: { x: 0, y: 0 },
					to: { x: 10, y: 0 }
				}
			});

			expect(client.measurements).toHaveLength(1);
			expect(client.measurements[0].to).toEqual({ x: 200, y: 0 });
		});

		it('ignores a measurement made in another scene', () => {
			const { client, socket } = measuringClient();
			socket.emit({
				type: 'measure.updated',
				payload: {
					participantId: 'p2',
					participantName: 'Bob',
					sceneId: 'other-scene',
					from: { x: 0, y: 0 },
					to: { x: 70, y: 70 }
				}
			});

			expect(client.measurements).toEqual([]);
		});

		it('takes a measurement off the map when its owner ends it', () => {
			const { client, socket } = measuringClient();
			socket.emit({
				type: 'measure.updated',
				payload: {
					participantId: 'p2',
					participantName: 'Bob',
					sceneId: 's1',
					from: { x: 0, y: 0 },
					to: { x: 70, y: 70 }
				}
			});
			expect(client.measurements).toHaveLength(1);

			socket.emit({ type: 'measure.ended', payload: { participantId: 'p2' } });
			expect(client.measurements).toEqual([]);
		});

		it('clears measurements when the scene changes under them', () => {
			const { client, socket } = measuringClient();
			socket.emit({
				type: 'measure.updated',
				payload: {
					participantId: 'p2',
					participantName: 'Bob',
					sceneId: 's1',
					from: { x: 0, y: 0 },
					to: { x: 70, y: 70 }
				}
			});

			socket.emit({ type: 'scene.activated', payload: { scene: { id: 's2' } } });
			expect(client.measurements).toEqual([]);
		});
	});
});
