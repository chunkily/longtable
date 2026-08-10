import { expect, test } from './fixtures/table';
import {
	GRID,
	LAYER,
	canvasBox,
	createToken,
	dragToken,
	inkAt,
	watchInkAt,
	type Point
} from './fixtures/map';
import type { Page } from '@playwright/test';

// A token someone else moves used to jump. Proving it slides instead
// means catching it *between* the two squares, which only a real browser
// mid-animation can show — and the assertion has to be that it is in
// neither square rather than that it is in some particular place, since
// where it has got to depends on when the frame landed.

// A tighter probe than the shared default: the midpoint sample must not
// overlap either endpoint, which at 4 squares apart it otherwise would.
const probe = (page: Page, point: Point) => inkAt(page, LAYER.tokens, point, 20);

test('a token someone else moves slides rather than jumping', async ({ table }) => {
	const player = await table.join();
	const gm = table.gm;

	const from = await createToken(gm.page, 'Goblin');
	// Far enough that the two probes can't overlap, and that the slide
	// lasts long enough to be caught in the middle of.
	const to: Point = { x: from.x - 4 * GRID, y: from.y };
	const midpoint: Point = { x: (from.x + to.x) / 2, y: from.y };

	await expect.poll(() => probe(player.page, from)).toBeGreaterThan(0);

	// Watching the halfway square before anything moves. Nothing has ever
	// been drawn there, so any ink at all means the token passed through
	// rather than teleporting over it.
	const transit = await watchInkAt(player.page, midpoint);

	// Drag it on the GM's map. The player only learns about it from the
	// broadcast, which is the case that used to teleport.
	await dragToken(gm.page, from, to);

	// It arrives in the right square...
	await expect.poll(() => probe(player.page, to)).toBeGreaterThan(0);
	expect(await probe(player.page, from)).toBe(0);
	expect(await probe(player.page, midpoint)).toBe(0);

	// ...having been seen in between on the way, which is the whole claim.
	expect(await transit.stop()).toBe(true);
});

// Whoever did the dragging has already watched the token travel under
// their own pointer. Sliding it a second time when the echo arrives
// would be a visible rubber-band back to the square they left.
test('the person dragging does not see it slide again on the echo', async ({ table }) => {
	const gm = table.gm;

	const from = await createToken(gm.page, 'Goblin');
	const to: Point = { x: from.x - 4 * GRID, y: from.y };
	const midpoint: Point = { x: (from.x + to.x) / 2, y: from.y };

	await expect.poll(() => probe(gm.page, from)).toBeGreaterThan(0);

	// Driven by hand rather than through dragToken, because the watch has
	// to start while the button is still down.
	const box = await canvasBox(gm.page);
	await gm.page.mouse.move(box.x + from.x, box.y + from.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });

	// Started while the button is still down and the token is already at
	// its destination under the pointer, so the drag itself can't trip it.
	// A re-slide on the echo would put the token back at the square it
	// left before tweening forward again, and this would see that frame.
	// Settling and looking afterwards would miss it entirely.
	const rubberBand = await watchInkAt(gm.page, from);
	await gm.page.mouse.up();

	await gm.page.waitForTimeout(600);
	expect(await rubberBand.stop()).toBe(false);
	expect(await probe(gm.page, to)).toBeGreaterThan(0);
	expect(await probe(gm.page, midpoint)).toBe(0);
});
