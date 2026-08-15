import { expect, test } from './fixtures/table';
import { TOOLBAR_CLEARANCE_Y, mapGestureOrigin, selectTool } from './fixtures/room';
import { LAYER, inkAt, type Point } from './fixtures/map';
import type { Page } from '@playwright/test';

// The width a drawing is made at has to reach everyone else and survive a
// reload, the same as its colour and its fill — a line that is thick for
// whoever drew it and hairline for the rest of the table is worse than
// one fixed width for all.
//
// Konva has no DOM, so "how thick is it" is a pixel count: a horizontal
// line is dragged out, and the ink is counted in a box that a thin stroke
// leaves mostly empty and a thick one nearly fills.

// Offsets from `mapGestureOrigin`, which already clears the floating
// toolbar. The probe adds that clearance back on, because the gesture is
// in page coordinates while inkAt reads the canvas's own buffer from its
// true top-left corner. A fresh scene sits at the identity transform, so
// these are world pixels too — the units a stroke's width is in.
const FROM: Point = { x: 120, y: 200 };
const TO: Point = { x: 320, y: 200 };
const MIDDLE: Point = { x: (FROM.x + TO.x) / 2, y: FROM.y + TOOLBAR_CLEARANCE_Y };

// Half the widest choice, so the box takes a Thick stroke in whole and
// leaves a Thin one ringed by empty pixels. Counting ink rather than
// testing one pixel keeps this off antialiasing: the edge of any stroke
// is partial coverage, and which side of a probe point it falls on moves
// with the device pixel ratio.
const PROBE_RADIUS = 8;

function strokeInk(page: Page) {
	return inkAt(page, LAYER.drawings, MIDDLE, PROBE_RADIUS);
}

/** The strip's one width button, which names the width it is on. */
function widthTrigger(page: Page) {
	return page.getByRole('button', { name: 'Stroke width:' });
}

function widthOption(page: Page, label: string) {
	return page.getByRole('button', { name: `${label} stroke`, exact: true });
}

/**
 * Picks a width through the popup, and waits for the popup to be gone.
 *
 * The panel is portalled over the map, so a gesture issued while it is
 * still closing lands on the panel rather than the canvas — silently,
 * since raw mouse coordinates make no actionability checks.
 */
async function pickWidth(page: Page, label: string) {
	await widthTrigger(page).click();
	await widthOption(page, label).click();
	await expect(widthOption(page, label)).toBeHidden();
}

async function dragLine(page: Page, origin: Point) {
	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
	await page.mouse.up();
}

test('the width picker is offered to every shape but the eraser', async ({ table }) => {
	const page = table.gm.page;

	await selectTool(page, 'Freehand');
	await expect(widthTrigger(page)).toBeVisible();

	await selectTool(page, 'Rectangle');
	await expect(widthTrigger(page)).toBeVisible();

	// The eraser takes whole strokes rather than making one, so a width
	// there would sit inert.
	await selectTool(page, 'Erase');
	await expect(widthTrigger(page)).toBeHidden();
});

// The strip carries the current width without being opened — that is
// what makes one button an acceptable trade for three.
test('the width button names the width it is on, and the popup ticks it', async ({ table }) => {
	const page = table.gm.page;

	await selectTool(page, 'Line');
	await expect(widthTrigger(page)).toHaveAccessibleName('Stroke width: Thin');

	await pickWidth(page, 'Medium');
	await expect(widthTrigger(page)).toHaveAccessibleName('Stroke width: Medium');

	await widthTrigger(page).click();
	await expect(widthOption(page, 'Medium')).toHaveAttribute('aria-pressed', 'true');

	// Escape rather than a click on an option, so both ways out are
	// exercised between this test and pickWidth. Closing puts focus back
	// on the button it was opened from — the half of the popover that a
	// hand-rolled panel is most likely to be missing.
	await page.keyboard.press('Escape');
	await expect(widthOption(page, 'Medium')).toBeHidden();
	await expect(widthTrigger(page)).toBeFocused();
});

// The one case the desktop viewport can't see: below lg the strip docks
// into the sheet's horizontally scrolling bar, close enough to the
// bottom edge that the panel has to open upward, and inside a box whose
// vertical axis is `auto`.
//
// It asserts the panel is reachable there, not which mechanism makes it
// reachable — disabling the popover's portal still passes this, because
// nothing between the strip and the sheet is positioned, so the panel's
// containing block is outside the scrolling box either way. Anything
// that changes (a `relative` wrapper added to the strip, most likely) is
// caught here and nowhere else.
test('the popup opens clear of the strip on a phone', async ({ table }) => {
	const page = table.gm.page;
	await page.setViewportSize({ width: 375, height: 812 });

	// The desktop copy of the strip is display:none at this width, so
	// these locators resolve to the sheet's copy without ambiguity.
	await selectTool(page, 'Line');
	await widthTrigger(page).click();
	await expect(widthOption(page, 'Thick')).toBeInViewport();

	// Clicking is the real check: Playwright verifies the click lands on
	// the element it aimed at, so a panel cut off by the scroller fails
	// here rather than quietly picking nothing.
	await widthOption(page, 'Thick').click();
	await expect(widthTrigger(page)).toHaveAccessibleName('Stroke width: Thick');
});

test('a line drawn on Thick lays down more ink than one on Thin', async ({ table }) => {
	const page = table.gm.page;
	const origin = await mapGestureOrigin(page);

	await selectTool(page, 'Line');
	await dragLine(page, origin);
	await expect.poll(() => strokeInk(page)).toBeGreaterThan(0);
	const thin = await strokeInk(page);

	// Picked without reselecting the tool, which is the case a value read
	// where the handlers are bound rather than where they fire would
	// break: they were bound while Thin was still the choice.
	await pickWidth(page, 'Thick');
	await dragLine(page, origin);
	// The thick line covers the thin one exactly, so what is counted here
	// is the thick stroke alone — nothing has to clear the map between the
	// two drags.
	await expect.poll(() => strokeInk(page)).toBeGreaterThan(thin);
});

test('a thick stroke arrives thick for everyone else, and after a reload', async ({ table }) => {
	const player = await table.join();
	const gm = table.gm;
	const origin = await mapGestureOrigin(gm.page);

	await selectTool(gm.page, 'Line');
	await pickWidth(gm.page, 'Thick');
	await dragLine(gm.page, origin);

	// The GM's copy is optimistic; the player's came off the wire, and the
	// one after the reload came out of the database. Those are the two
	// halves that a width only ever applied locally would fail on.
	await expect.poll(() => strokeInk(gm.page)).toBeGreaterThan(0);
	const drawn = await strokeInk(gm.page);

	// Compared with a margin rather than exactly: both are the same stroke
	// at the same zoom, and the tolerance is for antialiasing along the
	// two long edges, not for a different width arriving.
	await expect.poll(() => strokeInk(player.page)).toBeGreaterThan(drawn * 0.9);

	await player.page.reload();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => strokeInk(player.page)).toBeGreaterThan(drawn * 0.9);
});
