import { describe, expect, it } from 'vitest';
import { guessAssetKind } from './asset-kind';

describe('guessAssetKind', () => {
	// The sizes real token art actually comes in. A plain "bigger than
	// 150px means map" rule — the obvious first idea — calls every one of
	// these a map, which is why squareness carries the decision instead.
	it('calls square art of an ordinary size a token', () => {
		expect(guessAssetKind(256, 256)).toBe('token');
		expect(guessAssetKind(512, 512)).toBe('token');
		expect(guessAssetKind(1024, 1024)).toBe('token');
		// Not perfectly square, but nothing anyone would call a map.
		expect(guessAssetKind(500, 460)).toBe('token');
	});

	it('calls anything much wider or taller than it is square a map', () => {
		expect(guessAssetKind(1400, 900)).toBe('map');
		expect(guessAssetKind(4000, 3000)).toBe('map');
		// Small but still nowhere near square.
		expect(guessAssetKind(600, 300)).toBe('map');
	});

	// Square maps exist; square art the size of a poster is likelier to be
	// one than a portrait that will be shown at 70px on the table.
	it('calls very large square art a map', () => {
		expect(guessAssetKind(2048, 2048)).toBe('map');
	});

	// Degenerate input is a shape nobody can read, and the guess only ever
	// prompts, so the harmless answer is the quiet one.
	it('falls back to token for dimensions it cannot read', () => {
		expect(guessAssetKind(0, 0)).toBe('token');
		expect(guessAssetKind(-1, 100)).toBe('token');
	});
});
