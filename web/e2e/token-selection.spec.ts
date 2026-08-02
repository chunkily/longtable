import { expect, test, type Browser, type Page } from '@playwright/test';

// Selecting a token is the one piece of room UI that is deliberately
// *not* shared: it never goes on the wire, so proving it stays on one
// client needs two browsers. The highlight itself is a rotating Konva
// ring with no DOM behind it, so it has to be read off the canvas.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview,
// selection. Index 8 is the selection ring, and it is the only thing
// ever drawn there — so "is anything on this layer" is exactly "is
// anything selected".
const SELECTION_LAYER = 8;
const TOKEN_LAYER = 4;

// The scene dialog's default, and what canvas-relative pixels are
// divided by to get cells.
const GRID = 70;

// Opaque pixels across a whole layer.
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

// The ring is dashed and spinning, so the count wobbles frame to frame —
// this only ever asks whether it is there at all.
async function selectionInk(page: Page): Promise<number> {
	return layerInk(page, SELECTION_LAYER);
}

// Opaque pixels in a box around a point of the selection layer — the
// same probe, asked "is the ring *here*", which is how a ring that
// stayed behind can be told from one that followed.
async function selectionInkAt(page: Page, point: { x: number; y: number }): Promise<number> {
	return page.evaluate(
		({ layer, x, y }) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			// Wide enough to take in the whole ring around a 1×1 token
			// wherever its dashes happen to be pointing.
			const radius = 48;
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
		{ layer: SELECTION_LAYER, x: point.x, y: point.y }
	);
}

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

// Where a freshly created token lands, in canvas-relative pixels. A new
// token spawns on the cell at the centre of the creator's view
// (viewCenterCell in game-canvas.svelte), and a fresh scene sits at the
// identity transform — so canvas pixels are world coordinates and this
// is the same arithmetic the app does.
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

async function joinRoomAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

// Waits for two things the caller then depends on, both of which look
// like a broken feature rather than a race when they're missed:
//
//   - The dialog being *gone*, not just closed. It exits on an animation,
//     and while that runs it still covers the middle of the canvas and
//     still takes clicks. page.mouse.click() sends raw coordinates with
//     none of the actionability checks locator.click() makes, so the
//     click lands on the dialog and the canvas never hears about it.
//   - The token being on the map. It arrives over the socket rather than
//     from the click, so a click sent straight afterwards hits bare grid.
async function createToken(page: Page, name: string) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);
}

// The desktop sidebar copy — the mobile sheet renders the same snippet,
// but only one of the two is visible at the test viewport.
function detailsSection(page: Page) {
	return page.getByRole('region', { name: 'Selected token' }).first();
}

test('clicking a token selects it here and nowhere else', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Select');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);
	const token = spawnCentre(box);

	// Nothing selected to start with, on either the canvas or the strip.
	await expect(detailsSection(gm.page)).toContainText('No token selected');
	expect(await selectionInk(gm.page)).toBe(0);

	await gm.page.mouse.click(box.x + token.x, box.y + token.y);

	// The strip first: it's the assertion that says whether the click was
	// even understood as landing on the token, so checking it before the
	// pixels makes a miss read differently from a ring that didn't draw.
	await expect(detailsSection(gm.page)).toContainText('Goblin');
	await expect.poll(() => selectionInk(gm.page)).toBeGreaterThan(0);

	// The player is looking at the same token on the same scene and sees
	// no ring: selection is local, and this is the assertion that says so.
	// The GM's positive result above is its control — the same probe on
	// the same layer, one page over.
	await player.page.waitForTimeout(500);
	expect(await selectionInk(player.page)).toBe(0);
	await expect(detailsSection(player.page)).toContainText('No token selected');

	// Empty map three cells to the left clears it again.
	await gm.page.mouse.click(box.x + token.x - 3 * GRID, box.y + token.y);
	await expect.poll(() => selectionInk(gm.page)).toBe(0);
	await expect(detailsSection(gm.page)).toContainText('No token selected');

	await gm.context.close();
	await player.context.close();
});

test('a selection does not survive a reload', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Select Reload');

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);
	const token = spawnCentre(box);

	await gm.page.mouse.click(box.x + token.x, box.y + token.y);
	await expect.poll(() => selectionInk(gm.page)).toBeGreaterThan(0);

	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();

	// The token comes back from the server; the selection doesn't, because
	// it was never sent anywhere.
	await expect(detailsSection(gm.page)).toContainText('No token selected');
	expect(await selectionInk(gm.page)).toBe(0);

	await gm.context.close();
});

// The ring lives on its own layer rather than inside the token's Konva
// group, so nothing moves it for free — dragging a selected token is the
// case that proves it is actually being followed.
test('the ring follows the token it marks when it is dragged', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Select Drag');

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);
	const from = spawnCentre(box);
	// Up and to the left, so both probe boxes stay well inside the canvas
	// — getImageData off the edge comes back transparent rather than
	// failing, which would make the "not here any more" half pass blind.
	const to = { x: from.x - 2 * GRID, y: from.y - 2 * GRID };

	await gm.page.mouse.click(box.x + from.x, box.y + from.y);
	await expect.poll(() => selectionInkAt(gm.page, from)).toBeGreaterThan(0);

	await gm.page.mouse.move(box.x + from.x, box.y + from.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });
	await gm.page.mouse.up();

	await expect.poll(() => selectionInkAt(gm.page, to)).toBeGreaterThan(0);
	expect(await selectionInkAt(gm.page, from)).toBe(0);

	await gm.context.close();
});
