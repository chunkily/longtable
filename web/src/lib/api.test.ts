import { afterEach, describe, expect, it, vi } from 'vitest';
import { assetUrl, createRoom, listRoomAssets } from './api';

function jsonResponse(status: number, body: unknown) {
	return {
		ok: status >= 200 && status < 300,
		status,
		json: async () => body
	} as Response;
}

describe('apiFetch (via listRoomAssets/createRoom)', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('resolves with the parsed JSON body on success', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(200, [{ id: 'abc', name: 'Goblin' }]))
		);

		const assets = await listRoomAssets('7wdbtb', 'token');
		expect(assets).toEqual([{ id: 'abc', name: 'Goblin' }]);
	});

	it('throws with the server-provided error message on a non-ok response', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue(jsonResponse(400, { error: 'roomName is required' }))
		);

		await expect(createRoom('', 'gm', 'violet', 'password')).rejects.toThrow(
			'roomName is required'
		);
	});

	it('falls back to a generic message when the error body is not JSON', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok: false,
				status: 500,
				json: async () => {
					throw new Error('not json');
				}
			} as unknown as Response)
		);

		await expect(listRoomAssets('7wdbtb', 'token')).rejects.toThrow(
			'request failed with status 500'
		);
	});
});

describe('assetUrl', () => {
	it('builds a URL-encoded path to the asset endpoint', () => {
		expect(assetUrl('abc 123')).toBe('/api/assets/abc%20123');
	});
});
