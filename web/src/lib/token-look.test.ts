import { describe, expect, it } from 'vitest';
import { IDENTITY_COLORS } from './identity-color';
import { INK, betterInk, discLook, hexRgb, idHue, initials } from './token-look';
import { GAME_ICONS } from './game-icons.generated';
import { TOKEN_ICONS, needsGameIconsCredit, searchTokenIcons, tokenIcon } from './token-icons';

describe('the palette inks', () => {
	// Stored rather than computed, so a retuned shade is a decision about
	// its ink as well — and this is what catches the ink going stale.
	it.each(IDENTITY_COLORS)('$label carries the more readable ink', (c) => {
		expect(c.ink).toBe(betterInk(hexRgb(c.hex)));
	});
});

describe('discLook', () => {
	it('uses a chosen colour and its ink', () => {
		expect(discLook({ id: 't1', color: 'yellow' })).toEqual({ fill: '#eab308', ink: INK.dark });
		expect(discLook({ id: 't1', color: 'indigo' })).toEqual({ fill: '#4f46e5', ink: INK.light });
	});

	// A token from before colours, or one nobody picked for, keeps the
	// colour its id has always hashed to — nothing on the map changes.
	it('falls back to the colour the id picks', () => {
		const look = discLook({ id: 'goblin-1' });
		expect(look).toEqual(discLook({ id: 'goblin-1', color: '' }));
		expect(look.fill).toMatch(/^rgb\(/);
		// A key nobody knows draws like no key at all, rather than as nothing.
		expect(discLook({ id: 'goblin-1', color: 'chartreuse' })).toEqual(look);
	});

	// The old disc was always white ink, which disappeared on the yellow
	// half of the wheel; an id that hashes to yellow now gets dark ink.
	it('picks dark ink for a light hashed colour', () => {
		const yellowish = Array.from({ length: 200 }, (_, n) => `id-${n}`).find((id) => {
			const h = idHue(id);
			return h > 45 && h < 75;
		});
		expect(yellowish).toBeDefined();
		expect(discLook({ id: yellowish! }).ink).toBe(INK.dark);
	});
});

describe('initials', () => {
	it('is the first two letters, upper case', () => {
		expect(initials('goblin')).toBe('GO');
		expect(initials('X')).toBe('X');
	});
});

describe('token icons', () => {
	it('has no duplicate keys, and every key is namespaced', () => {
		const keys = TOKEN_ICONS.map((i) => i.key);
		expect(new Set(keys).size).toBe(keys.length);
		expect(keys.every((k) => /^(lucide|game-icons):/.test(k))).toBe(true);
	});

	// Each pool draws its own way — a component or vendored SVG text — and
	// an icon with neither would be a blank square in the picker.
	it('gives every icon exactly one drawing', () => {
		for (const i of TOKEN_ICONS) expect(!!i.icon !== !!i.svg, i.key).toBe(true);
	});

	it('credits game-icons.net only when one of its icons is in the list', () => {
		const lucide = TOKEN_ICONS.filter((i) => i.key.startsWith('lucide:'));
		expect(needsGameIconsCredit(lucide)).toBe(false);
		expect(needsGameIconsCredit(TOKEN_ICONS)).toBe(GAME_ICONS.length > 0);
	});

	it('looks a key up, and gives nothing for none or a retired one', () => {
		expect(tokenIcon('lucide:skull')?.label).toBe('Skull');
		expect(tokenIcon('')).toBeUndefined();
		expect(tokenIcon('lucide:dragon')).toBeUndefined();
	});

	// "undead" isn't anyone's label; it's what a GM types looking for one.
	it('searches by what an icon is for, not only its name', () => {
		const undead = searchTokenIcons('undead').map((i) => i.label);
		expect(undead).toEqual(expect.arrayContaining(['Skull', 'Bone', 'Ghost']));
		expect(searchTokenIcons('wolf').map((i) => i.label)).toEqual(
			expect.arrayContaining(['Dog', 'Paw print'])
		);
		expect(searchTokenIcons('').length).toBe(TOKEN_ICONS.length);
	});

	// Word starts only, so a short search doesn't drag in every word that
	// merely contains it.
	it('matches the start of a word, not the middle', () => {
		expect(searchTokenIcons('ear').map((i) => i.label)).not.toContain('Paw print');
	});
});
