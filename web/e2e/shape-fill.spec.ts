import { expect, test } from './fixtures/table';
import { TOOLBAR_CLEARANCE_Y, mapGestureOrigin, selectTool } from './fixtures/room';
import { LAYER, inkAt, type Point } from './fixtures/map';
import type { Page } from '@playwright/test';

// A fill is only offered for the two kinds that enclose an area, and it
// has to survive the round trip — a shape that looks filled to whoever
// drew it and hollow to everyone else is worse than no fill at all.
//
// Konva has no DOM, so "is it filled" is a pixel read in the middle of
// the shape, where an outline puts nothing.

// Offsets from `mapGestureOrigin`, which already clears the floating
// toolbar. The probes below have to add that clearance back on: the
// gesture is in page coordinates, and inkAt reads the canvas's own
// buffer, which still starts at its true top-left corner.
const FROM: Point = { x: 120, y: 120 };
const TO: Point = { x: 320, y: 260 };

const onCanvas = (p: Point): Point => ({ x: p.x, y: p.y + TOOLBAR_CLEARANCE_Y });
const MIDDLE: Point = onCanvas({ x: (FROM.x + TO.x) / 2, y: (FROM.y + TO.y) / 2 });

async function dragShape(page: Page, origin: Point) {
	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
	await page.mouse.up();
}

/** Ink well inside the shape, where only a fill reaches. */
function interiorInk(page: Page) {
	return inkAt(page, LAYER.drawings, MIDDLE, 12);
}

test('the fill toggle only appears for the shapes that enclose an area', async ({ table }) => {
	const page = table.gm.page;
	const fill = page.getByRole('button', { name: 'Fill', exact: true });

	await selectTool(page, 'Rectangle');
	await expect(fill).toBeVisible();

	await selectTool(page, 'Ellipse');
	await expect(fill).toBeVisible();

	// A line and a freehand stroke enclose nothing, so the control goes
	// rather than sitting there doing nothing.
	await selectTool(page, 'Line');
	await expect(fill).toBeHidden();

	await selectTool(page, 'Freehand');
	await expect(fill).toBeHidden();
});

test('a rectangle is hollow by default and shaded once Fill is on', async ({ table }) => {
	const page = table.gm.page;
	const origin = await mapGestureOrigin(page);

	await selectTool(page, 'Rectangle');
	await dragShape(page, origin);
	// The outline is drawn out at the edges, so the middle stays empty.
	await expect.poll(() => inkAt(page, LAYER.drawings, onCanvas(FROM), 12)).toBeGreaterThan(0);
	expect(await interiorInk(page)).toBe(0);

	// Toggled without reselecting the tool, which is the case a prop read
	// in the wrong place would break: the handlers are bound in an effect,
	// and a value captured when they were bound would still be false here.
	await page.getByRole('button', { name: 'Fill', exact: true }).click();
	await dragShape(page, origin);
	await expect.poll(() => interiorInk(page)).toBeGreaterThan(0);
});

test('a filled ellipse arrives filled for everyone else too', async ({ table }) => {
	const player = await table.join();
	const gm = table.gm;
	const origin = await mapGestureOrigin(gm.page);

	await selectTool(gm.page, 'Ellipse');
	await gm.page.getByRole('button', { name: 'Fill', exact: true }).click();
	await dragShape(gm.page, origin);

	// The GM's own copy is optimistic; the player's is the one that came
	// off the wire and out of the database, which is the half that a
	// missing column or an unsent field would break.
	await expect.poll(() => interiorInk(gm.page)).toBeGreaterThan(0);
	await expect.poll(() => interiorInk(player.page)).toBeGreaterThan(0);
});

// The fill survives a reload, so it is stored rather than only broadcast.
test('a filled shape is still filled after a reload', async ({ table }) => {
	const page = table.gm.page;
	const origin = await mapGestureOrigin(page);

	await selectTool(page, 'Rectangle');
	await page.getByRole('button', { name: 'Fill', exact: true }).click();
	await dragShape(page, origin);
	await expect.poll(() => interiorInk(page)).toBeGreaterThan(0);

	await page.reload();
	await expect(page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => interiorInk(page)).toBeGreaterThan(0);
});
