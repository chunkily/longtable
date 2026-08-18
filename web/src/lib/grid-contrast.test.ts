import { beforeEach, describe, expect, it } from 'vitest';
import {
	DEFAULT_HIGH_CONTRAST_GRID,
	loadHighContrastGrid,
	saveHighContrastGrid
} from './grid-contrast';

describe('high-contrast grid preference', () => {
	beforeEach(() => window.localStorage.clear());

	it('starts off, so a browser that never touches the control draws the faint grid', () => {
		expect(loadHighContrastGrid()).toBe(DEFAULT_HIGH_CONTRAST_GRID);
		expect(loadHighContrastGrid()).toBe(false);
	});

	it('round-trips the choice, which is the whole point of persisting it', () => {
		saveHighContrastGrid(true);
		expect(loadHighContrastGrid()).toBe(true);

		saveHighContrastGrid(false);
		expect(loadHighContrastGrid()).toBe(false);
	});

	// Turning it off removes the key rather than writing a second word for
	// "no", so the stored state has one spelling per meaning and a later
	// change of default can't be overridden by an old explicit "off".
	it('leaves nothing behind when switched off', () => {
		saveHighContrastGrid(true);
		saveHighContrastGrid(false);

		expect(window.localStorage.getItem('longtable:gridContrast')).toBeNull();
	});

	// The value is read as a word, not as "any non-empty string is true" —
	// which is what a bare boolean stored as text would degrade into the
	// first time something else wrote to the key.
	it('falls back to the default for a value it does not recognise', () => {
		window.localStorage.setItem('longtable:gridContrast', 'true');
		expect(loadHighContrastGrid()).toBe(false);

		window.localStorage.setItem('longtable:gridContrast', 'bold');
		expect(loadHighContrastGrid()).toBe(true);
	});
});
