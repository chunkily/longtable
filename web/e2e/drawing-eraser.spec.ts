import { expect, test, type Page } from '@playwright/test';

// The eraser is the one drawing feature whose rules can't be checked
// from the DOM: what it erases depends on Konva hit-testing against a
// stroke, and who's allowed to erase is enforced server-side. So this
// exercises it for real, through two browsers, and reads the result off
// the canvas pixels.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, preview. Index 3 is
// the drawings layer — the only one that should ever have ink here.
const DRAWING_LAYER = 3;

// Points are in canvas-relative pixels, which are also world
// coordinates: a fresh scene starts at the identity transform, and
// nothing in this test pans or zooms.
const GM_LINE = { from: { x: 100, y: 100 }, to: { x: 300, y: 200 } };
const GM_LINE_MIDPOINT = { x: 200, y: 150 };
const PLAYER_LINE = { from: { x: 450, y: 300 }, to: { x: 650, y: 380 } };
const PLAYER_LINE_MIDPOINT = { x: 550, y: 340 };

// Opaque pixel count in a small box around a point of the drawings
// layer — "is there a stroke here?" without caring about antialiasing.
async function inkAt(page: Page, point: { x: number; y: number }): Promise<number> {
	return page.evaluate(
		({ layer, x, y }) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			const radius = 6;
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
		{ layer: DRAWING_LAYER, x: point.x, y: point.y }
	);
}

async function canvasOrigin(page: Page): Promise<{ x: number; y: number }> {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return { x: box.x, y: box.y };
}

// Selecting a tool is what rebinds the canvas's pointer handlers, and
// that happens in a Svelte effect — so a click sent in the same
// millisecond as the button press can land before the new tool is
// listening. Waiting for the button's own active styling is the
// observable signal that the state change has been applied.
//
// The toolbar buttons toggle, so clicking the tool that is already
// selected turns it off; this leaves the requested tool selected either
// way rather than depending on what was selected before.
async function selectTool(page: Page, name: string) {
	const button = page.getByRole('button', { name });
	const alreadyActive = await button.evaluate((el) => el.className.includes('bg-primary'));
	if (!alreadyActive) await button.click();
	await expect(button).toHaveClass(/bg-primary/);
}

async function dragWithTool(
	page: Page,
	tool: string,
	shape: { from: { x: number; y: number }; to: { x: number; y: number } }
) {
	await selectTool(page, tool);
	const origin = await canvasOrigin(page);
	await page.mouse.move(origin.x + shape.from.x, origin.y + shape.from.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + shape.to.x, origin.y + shape.to.y, { steps: 8 });
	await page.mouse.up();
}

async function drawLine(
	page: Page,
	line: { from: { x: number; y: number }; to: { x: number; y: number } }
) {
	await dragWithTool(page, 'Line', line);
}

async function eraseAt(page: Page, point: { x: number; y: number }) {
	// The eraser finds a stroke by hit-testing the canvas, so the stroke
	// has to have been rendered on *this* page before the click — seeing
	// it arrive on someone else's screen isn't enough.
	await expect.poll(() => inkAt(page, point)).toBeGreaterThan(0);
	await selectTool(page, 'Erase');
	const origin = await canvasOrigin(page);
	await page.mouse.click(origin.x + point.x, origin.y + point.y);
}

test('a GM erases anyone drawing, a player only their own', async ({ browser }) => {
	const gmContext = await browser.newContext();
	const gmPage = await gmContext.newPage();

	await gmPage.goto('/');
	await gmPage.getByLabel('Room name').fill('Ink Test');
	await gmPage.getByLabel('Your name (GM)').fill('Alice');
	await gmPage.getByLabel('GM password').fill('hunter2');
	await gmPage.getByRole('button', { name: 'Create room' }).click();

	await expect(gmPage).toHaveURL(/\/r\/[a-z0-9]+/);
	const slug = new URL(gmPage.url()).pathname.split('/').pop()!;

	await gmPage.getByRole('button', { name: 'New scene' }).click();
	await gmPage.getByLabel('Name').fill('Map');
	await gmPage.getByRole('button', { name: 'Create scene' }).click();
	await expect(gmPage.locator('canvas').first()).toBeVisible();

	await drawLine(gmPage, GM_LINE);
	await expect.poll(() => inkAt(gmPage, GM_LINE_MIDPOINT)).toBeGreaterThan(0);

	const playerContext = await browser.newContext();
	const playerPage = await playerContext.newPage();
	await playerPage.goto(`/r/${slug}`);
	await playerPage.getByLabel('Your name').fill('Bob');
	await playerPage.getByRole('button', { name: 'Join' }).click();
	await expect(playerPage.locator('canvas').first()).toBeVisible();

	// The GM's line reaches the player, who then can't erase it.
	await eraseAt(playerPage, GM_LINE_MIDPOINT);
	// Asserting that nothing happens needs a wait: there's no event to
	// hang the assertion on, which is the whole point.
	await playerPage.waitForTimeout(500);
	expect(await inkAt(playerPage, GM_LINE_MIDPOINT)).toBeGreaterThan(0);
	expect(await inkAt(gmPage, GM_LINE_MIDPOINT)).toBeGreaterThan(0);

	// Their own line, though, they can erase — and it goes for everyone.
	await drawLine(playerPage, PLAYER_LINE);
	await expect.poll(() => inkAt(gmPage, PLAYER_LINE_MIDPOINT)).toBeGreaterThan(0);
	await eraseAt(playerPage, PLAYER_LINE_MIDPOINT);
	await expect.poll(() => inkAt(playerPage, PLAYER_LINE_MIDPOINT)).toBe(0);
	await expect.poll(() => inkAt(gmPage, PLAYER_LINE_MIDPOINT)).toBe(0);

	// The GM can erase what the player couldn't.
	await eraseAt(gmPage, GM_LINE_MIDPOINT);
	await expect.poll(() => inkAt(gmPage, GM_LINE_MIDPOINT)).toBe(0);
	await expect.poll(() => inkAt(playerPage, GM_LINE_MIDPOINT)).toBe(0);

	// Erased server-side, not just hidden locally: a reload re-syncs the
	// scene from the server and both strokes are still gone.
	await gmPage.reload();
	await expect(gmPage.locator('canvas').first()).toBeVisible();
	await expect.poll(() => inkAt(gmPage, GM_LINE_MIDPOINT)).toBe(0);
	await expect.poll(() => inkAt(gmPage, PLAYER_LINE_MIDPOINT)).toBe(0);

	await gmContext.close();
	await playerContext.close();
});

// The ellipse is inscribed in the box you drag, the way Paint does it —
// not grown from the press point outwards. Dragging (100,100)→(300,200)
// therefore puts its edge at the midpoints of that box's sides and
// leaves the middle empty. Under the old centre-and-radius behaviour the
// same drag produced a circle of radius ~223 around (100,100), which
// passes through none of the points asserted here.
test('the ellipse tool fills the box it is dragged out in', async ({ page }) => {
	await page.goto('/');
	await page.getByLabel('Room name').fill('Ellipse');
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	await dragWithTool(page, 'Ellipse', { from: { x: 100, y: 100 }, to: { x: 300, y: 200 } });

	const top = { x: 200, y: 100 };
	const left = { x: 100, y: 150 };
	const centre = { x: 200, y: 150 };
	await expect.poll(() => inkAt(page, top)).toBeGreaterThan(0);
	await expect.poll(() => inkAt(page, left)).toBeGreaterThan(0);
	// Unfilled, so the middle is empty — and the eraser agrees, ignoring
	// a click there rather than treating the interior as part of it.
	expect(await inkAt(page, centre)).toBe(0);

	await selectTool(page, 'Erase');
	const origin = await canvasOrigin(page);
	await page.mouse.click(origin.x + centre.x, origin.y + centre.y);
	await page.waitForTimeout(500);
	expect(await inkAt(page, top)).toBeGreaterThan(0);

	// Clicking the curve itself does erase it.
	await page.mouse.click(origin.x + top.x, origin.y + top.y);
	await expect.poll(() => inkAt(page, top)).toBe(0);
	await expect.poll(() => inkAt(page, left)).toBe(0);
});

test('the eraser grabs a stroke from beside it, not only dead-on', async ({ page }) => {
	await page.goto('/');
	await page.getByLabel('Room name').fill('Near Miss');
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	await drawLine(page, GM_LINE);
	await expect.poll(() => inkAt(page, GM_LINE_MIDPOINT)).toBeGreaterThan(0);

	// 9px perpendicular to a stroke that is only 1.5px thick either side
	// of its centreline: nowhere near a rendered pixel, but inside the
	// eraser's reach. GM_LINE runs (100,100) → (300,200), so (-4, 8) is
	// square to it.
	await selectTool(page, 'Erase');
	const origin = await canvasOrigin(page);
	await page.mouse.click(origin.x + GM_LINE_MIDPOINT.x - 4, origin.y + GM_LINE_MIDPOINT.y + 8);
	await expect.poll(() => inkAt(page, GM_LINE_MIDPOINT)).toBe(0);
});
