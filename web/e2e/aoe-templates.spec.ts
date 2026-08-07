import { expect, test, type Browser, type Page } from '@playwright/test';
import { TOOLBAR_CLEARANCE_Y, mapGestureOrigin, openNewSceneDialog, selectTool } from './room';

// Area templates are the measuring tool's gesture wearing a different
// shape: ephemeral, one per participant, gone when the drag ends. None
// of that is visible from the DOM or the database, so this drives two
// browsers and reads the canvas.
//
// What it deliberately does *not* check is which grid squares a template
// covers — nothing highlights them, on purpose. Tables disagree about
// which squares an area catches, so the app draws the true shape and
// leaves the reading to the players. See $lib/aoe's header comment.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview,
// selection, hover.
const MEASURE_LAYER = 6;

// Everything on the measurement layer. A template is an outline, a fill,
// an origin dot and a label; this only asks whether one is there and
// roughly how much of the map it takes up, so counting the whole layer
// beats chasing any one part of it.
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

	await openNewSceneDialog(page);
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

// Canvas-relative pixels double as world coordinates: a fresh scene
// starts at the identity transform and nothing here pans or zooms.
const FROM = { x: 140, y: 140 };
const TO = { x: 420, y: 280 };

async function dragTemplate(page: Page, origin: { x: number; y: number }) {
	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
}

test('a template is on everyone else’s map while it is dragged, and gone after', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Template Share');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	expect(await measureInk(player.page)).toBe(0);

	await selectTool(gm.page, 'Cone template');
	const origin = await mapGestureOrigin(gm.page);
	await dragTemplate(gm.page, origin);

	// Mid-drag: the point of the whole feature is that the rest of the
	// room can see what you're about to do, before you do it.
	await expect.poll(() => measureInk(player.page)).toBeGreaterThan(0);

	await gm.page.mouse.up();

	// A template means nothing once the drag is over, so it must leave
	// nothing behind on anyone's map. Both polled, including the map
	// belonging to whoever drew it: Konva clears the layer through
	// batchDraw, which waits for an animation frame, and a tab sitting
	// idle behind another one is in no hurry to run it.
	await expect.poll(() => measureInk(gm.page)).toBe(0);
	await expect.poll(() => measureInk(player.page)).toBe(0);

	await gm.context.close();
	await player.context.close();
});

// Each shape covers a visibly different area for the same drag, which is
// the cheapest way to prove the kind actually reaches the renderer
// rather than every tool drawing the same thing.
test('each template shape draws a different area for the same drag', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Template Shapes');

	const ink: Record<string, number> = {};
	for (const tool of ['Circle template', 'Cone template', 'Line template', 'Cube template']) {
		await selectTool(gm.page, tool);
		const origin = await mapGestureOrigin(gm.page);
		await dragTemplate(gm.page, origin);
		await expect.poll(() => measureInk(gm.page)).toBeGreaterThan(0);
		ink[tool] = await measureInk(gm.page);
		await gm.page.mouse.up();
		await expect.poll(() => measureInk(gm.page)).toBe(0);
	}

	// A circle of that radius is the largest of the four, and a 5 ft line
	// the smallest — if any two came out equal, the tool wouldn't be
	// reaching the shape.
	expect(new Set(Object.values(ink)).size).toBe(4);
	expect(ink['Circle template']).toBeGreaterThan(ink['Line template']);

	await gm.context.close();
});

test('a line template redraws wider when its width is changed', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Template Line Width');

	await selectTool(gm.page, 'Line template');
	await dragTemplate(gm.page, await mapGestureOrigin(gm.page));
	await expect.poll(() => measureInk(gm.page)).toBeGreaterThan(0);
	const atDefaultWidth = await measureInk(gm.page);
	await gm.page.mouse.up();

	// Width is the one part of a Line a drag can't express, so it comes
	// from a control rather than the gesture.
	await gm.page.getByRole('button', { name: '20 foot wide line' }).click();
	await dragTemplate(gm.page, await mapGestureOrigin(gm.page));
	await expect.poll(() => measureInk(gm.page)).toBeGreaterThan(atDefaultWidth);
	await gm.page.mouse.up();

	await gm.context.close();
});

test('snapping moves where a template lands without changing the drag', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Template Snap');

	// A point of origin deliberately off both a corner and a centre: at a
	// 70px grid this snaps to (280, 210), a good 28px away.
	const offGrid = { x: 263, y: 187 };

	// The origin dot, in a small box around the raw pointer position. This
	// asks "did the origin land where I pointed, or somewhere else" —
	// which is the only thing snapping still decides. Whole-layer ink
	// can't answer it any more: snapping moves the origin without changing
	// the radius, and a circle of the same size translated a few pixels
	// covers the same count.
	//
	// Counting *opaque* pixels rather than any ink is what makes it work
	// at all. A template is filled at 0.18 alpha, so the probe sits inside
	// the circle either way and "is there anything here" answers yes in
	// both. The origin dot is solid, so a threshold well above 46 sees the
	// dot and ignores the fill it is sitting on.
	async function originDotInk(): Promise<number> {
		return gm.page.evaluate(
			({ layer, x, y }) => {
				const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
				const context = canvas.getContext('2d')!;
				const dpr = window.devicePixelRatio || 1;
				const radius = 8;
				const data = context.getImageData(
					(x - radius) * dpr,
					(y - radius) * dpr,
					radius * 2 * dpr,
					radius * 2 * dpr
				).data;

				let opaque = 0;
				for (let i = 3; i < data.length; i += 4) if (data[i] > 200) opaque++;
				return opaque;
			},
			// The drag below is measured from mapGestureOrigin, which starts
			// below the floating toolbar; the canvas's pixel buffer starts at
			// its true corner, so the clearance goes back on here.
			{ layer: MEASURE_LAYER, x: offGrid.x, y: offGrid.y + TOOLBAR_CLEARANCE_Y }
		);
	}

	async function dragFromOffGrid(): Promise<number> {
		const origin = await mapGestureOrigin(gm.page);
		await gm.page.mouse.move(origin.x + offGrid.x, origin.y + offGrid.y);
		await gm.page.mouse.down();
		await gm.page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 6 });
		await expect.poll(() => measureInk(gm.page)).toBeGreaterThan(0);
		const ink = await originDotInk();
		await gm.page.mouse.up();
		await expect.poll(() => measureInk(gm.page)).toBe(0);
		return ink;
	}

	await selectTool(gm.page, 'Circle template');
	const snappedToCorners = await dragFromOffGrid();

	await gm.page.getByRole('button', { name: 'Free', exact: true }).click();
	const unsnapped = await dragFromOffGrid();

	// Left free, the origin dot sits under the pointer. Snapped to
	// corners it has moved off to an intersection, taking the dot with it
	// and leaving nothing behind — the circle's outline is a whole radius
	// away, so it can't wander into the probe either.
	expect(unsnapped).toBeGreaterThan(0);
	expect(snappedToCorners).toBe(0);

	await gm.context.close();
});
