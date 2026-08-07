import { expect, test, type Browser, type Page } from '@playwright/test';
import { openNewSceneDialog, selectTool } from './room';

// Pinch-to-zoom on a touch device — the one gesture the map was missing,
// and the reason it was unusable on the tablet people actually bring to
// a table.
//
// Playwright's `page.touchscreen` is single-touch, so a real pinch can't
// be driven from the ordinary API. These tests dispatch two touch points
// through raw CDP instead, which is Chromium-only and the first thing in
// this suite to need it. The arithmetic itself is unit-tested in
// src/lib/pinch.test.ts; what's checked here is that the gesture reaches
// the stage at all, and that a zoom re-renders the things sized in
// screen pixels rather than only redrawing them.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview,
// selection, hover.
const GRID_LAYER = 1;
const DRAWING_LAYER = 3;
const TOKEN_LAYER = 4;

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

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

/**
 * Drives a two-finger gesture through CDP. Points are viewport
 * coordinates. The move is broken into steps because the handler reads a
 * ratio against the previous sample — a single jump would work, but a
 * real gesture arrives as many small ones, and stepping here is what
 * makes this exercise the same accumulate-per-frame path.
 */
async function pinch(
	page: Page,
	start: [{ x: number; y: number }, { x: number; y: number }],
	end: [{ x: number; y: number }, { x: number; y: number }],
	steps = 8
) {
	const client = await page.context().newCDPSession(page);
	const lerp = (a: number, b: number, t: number) => a + (b - a) * t;

	await client.send('Input.dispatchTouchEvent', {
		type: 'touchStart',
		touchPoints: [
			{ x: start[0].x, y: start[0].y, id: 1 },
			{ x: start[1].x, y: start[1].y, id: 2 }
		]
	});

	for (let i = 1; i <= steps; i++) {
		const t = i / steps;
		await client.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: [
				{ x: lerp(start[0].x, end[0].x, t), y: lerp(start[0].y, end[0].y, t), id: 1 },
				{ x: lerp(start[1].x, end[1].x, t), y: lerp(start[1].y, end[1].y, t), id: 2 }
			]
		});
	}

	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await client.detach();
}

async function openRoomAsGM(browser: Browser, roomName: string) {
	// hasTouch is what makes the browser dispatch touch events at all;
	// without it the CDP calls land and Konva never binds a touch handler.
	const context = await browser.newContext({ hasTouch: true });
	const page = await context.newPage();

	await page.goto('/');
	await page.getByLabel('Room name').fill(roomName);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();
	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

// Spreading the fingers scales the map up, which shows up as a token
// covering more pixels and the grid drawing fewer lines across the same
// viewport. Both are read off canvases rather than asked of Konva, since
// the stage isn't reachable from the page.
test('a two-finger spread zooms the map in, and a pinch takes it back out', async ({ browser }) => {
	const { context, page } = await openRoomAsGM(browser, 'Pinch Zoom');

	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill('Ogre');
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);

	const box = await canvasBox(page);
	const centre = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
	const atRest = await layerInk(page, TOKEN_LAYER);
	const gridAtRest = await layerInk(page, GRID_LAYER);

	await pinch(
		page,
		[
			{ x: centre.x - 40, y: centre.y },
			{ x: centre.x + 40, y: centre.y }
		],
		[
			{ x: centre.x - 160, y: centre.y },
			{ x: centre.x + 160, y: centre.y }
		]
	);

	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(atRest * 1.5);
	// Fewer grid lines fit across the viewport once each square is bigger,
	// so the grid layer holds less ink rather than more — which is also
	// the assertion that the grid was re-rendered for the new scale
	// instead of being left at the old spacing.
	expect(await layerInk(page, GRID_LAYER)).toBeLessThan(gridAtRest);

	const zoomedIn = await layerInk(page, TOKEN_LAYER);

	await pinch(
		page,
		[
			{ x: centre.x - 160, y: centre.y },
			{ x: centre.x + 160, y: centre.y }
		],
		[
			{ x: centre.x - 40, y: centre.y },
			{ x: centre.x + 40, y: centre.y }
		]
	);

	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeLessThan(zoomedIn);

	await context.close();
});

// The zoom is bounded by the same limits the wheel respects. Spreading
// far past the maximum has to stop scaling rather than run away, and —
// the part worth pinning — has to stop *moving* too: an anchor computed
// against an unclamped scale would keep sliding the map while the number
// stayed put.
test('zooming past the limit stops rather than sliding the map', async ({ browser }) => {
	const { context, page } = await openRoomAsGM(browser, 'Pinch Limit');

	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill('Ogre');
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);

	const box = await canvasBox(page);
	const centre = { x: box.x + box.width / 2, y: box.y + box.height / 2 };

	const spreadHard = async () =>
		pinch(
			page,
			[
				{ x: centre.x - 20, y: centre.y },
				{ x: centre.x + 20, y: centre.y }
			],
			[
				{ x: centre.x - 300, y: centre.y },
				{ x: centre.x + 300, y: centre.y }
			]
		);

	await spreadHard();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);
	const atLimit = await layerInk(page, TOKEN_LAYER);

	// Already at MAX_SCALE, so a second identical gesture should change
	// nothing at all.
	await spreadHard();
	expect(await layerInk(page, TOKEN_LAYER)).toBe(atLimit);

	await context.close();
});

// A tool owns the pointer and a pinch is two of them. The second finger
// abandons the in-flight gesture rather than folding into it, so nobody
// ends up with half a line they never asked for — and, the part that
// reaches other people, no measurement frozen on their map.
test('a pinch started mid-stroke abandons it instead of drawing', async ({ browser }) => {
	const { context, page } = await openRoomAsGM(browser, 'Pinch Mid Stroke');

	const box = await canvasBox(page);
	const centre = { x: box.x + box.width / 2, y: box.y + box.height / 2 };

	await selectTool(page, 'Freehand');

	const client = await page.context().newCDPSession(page);
	// One finger down and moving: a stroke is now in progress.
	await client.send('Input.dispatchTouchEvent', {
		type: 'touchStart',
		touchPoints: [{ x: centre.x - 60, y: centre.y, id: 1 }]
	});
	for (let i = 1; i <= 5; i++) {
		await client.send('Input.dispatchTouchEvent', {
			type: 'touchMove',
			touchPoints: [{ x: centre.x - 60 + i * 12, y: centre.y, id: 1 }]
		});
	}

	// A second finger lands and the gesture becomes a pinch.
	await client.send('Input.dispatchTouchEvent', {
		type: 'touchMove',
		touchPoints: [
			{ x: centre.x, y: centre.y, id: 1 },
			{ x: centre.x + 80, y: centre.y, id: 2 }
		]
	});
	await client.send('Input.dispatchTouchEvent', {
		type: 'touchMove',
		touchPoints: [
			{ x: centre.x - 60, y: centre.y, id: 1 },
			{ x: centre.x + 200, y: centre.y, id: 2 }
		]
	});
	await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
	await client.detach();

	// Nothing was committed: the drawings layer is still empty. Reloading
	// proves it against the server rather than against this canvas.
	await page.reload();
	await expect(page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => layerInk(page, DRAWING_LAYER)).toBe(0);

	await context.close();
});
