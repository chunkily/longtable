import { describe, expect, it, vi } from 'vitest';
import { RoomClient } from './room.svelte';

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
		client.deleteDrawing('d1');
		expect(JSON.parse(socket.sent[0])).toEqual({
			type: 'draw.delete',
			payload: { drawingId: 'd1' }
		});
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
				drawings: [{ id: 'd3', sceneId: 's2', kind: 'circle', points: [], color: '#008000' }]
			}
		});
		expect(client.drawings.map((d) => d.id)).toEqual(['d3']);
	});

	it('sends draw.create with sceneId, kind, points, and color', () => {
		const { client, socket } = connectedClient();
		const points = [
			{ x: 1, y: 2 },
			{ x: 3, y: 4 }
		];
		client.createDrawing('scene1', 'line', points, '#cc0000');
		expect(JSON.parse(socket.sent[0])).toEqual({
			type: 'draw.create',
			payload: { sceneId: 'scene1', kind: 'line', points, color: '#cc0000' }
		});
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

			vi.advanceTimersByTime(1500);
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
});
