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

/**
 * What a room keeps a picture for. It's the room's opinion rather than a
 * property of the file, so the same image can be a token here and a map
 * next door — and so it can be corrected later, unlike the pixels.
 */
export type AssetKind = 'token' | 'map';

export interface Asset {
	id: string;
	/**
	 * What this room calls the asset — what the library and the pickers
	 * show, and half of what search matches. Defaults to the filename
	 * minus its extension.
	 */
	name: string;
	filename: string;
	mimeType: string;
	byteSize: number;
	/** Free-text credit or licence for this room's copy, '' when none was given. */
	attribution: string;
	/** Which half of the library this sits in. */
	kind: AssetKind;
	/**
	 * Pixels per grid square, measured when the map was aligned on the
	 * assets page, so a scene made from it can default to the right
	 * number. Null for anything nobody aligned — every token, and any map
	 * added before the assets page existed.
	 */
	gridSize: number | null;
	/**
	 * Set when an animated upload was accepted as a still image, so the
	 * uploader can be told rather than left wondering why their goblin
	 * stopped moving. Absent on the ordinary case.
	 */
	flattened?: boolean;
}

/** Everything the assets page collects about a file before sending it. */
export interface AssetDetails {
	name: string;
	attribution: string;
	kind: AssetKind;
	/** Measured pixels per square, for a map that was aligned. */
	gridSize: number | null;
	/**
	 * Pixels to add to the left and top before the image is stored;
	 * negative crops instead. This is how grid alignment is applied — the
	 * server bakes it into the pixels during the re-encode every upload
	 * already goes through, so nothing downstream ever has to know an
	 * offset existed.
	 */
	gridOffsetX: number;
	gridOffsetY: number;
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
 *
 * Every detail travels with the file because nothing is stored until it
 * does: there's no draft library entry to go back and fill in.
 */
export async function uploadAsset(
	slug: string,
	sessionToken: string,
	file: File,
	details: Partial<AssetDetails> = {}
): Promise<Asset> {
	const form = new FormData();
	form.append('file', file);
	if (details.name?.trim()) form.append('name', details.name.trim());
	if (details.attribution?.trim()) form.append('attribution', details.attribution.trim());
	// Left off rather than defaulted client-side when absent: the server
	// reads a missing kind as "not supplied" and keeps whatever this room
	// already decided, which is what makes re-adding a map safe.
	if (details.kind) form.append('kind', details.kind);
	if (details.gridSize) form.append('gridSize', String(details.gridSize));
	// Sent only when non-zero: an offset of 0 is the overwhelmingly common
	// case, and leaving the field off keeps the server on the path where
	// the image is passed through untouched.
	if (details.gridOffsetX) form.append('gridOffsetX', String(details.gridOffsetX));
	if (details.gridOffsetY) form.append('gridOffsetY', String(details.gridOffsetY));

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

/**
 * Renames, re-credits and reclassifies a room's copy of an asset. The
 * image and its measured grid can't be edited: both live in the stored
 * pixels, and changing those would make a different asset. The kind
 * isn't in the pixels, so that one moves.
 */
export function updateAsset(
	slug: string,
	sessionToken: string,
	assetId: string,
	details: { name: string; attribution: string; kind: AssetKind }
): Promise<Asset> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/assets/${encodeURIComponent(assetId)}`, {
		method: 'PATCH',
		headers: { Authorization: `Bearer ${sessionToken}` },
		body: JSON.stringify(details)
	});
}

/**
 * Takes an asset off this room's shelf.
 *
 * The picture itself isn't deleted — it's content-addressed and shared,
 * so another room that added the same file keeps it, and adding it here
 * again brings it back. Nor does this reach onto the table: a scene or
 * token already using the image goes on using it.
 */
export async function removeAsset(
	slug: string,
	sessionToken: string,
	assetId: string
): Promise<void> {
	const res = await fetch(
		`/api/rooms/${encodeURIComponent(slug)}/assets/${encodeURIComponent(assetId)}`,
		{ method: 'DELETE', headers: { Authorization: `Bearer ${sessionToken}` } }
	);
	// 204 and an empty body, so this can't go through apiFetch — that one
	// parses JSON out of every response.
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new ApiError(body.error ?? `removal failed with status ${res.status}`);
	}
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
