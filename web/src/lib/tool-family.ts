// Which map tool is active, and which family of tools it belongs to.
//
// The tools themselves are a flat union — the canvas only ever asks
// "what am I doing with this drag?" and doesn't care how the toolbar is
// arranged. The families are a UI grouping laid over that union so the
// toolbar can show five icons instead of eleven, with the active
// family's variants on a strip below it.
//
// The family is *derived* from the active tool rather than stored
// alongside it. Two pieces of state would let them disagree — a strip
// showing draw's shapes while the canvas is measuring — and there is no
// question the pair could answer that the tool can't answer alone.

import type { DrawingKind } from './room.svelte';

// 'none' is plain pan/token-drag mode — the hand. Every other tool takes
// over the stage's pointer handling exclusively, since they all
// interpret a left-drag differently.
export type Tool =
	| 'none'
	| 'fog-reveal'
	| 'fog-hide'
	| DrawingKind
	| 'ping'
	| 'eraser'
	| 'measure'
	| 'template-circle'
	| 'template-cone'
	| 'template-line'
	| 'template-cube';

export type ToolFamily = 'hand' | 'draw' | 'measure' | 'fog' | 'ping';

// The eraser sits in the draw family deliberately: it's the same gesture
// on the same objects, and it was only ever a top-level button because
// the toolbar was flat.
export const DRAW_TOOLS = ['freehand', 'line', 'rect', 'ellipse', 'eraser'] as const;

// The measuring tool and the four area templates share one family
// because they share one gesture — drag from an origin, read a size off
// the result. This is the busiest strip, and the one that decides how
// wide the strip has to be.
export const MEASURE_TOOLS = [
	'measure',
	'template-circle',
	'template-cone',
	'template-line',
	'template-cube'
] as const;

export const FOG_TOOLS = ['fog-reveal', 'fog-hide'] as const;

/**
 * The family a tool belongs to. Total over the union: every tool has
 * exactly one home, and 'none' is the hand rather than an absence, so
 * there is always a family to highlight.
 */
export function familyOf(tool: Tool): ToolFamily {
	if ((DRAW_TOOLS as readonly string[]).includes(tool)) return 'draw';
	if ((MEASURE_TOOLS as readonly string[]).includes(tool)) return 'measure';
	if ((FOG_TOOLS as readonly string[]).includes(tool)) return 'fog';
	if (tool === 'ping') return 'ping';
	return 'hand';
}

/**
 * The tool a family selects when you pick it off the tool row, given
 * whatever that family was last left on. Picking Draw should put back
 * the shape you were drawing with rather than resetting to freehand
 * every time — the strip is a memory of what you were doing, not a
 * fresh start.
 *
 * `remembered` is ignored when it belongs to another family, so a caller
 * that hasn't tracked one yet (or has one left over from a family
 * switch) still gets a sensible tool rather than the wrong strip.
 */
export function toolForFamily(family: ToolFamily, remembered?: Tool): Tool {
	if (family === 'hand') return 'none';
	if (family === 'ping') return 'ping';
	if (remembered && familyOf(remembered) === family) return remembered;
	if (family === 'draw') return 'freehand';
	if (family === 'measure') return 'measure';
	return 'fog-reveal';
}

/**
 * Whether a family has a contextual strip at all. Hand and ping have no
 * options, so they show nothing rather than an empty bar — an empty
 * strip still costs a row of screen and still covers the map.
 */
export function familyHasStrip(family: ToolFamily): boolean {
	return family === 'draw' || family === 'measure' || family === 'fog';
}
