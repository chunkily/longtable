import { expect, test, type Browser, type Page } from '@playwright/test';

// A measurement exists only while someone is dragging it out, and it has
// to be on everyone's map for those few seconds — neither half of that
// can be seen from the DOM or from the database, so this drives two
// browsers and reads the canvas.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview.
const MEASURE_LAYER = 6;

// Everything drawn on the measurement layer. A measurement is a line, a
// dot at each end and a label, and this only ever asks whether one is
// there at all, so counting the whole layer beats chasing any one part
// of it.
async function measureInk(page: Page): Promise<number> {
	return page.evaluate((layer) => {
		const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;

		let opaque = 0;
		for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
		return opaque;
	}, MEASURE_LAYER);
}

async function canvasOrigin(page: Page): Promise<{ x: number; y: number }> {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return { x: box.x, y: box.y };
}

// See drawing-eraser.spec.ts: the active styling is the observable
// signal that the canvas has rebound its pointer handlers to the new
// tool, so a drag started after it can't land on the old one.
async function selectTool(page: Page, name: string) {
	const button = page.getByRole('button', { name, exact: true });
	const alreadyActive = await button.evaluate((el) => el.className.includes('bg-primary'));
	if (!alreadyActive) await button.click();
	await expect(button).toHaveClass(/bg-primary/);
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

// Canvas-relative pixels are world coordinates here: a fresh scene
// starts at the identity transform and nothing below pans or zooms.
// With the default 70px grid this drag runs four cells diagonally.
const MEASURE_FROM = { x: 105, y: 105 };
const MEASURE_TO = { x: 385, y: 385 };

test('a measurement is shared while it is dragged and gone once it ends', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Measure');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await selectTool(gm.page, 'Measure');
	const origin = await canvasOrigin(gm.page);
	await gm.page.mouse.move(origin.x + MEASURE_FROM.x, origin.y + MEASURE_FROM.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(origin.x + MEASURE_TO.x, origin.y + MEASURE_TO.y, { steps: 8 });

	// Still held down: the whole point is that the rest of the table can
	// see it while it's being made, not only afterwards.
	await expect.poll(() => measureInk(gm.page)).toBeGreaterThan(0);
	await expect.poll(() => measureInk(player.page)).toBeGreaterThan(0);

	await gm.page.mouse.up();

	await expect.poll(() => measureInk(gm.page)).toBe(0);
	await expect.poll(() => measureInk(player.page)).toBe(0);

	// And nothing was left behind on the scene to come back with it.
	await player.page.reload();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	expect(await measureInk(player.page)).toBe(0);

	await gm.context.close();
	await player.context.close();
});

// Without the server clearing up after a dropped connection there is no
// end event for this measurement at all, and it would hang on the
// player's map until the scene changed.
test("a measurer's line is cleared when they disconnect mid-drag", async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Measure Drop');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await selectTool(gm.page, 'Measure');
	const origin = await canvasOrigin(gm.page);
	await gm.page.mouse.move(origin.x + MEASURE_FROM.x, origin.y + MEASURE_FROM.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(origin.x + MEASURE_TO.x, origin.y + MEASURE_TO.y, { steps: 8 });
	await expect.poll(() => measureInk(player.page)).toBeGreaterThan(0);

	// Closed mid-drag, so no measure.end is ever sent.
	await gm.context.close();

	await expect.poll(() => measureInk(player.page)).toBe(0);

	await player.context.close();
});
