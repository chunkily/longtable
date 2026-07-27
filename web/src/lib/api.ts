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

export async function uploadAsset(slug: string, sessionToken: string, file: File): Promise<Asset> {
	const form = new FormData();
	form.append('file', file);

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

export function assetUrl(assetId: string): string {
	return `/api/assets/${encodeURIComponent(assetId)}`;
}
