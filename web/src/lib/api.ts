// Thin REST client for the Go backend. Requests are relative — in
// production the Go binary serves both the frontend and the API from
// the same origin; in dev, vite.config.ts proxies /api to a locally
// running `go run ./cmd/longtable`.

export interface Session {
	roomSlug: string;
	roomName: string;
	participantId: string;
	displayName: string;
	/** An IDENTITY_COLORS key, or '' for a seat that never chose one. */
	color: string;
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

/**
 * A failure the server described. Carries the status because *which*
 * failure it was changes what the UI should say: a 404 on a room is
 * worth telling someone about immediately, while a 500 or a dropped
 * connection is worth retrying past.
 */
export class ApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
	}
}

/**
 * Whether a rejection was the server saying "no such thing", as opposed
 * to anything else that can reject a fetch — a blip, a proxy, a server
 * that fell over. Written as a guard rather than left to callers because
 * a bare `err.status === 404` needs the `instanceof` first, and a caller
 * who forgets it gets `undefined === 404` and a silent false.
 */
export function isNotFound(err: unknown): boolean {
	return err instanceof ApiError && err.status === 404;
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
	const res = await fetch(path, {
		...init,
		headers: { 'Content-Type': 'application/json', ...init?.headers }
	});
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new ApiError(body.error ?? `request failed with status ${res.status}`, res.status);
	}
	// 204 means the request succeeded and said nothing — a DELETE that
	// worked. Parsing that as JSON throws on an empty body, which would
	// turn every successful delete into an error toast.
	if (res.status === 204) return undefined as T;
	return res.json();
}

// No colour here, or on gmLogin below: the GM's is a fixed black rather
// than one of the sixteen, so there is nothing to send. The server stores
// none for a GM seat either.
//
// `joinPassword` is optional and separate from `password` (the GM
// password, required): it's the same setting Manage room changes later,
// just set up front instead of in a second trip through that dialog.
export function createRoom(
	roomName: string,
	gmName: string,
	password: string,
	joinPassword?: string
): Promise<Session> {
	return apiFetch('/api/rooms', {
		method: 'POST',
		body: JSON.stringify({ roomName, gmName, password, joinPassword })
	});
}

/**
 * A chair at a table, as seen by a device that hasn't proved anything
 * about who it is yet. Carries no credential — taking a seat is what
 * gets you one. See ADR-0008.
 */
export interface Seat {
	participantId: string;
	displayName: string;
	/**
	 * An IDENTITY_COLORS key, or '' for a seat from before colours. Here
	 * so the picker can show what is taken *before* anyone joins, which
	 * is the only moment that is any use.
	 */
	color: string;
	role: 'gm' | 'player';
	/** Whether anyone has a socket open on this seat right now. */
	connected: boolean;
}

/**
 * The room's seats, for the pre-join screen. Unauthenticated, because
 * it answers before you have a session — reaching it means already
 * holding the room's link.
 */
export function listSeats(
	slug: string
): Promise<{ roomName: string; seats: Seat[]; joinPasswordRequired: boolean }> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/seats`);
}

/**
 * Joins a room, three ways depending on what's passed:
 *
 * - `sessionToken` — a browser that still remembers this room resumes
 *   without touching the seat picker.
 * - `participantId` — take a seat someone already sat in, and get this
 *   device's own session on it.
 * - `displayName` — "I'm new here": a fresh seat, exactly what joining
 *   used to be.
 *
 * `joinPassword` is checked against the room's join password when one
 * is set (`joinPasswordRequired` on `listSeats`), for either of the
 * first two — never for a resuming `sessionToken`, which already proved
 * its seat.
 */
export function joinRoom(
	slug: string,
	opts: {
		displayName?: string;
		color?: string;
		sessionToken?: string;
		participantId?: string;
		joinPassword?: string;
	}
): Promise<Session> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/join`, {
		method: 'POST',
		body: JSON.stringify(opts)
	});
}

export function addSeat(
	slug: string,
	sessionToken: string,
	displayName: string,
	color: string
): Promise<Seat> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/seats`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${sessionToken}` },
		body: JSON.stringify({ displayName, color })
	});
}

export function removeSeat(
	slug: string,
	sessionToken: string,
	participantId: string
): Promise<void> {
	return apiFetch(
		`/api/rooms/${encodeURIComponent(slug)}/seats/${encodeURIComponent(participantId)}`,
		{ method: 'DELETE', headers: { Authorization: `Bearer ${sessionToken}` } }
	);
}

/**
 * Deletes the room and everything in it. GM-only, and there is no undo —
 * the images survive, because they belong to every room that uploaded
 * the same bytes.
 *
 * Anyone still connected is told over the socket (`room.deleted`) rather
 * than by this call, which only ever answers the person who asked.
 */
export function deleteRoom(slug: string, sessionToken: string): Promise<void> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}`, {
		method: 'DELETE',
		headers: { Authorization: `Bearer ${sessionToken}` }
	});
}

/**
 * Sets the password that signs somebody into this room's GM seat.
 *
 * Takes no current password — the session token is the proof, the same
 * as adding or removing a seat. Signs nobody out, including the device
 * that calls it.
 */
export function setGMPassword(slug: string, sessionToken: string, password: string): Promise<void> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/gm-password`, {
		method: 'PUT',
		headers: { Authorization: `Bearer ${sessionToken}` },
		body: JSON.stringify({ password })
	});
}

/**
 * Sets, changes or clears the password a Player needs to join this room.
 * Independent of the GM password — this one gates joining, not the GM
 * seat. An empty string clears it back to "anyone with the link may
 * join", which is this setting's own valid state rather than a typo.
 */
export function setJoinPassword(
	slug: string,
	sessionToken: string,
	password: string
): Promise<void> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/join-password`, {
		method: 'PUT',
		headers: { Authorization: `Bearer ${sessionToken}` },
		body: JSON.stringify({ password })
	});
}

/**
 * Checks a join password without joining anything, unauthenticated like
 * `listSeats` — it answers before there's a session to authenticate.
 * Rejects (an `ApiError`) on a wrong password; resolves on a right one,
 * or when the room has none set at all.
 *
 * What lets the pre-join screen refuse a wrong password the moment it's
 * typed, rather than after a Player has also picked a seat, a colour and
 * a name — all of which `joinRoom` would otherwise have to discard.
 */
export function checkJoinPassword(slug: string, password: string): Promise<void> {
	return apiFetch(`/api/rooms/${encodeURIComponent(slug)}/join-password/check`, {
		method: 'POST',
		body: JSON.stringify({ password })
	});
}

/**
 * Signs this device out of the room, leaving the seat and any other
 * device on it alone. Best-effort: the browser is dropping its own
 * session either way, so a server that can't be reached must not keep
 * someone in a room they've left.
 */
export async function endSession(slug: string, sessionToken: string): Promise<void> {
	try {
		await fetch(`/api/rooms/${encodeURIComponent(slug)}/session`, {
			method: 'DELETE',
			headers: { Authorization: `Bearer ${sessionToken}` }
		});
	} catch {
		// Ignored on purpose — see above.
	}
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
		throw new ApiError(body.error ?? `upload failed with status ${res.status}`, res.status);
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
		throw new ApiError(body.error ?? `removal failed with status ${res.status}`, res.status);
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

/**
 * The Host's banner message, or '' when the server was started without
 * one. Unauthenticated, because it is shown on every page including the
 * home page of a browser that has never joined anything.
 *
 * Never throws: a server that can't answer this is a server whose banner
 * nobody needs, and the page behind it works either way.
 */
export async function fetchNotice(): Promise<string> {
	try {
		const res = await fetch('/api/notice');
		if (!res.ok) return '';
		const body = (await res.json()) as { notice?: string };
		return body.notice ?? '';
	} catch {
		return '';
	}
}
