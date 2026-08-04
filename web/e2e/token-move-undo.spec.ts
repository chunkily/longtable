import { expect, test, type Browser, type Page } from '@playwright/test';

// Undoing a token move over the real stack. Both halves need a browser:
// the move is a drag on a Konva canvas with no DOM to assert on, and the
// claim being made — "back in the square it came from, for everyone" —
// is about where pixels are on a second client's screen.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview,
// selection, hover.
const TOKEN_LAYER = 4;

// The scene dialog's default, and what canvas-relative pixels are
// divided by to get cells.
const GRID = 70;

async function layerInk(page: Page, layer: number): Promise<number> {
	return page.evaluate((index) => {
		const canvas = document.querySelectorAll('canvas')[index] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;

		let opaque = 0;
		for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
		return opaque;
	}, layer);
}

// Opaque pixels in a box around a point of the token layer — "is the
// token *here*", which is what makes "went back to the square it left"
// checkable rather than just "moved somewhere".
async function tokenInkAt(page: Page, point: { x: number; y: number }): Promise<number> {
	return page.evaluate(
		({ layer, x, y }) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			const radius = 30;
			const data = context.getImageData(
				(x - radius) * dpr,
				(y - radius) * dpr,
				radius * 2 * dpr,
				radius * 2 * dpr
			).data;

			let opaque = 0;
			for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
			return opaque;
		},
		{ layer: TOKEN_LAYER, x: point.x, y: point.y }
	);
}

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

// Where a freshly created token lands: the cell at the centre of the
// *creator's* view, and a fresh scene sits at the identity transform, so
// canvas pixels are world coordinates. See token-selection.spec.ts.
//
// Which view it is matters here in a way it doesn't in the single-browser
// specs: a GM's toolbar is taller than a Player's, so the two canvases
// are neither the same size nor at the same page offset. World points are
// shared between the browsers, but turning one into a mouse coordinate
// has to use the box belonging to the page being clicked.
function spawnCentre(box: { width: number; height: number }) {
	const cell = {
		x: Math.round(box.width / 2 / GRID),
		y: Math.round(box.height / 2 / GRID)
	};
	return { x: cell.x * GRID + GRID / 2, y: cell.y * GRID + GRID / 2 };
}

async function openRoomAsGM(browser: Browser, roomName: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto('/');
	await page.getByLabel('Room name').fill(roomName);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	const slug = new URL(page.url()).pathname.split('/').pop()!;

	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page, slug };
}

async function joinRoomAsPlayer(browser: Browser, slug: string, name = 'Bob') {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await page.getByLabel('Your name').fill(name);
	await page.getByRole('button', { name: 'Join' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

async function createToken(page: Page, name: string) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);
}

async function dragToken(page: Page, box: { x: number; y: number }, from: Point, to: Point) {
	await page.mouse.move(box.x + from.x, box.y + from.y);
	await page.mouse.down();
	await page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });
	await page.mouse.up();
}

interface Point {
	x: number;
	y: number;
}

function undoButton(page: Page) {
	return page.getByRole('button', { name: 'Undo', exact: true });
}

// Waits for a move made elsewhere to have finished arriving — not just
// for the token to be visible at its destination, but for the slide that
// carried it there to be over. Ink shows up under the probe partway
// through the tween, and grabbing the token while it is still animating
// starts a drag Konva then fights with the tween, which leaves the token
// where it was. The slide is a fixed 0.22s (TOKEN_MOVE_SECONDS in
// game-canvas.svelte); this is that with room to spare.
async function settleAt(page: Page, point: Point) {
	await expect.poll(() => tokenInkAt(page, point)).toBeGreaterThan(0);
	await page.waitForTimeout(400);
}

// Anyone at the table can move a token, so anyone has to be able to take
// the move back — this drives it from the Player's browser rather than
// the GM's for that reason.
test('a player undoes their own token move and the whole room sees it go back', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Undo Token Move');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const gmBox = await canvasBox(gm.page);
	const playerBox = await canvasBox(player.page);
	const spawn = spawnCentre(gmBox);
	// Far enough that the two probes can't overlap.
	const moved = { x: spawn.x - 3 * GRID, y: spawn.y - 2 * GRID };

	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);
	await dragToken(player.page, playerBox, spawn, moved);
	await expect.poll(() => tokenInkAt(gm.page, moved)).toBeGreaterThan(0);

	await undoButton(player.page).click();

	// Back where it started on both screens. The GM never touched it and
	// learns about both the move and its undo the same way — from the
	// broadcast — which is the half that would silently not happen if undo
	// only put the token back locally.
	await expect.poll(() => tokenInkAt(gm.page, spawn)).toBeGreaterThan(0);
	expect(await tokenInkAt(gm.page, moved)).toBe(0);
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	// And it's really back on the server, not just on two canvases.
	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => tokenInkAt(gm.page, spawn)).toBeGreaterThan(0);

	await gm.context.close();
	await player.context.close();
});

// The acceptance criterion that undo never reverts somebody else's move.
// The history can't tell who dragged last, so the position stands in for
// it: a token that isn't where your move left it has been moved since,
// and the entry is passed over rather than dragging it back out from
// under whoever moved it.
test('undo passes over a move once someone else has moved the same token', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Undo Token Move Contested');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const gmBox = await canvasBox(gm.page);
	const playerBox = await canvasBox(player.page);
	const spawn = spawnCentre(gmBox);
	const byGM = { x: spawn.x - 3 * GRID, y: spawn.y };
	const byPlayer = { x: spawn.x - 3 * GRID, y: spawn.y - 3 * GRID };

	await dragToken(gm.page, gmBox, spawn, byGM);
	await settleAt(player.page, byGM);

	await dragToken(player.page, playerBox, byGM, byPlayer);
	await settleAt(gm.page, byPlayer);

	// The GM still has their own move on the stack, so the button is live.
	await expect(undoButton(gm.page)).toBeEnabled();
	await undoButton(gm.page).click();

	// Going disabled is how the test knows the click was handled: the entry
	// was taken off the stack and declined, leaving nothing behind it.
	// Without that signal, "the token didn't move" is a race with an undo
	// that simply hadn't happened yet.
	await expect(undoButton(gm.page)).toBeDisabled();
	expect(await tokenInkAt(gm.page, byPlayer)).toBeGreaterThan(0);
	expect(await tokenInkAt(gm.page, byGM)).toBe(0);

	await gm.context.close();
	await player.context.close();
});
