import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import {
	GRID,
	LAYER,
	canvasBox,
	createToken,
	detailsPanel,
	dragToken,
	inkAt,
	layerInk,
	selectToken,
	type Point
} from './fixtures/map';

// Selecting a token is the one piece of room UI that is deliberately
// *not* shared: it never goes on the wire, so proving it stays on one
// client needs two browsers. The highlight itself is a rotating Konva
// ring with no DOM behind it, so it has to be read off the canvas.

// The selection layer is the only thing ever drawn on its own canvas, so
// "is anything on this layer" is exactly "is anything selected". The
// ring is dashed and spinning, so the count wobbles frame to frame —
// this only ever asks whether it is there at all.
const selectionInk = (page: Page) => layerInk(page, LAYER.selection);

// The same probe asked "is the ring *here*", which is how a ring that
// stayed behind can be told from one that followed. Wide enough to take
// in the whole ring around a 1×1 token wherever its dashes point.
const selectionInkAt = (page: Page, point: Point) => inkAt(page, LAYER.selection, point, 48);

test('clicking a token selects it here and nowhere else', async ({ table }) => {
	const player = await table.join();
	const gm = table.gm;

	const token = await createToken(gm.page, 'Goblin');

	// Nothing selected to start with, on either the canvas or the strip.
	// The empty strip is a plain shaded block with no text in it, so
	// "nothing selected" is asserted as the token's name being absent.
	await expect(detailsPanel(gm.page)).not.toContainText('Goblin');
	expect(await selectionInk(gm.page)).toBe(0);

	// The strip first: it's the assertion that says whether the click was
	// even understood as landing on the token, so checking it before the
	// pixels makes a miss read differently from a ring that didn't draw.
	await selectToken(gm.page, token, 'Goblin');
	await expect.poll(() => selectionInk(gm.page)).toBeGreaterThan(0);

	// The player is looking at the same token on the same scene and sees
	// no ring: selection is local, and this is the assertion that says so.
	// The GM's positive result above is its control — the same probe on
	// the same layer, one page over.
	await player.page.waitForTimeout(500);
	expect(await selectionInk(player.page)).toBe(0);
	await expect(detailsPanel(player.page)).not.toContainText('Goblin');

	// Empty map three cells to the left clears it again.
	const box = await canvasBox(gm.page);
	await gm.page.mouse.click(box.x + token.x - 3 * GRID, box.y + token.y);
	await expect.poll(() => selectionInk(gm.page)).toBe(0);
	await expect(detailsPanel(gm.page)).not.toContainText('Goblin');
});

test('a selection does not survive a reload', async ({ table }) => {
	const gm = table.gm;

	const token = await createToken(gm.page, 'Goblin');
	await selectToken(gm.page, token, 'Goblin');
	await expect.poll(() => selectionInk(gm.page)).toBeGreaterThan(0);

	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();

	// The token comes back from the server; the selection doesn't, because
	// it was never sent anywhere.
	await expect(detailsPanel(gm.page)).not.toContainText('Goblin');
	expect(await selectionInk(gm.page)).toBe(0);
});

// The ring lives on its own layer rather than inside the token's Konva
// group, so nothing moves it for free — dragging a selected token is the
// case that proves it is actually being followed.
test('the ring follows the token it marks when it is dragged', async ({ table }) => {
	const gm = table.gm;

	const from = await createToken(gm.page, 'Goblin');
	// Up and to the left, so both probe boxes stay well inside the canvas
	// — getImageData off the edge comes back transparent rather than
	// failing, which would make the "not here any more" half pass blind.
	const to: Point = { x: from.x - 2 * GRID, y: from.y - 2 * GRID };

	await selectToken(gm.page, from, 'Goblin');
	await expect.poll(() => selectionInkAt(gm.page, from)).toBeGreaterThan(0);

	await dragToken(gm.page, from, to);

	await expect.poll(() => selectionInkAt(gm.page, to)).toBeGreaterThan(0);
	expect(await selectionInkAt(gm.page, from)).toBe(0);
});
