import { expect, test } from './fixtures/table';
import { GRID, LAYER, canvasBox, createToken, inkAt, layerInk, watchInkAt } from './fixtures/map';
import type { Point } from './fixtures/map';

// The ghost, the line and the distance label are all drawn on the
// preview layer, and all three only exist while a button is held down —
// so every one of these drives the mouse by hand rather than through
// dragToken, and probes between the `down` and the `up`.
//
// The distance the label reports is unit-tested in
// `src/lib/token-drag.test.ts`; text painted into a canvas can't be read
// back, so what a browser is needed for is *where* the overlay is and
// *when* it goes away.

test('dragging a token leaves a ghost behind it and a line to where it would land', async ({
	table
}) => {
	const gm = table.gm;

	const from = await createToken(gm.page, 'Goblin');
	const to: Point = { x: from.x + 4 * GRID, y: from.y };
	const midpoint: Point = { x: (from.x + to.x) / 2, y: from.y };

	// Nothing on this layer before the drag, so anything found below
	// belongs to the drag rather than to a tool left armed.
	expect(await layerInk(gm.page, LAYER.preview)).toBe(0);

	const box = await canvasBox(gm.page);
	await gm.page.mouse.move(box.x + from.x, box.y + from.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });

	// The ghost sits on the square the token was picked up from — which is
	// now empty on the token layer, since the token itself is under the
	// pointer four squares away.
	await expect.poll(() => inkAt(gm.page, LAYER.preview, from)).toBeGreaterThan(0);
	expect(await inkAt(gm.page, LAYER.tokens, from)).toBe(0);

	// And the line runs between the two, through a square nothing else has
	// ever drawn in.
	expect(await inkAt(gm.page, LAYER.preview, midpoint)).toBeGreaterThan(0);

	await gm.page.mouse.up();

	// All of it goes on release, ghost included.
	await expect.poll(() => layerInk(gm.page, LAYER.preview)).toBe(0);
	await expect.poll(() => inkAt(gm.page, LAYER.tokens, to)).toBeGreaterThan(0);
});

// A drop back onto the square it started from is the case that gets
// missed: RoomClient treats it as a no-op, so no state change arrives to
// force a re-render, and anything relying on one to tidy up would strand
// the ghost on the map for the rest of the session.
test('the ghost goes even when the token is dropped back where it started', async ({ table }) => {
	const gm = table.gm;

	const from = await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);

	await gm.page.mouse.move(box.x + from.x, box.y + from.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + from.x + 2 * GRID, box.y + from.y, { steps: 6 });
	await expect.poll(() => inkAt(gm.page, LAYER.preview, from)).toBeGreaterThan(0);

	// Back to the square it came from, then let go.
	await gm.page.mouse.move(box.x + from.x, box.y + from.y, { steps: 6 });
	await gm.page.mouse.up();

	await expect.poll(() => layerInk(gm.page, LAYER.preview)).toBe(0);
});

// "How far am I moving this" is the dragger's own question, so the
// overlay never goes on the wire. If it ever did, everyone else's map
// would grow a line and a label every time anybody nudged anything.
test('nobody else sees the ghost or the line', async ({ table }) => {
	const player = await table.join();
	const gm = table.gm;

	const from = await createToken(gm.page, 'Goblin');
	const to: Point = { x: from.x + 4 * GRID, y: from.y };
	const midpoint: Point = { x: (from.x + to.x) / 2, y: from.y };

	// Sampled every frame from before the drag until after it, rather than
	// probed once mid-drag: a leak that arrived over the socket would show
	// up at whatever moment the broadcast landed, and a single probe can
	// fall either side of that.
	const ghost = await watchInkAt(player.page, from, LAYER.preview);
	const line = await watchInkAt(player.page, midpoint, LAYER.preview);

	const box = await canvasBox(gm.page);
	await gm.page.mouse.move(box.x + from.x, box.y + from.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });

	// The GM's own overlay is up, so the drag is genuinely in progress and
	// a silent player means absence rather than mistimed probes.
	await expect.poll(() => inkAt(gm.page, LAYER.preview, from)).toBeGreaterThan(0);

	await gm.page.mouse.up();
	await expect.poll(() => inkAt(player.page, LAYER.tokens, to)).toBeGreaterThan(0);

	expect(await ghost.stop()).toBe(false);
	expect(await line.stop()).toBe(false);
});
