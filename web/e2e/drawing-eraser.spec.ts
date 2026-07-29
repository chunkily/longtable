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
async function selectTool(page: Page, name: string) {
	const button = page.getByRole('button', { name });
	await button.click();
	await expect(button).toHaveClass(/bg-primary/);
}

async function drawLine(
	page: Page,
	line: { from: { x: number; y: number }; to: { x: number; y: number } }
) {
	await selectTool(page, 'Line');
	const origin = await canvasOrigin(page);
	await page.mouse.move(origin.x + line.from.x, origin.y + line.from.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + line.to.x, origin.y + line.to.y, { steps: 8 });
	await page.mouse.up();
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
