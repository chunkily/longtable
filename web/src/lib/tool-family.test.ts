import { describe, expect, it } from 'vitest';
import {
	DRAW_TOOLS,
	FOG_TOOLS,
	MEASURE_TOOLS,
	familyHasStrip,
	familyOf,
	toolForFamily,
	type Tool
} from './tool-family';

describe('familyOf', () => {
	it('puts the eraser in the draw family, not one of its own', () => {
		expect(familyOf('eraser')).toBe('draw');
	});

	it('puts every area template in the measure family', () => {
		for (const tool of ['template-circle', 'template-cone', 'template-line', 'template-cube']) {
			expect(familyOf(tool as Tool)).toBe('measure');
		}
	});

	it('reads the no-tool state as the hand rather than as no family', () => {
		expect(familyOf('none')).toBe('hand');
	});

	it('assigns every tool in the union exactly one family', () => {
		const all: Tool[] = ['none', 'ping', ...DRAW_TOOLS, ...MEASURE_TOOLS, ...FOG_TOOLS];
		for (const tool of all) {
			expect(familyOf(tool)).toBeDefined();
		}
		// Nothing lands in the hand family by accident — only 'none' does,
		// which is what makes "no tool" and "the hand tool" the same state
		// rather than two that can disagree.
		expect(all.filter((t) => familyOf(t) === 'hand')).toEqual(['none']);
	});
});

describe('toolForFamily', () => {
	it('returns to the variant that family was last left on', () => {
		expect(toolForFamily('draw', 'ellipse')).toBe('ellipse');
		expect(toolForFamily('measure', 'template-cone')).toBe('template-cone');
	});

	// A remembered tool from another family would otherwise select the
	// wrong strip — picking Draw and landing on the ruler.
	it('ignores a remembered tool belonging to a different family', () => {
		expect(toolForFamily('draw', 'measure')).toBe('freehand');
		expect(toolForFamily('measure', 'eraser')).toBe('measure');
		expect(toolForFamily('fog', 'freehand')).toBe('fog-reveal');
	});

	it('falls back to a sensible default with nothing remembered', () => {
		expect(toolForFamily('draw')).toBe('freehand');
		expect(toolForFamily('measure')).toBe('measure');
		expect(toolForFamily('fog')).toBe('fog-reveal');
	});

	it('maps the option-less families onto their single tool', () => {
		expect(toolForFamily('hand', 'ellipse')).toBe('none');
		expect(toolForFamily('ping', 'ellipse')).toBe('ping');
	});

	// Round-tripping is what keeps the tool row's highlight honest: the
	// family you pick is the family the resulting tool reports.
	it('round-trips through familyOf for every family', () => {
		for (const family of ['hand', 'draw', 'measure', 'fog', 'ping'] as const) {
			expect(familyOf(toolForFamily(family))).toBe(family);
		}
	});
});

describe('familyHasStrip', () => {
	it('gives hand and ping no strip, since neither has options', () => {
		expect(familyHasStrip('hand')).toBe(false);
		expect(familyHasStrip('ping')).toBe(false);
	});

	it('gives the three families with variants a strip', () => {
		expect(familyHasStrip('draw')).toBe(true);
		expect(familyHasStrip('measure')).toBe(true);
		expect(familyHasStrip('fog')).toBe(true);
	});
});
