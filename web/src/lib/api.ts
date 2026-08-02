// Thin REST client for the Go backend. Requests are relative — in
// production the Go binary serves both the frontend and the API from
// the same origin; in dev, vite.config.ts proxies /api to a locally
// running `go run ./cmd/longtable`.

export interface RoomSummary {
	slug: string;
	name: string;
	createdAt: string;
}

export interface Session {
	roomSlug: string;
	roomName: string;
	participantId: string;
	displayName: string;
	role: 'gm' | 'player';
	sessionToken: string;
}

export interface Asset {
	id: string;
	filename: string;
	mimeType: string;
	byteSize: number;
	/** Free-text credit or licence for this room's copy, '' when none was given. */
	attribution: string;
	/**
	 * Set when an animated upload was accepted as a still image, so the
	 * uploader can be told rather than left wondering why their goblin
	 * stopped moving. Absent on the ordinary case.
	 */
	flattened?: boolean;
}

class ApiError extends Error {}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(path, {
		...init,
		headers: { 'Content-Type': 'application/json', ...init?.headers }
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new ApiError(body.error ?? `request failed with status ${res.status}`);
	}
	return res.json();
}

export function listRooms(): Promise<RoomSummary[]> {
	return apiFetch('/api/rooms');
}

export function createRoom(roomName: string, gmName: string, password: string): Promise<Session> {
	return apiFetch('/api/rooms', {
		method: 'POST',
		body: JSON.stringify({ roomName, gmName, password })
	});
}

export function joinRoom(
	slug: string,
	opts: { displayName?: string; sessionToken?: string }
): Promise<Session> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/join`, {
		method: 'POST',
		body: JSON.stringify(opts)
	});
}

export function gmLogin(slug: string, displayName: string, password: string): Promise<Session> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/gm-login`, {
		method: 'POST',
		body: JSON.stringify({ displayName, password })
	});
}

/**
 * Uploads an image and adds it to the room's library. The server decodes
 * and re-encodes it to WebP, so what comes back is not the file that went
 * in — `filename` is rewritten to match, and the returned id is shared
 * with any room that already had these exact pixels.
 */
export async function uploadAsset(
	slug: string,
	sessionToken: string,
	file: File,
	attribution = ''
): Promise<Asset> {
	const form = new FormData();
	form.append('file', file);
	if (attribution.trim()) form.append('attribution', attribution.trim());

	const res = await fetch(`/api/rooms/${encodeURIComponent(slug)}/assets`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${sessionToken}` },
		body: form
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new ApiError(body.error ?? `upload failed with status ${res.status}`);
	}
	return res.json();
}

/** The room's asset library, newest first. Requires a session for that room. */
export function listRoomAssets(slug: string, sessionToken: string): Promise<Asset[]> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/assets`, {
		headers: { Authorization: `Bearer ${sessionToken}` }
	});
}

/**
 * Whether a session is still good for a room, for the reconnect loop.
 *
 * Three answers, not two. A refused WebSocket upgrade reaches the
 * browser as a bare `onclose` with no status, so the socket alone can't
 * separate "the server is restarting, keep trying" from "this session is
 * gone, send them back to the join form" — and `unreachable` is the
 * third case, where the probe itself couldn't get an answer and the only
 * safe reading is to keep trying.
 */
export async function checkSession(
	slug: string,
	sessionToken: string
): Promise<'ok' | 'invalid' | 'unreachable'> {
	try {
		const res = await fetch(`/api/rooms/${encodeURIComponent(slug)}/session`, {
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
		if (res.ok) return 'ok';
		// 401 for a token the room doesn't know, 404 for a room that isn't
		// there any more. Neither gets better by waiting. Anything else is
		// the server having a bad time, which might.
		if (res.status === 401 || res.status === 404) return 'invalid';
		return 'unreachable';
	} catch {
		return 'unreachable';
	}
}

export function assetUrl(assetId: string): string {
	return `/api/assets/${encodeURIComponent(assetId)}`;
}
