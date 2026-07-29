import { expect, test, type Page } from '@playwright/test';

// Undo and redo over the real stack: each step is a genuine draw.create
// or draw.delete to the server, so what these check is that a stroke
// actually comes back under the same id rather than just reappearing on
// one client's canvas.

const DRAWING_LAYER = 3;

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

async function canvasOrigin(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return { x: box.x, y: box.y };
}

async function selectTool(page: Page, name: string) {
	const button = page.getByRole('button', { name });
	const alreadyActive = await button.evaluate((el) => el.className.includes('bg-primary'));
	if (!alreadyActive) await button.click();
	await expect(button).toHaveClass(/bg-primary/);
}

async function drawLine(page: Page, y: number) {
	await selectTool(page, 'Line');
	const origin = await canvasOrigin(page);
	await page.mouse.move(origin.x + 100, origin.y + y);
	await page.mouse.down();
	await page.mouse.move(origin.x + 400, origin.y + y, { steps: 8 });
	await page.mouse.up();
}

async function createRoomWithScene(page: Page, name: string) {
	await page.goto('/');
	await page.getByLabel('Room name').fill(name);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();
}

const FIRST = { x: 250, y: 120 };
const SECOND = { x: 250, y: 200 };

test('undo and redo step back and forth through your own drawings', async ({ page }) => {
	await createRoomWithScene(page, 'Undo');

	await drawLine(page, FIRST.y);
	await drawLine(page, SECOND.y);
	await expect.poll(() => inkAt(page, FIRST)).toBeGreaterThan(0);
	await expect.poll(() => inkAt(page, SECOND)).toBeGreaterThan(0);

	// Newest first, one step at a time.
	await page.keyboard.press('Control+z');
	await expect.poll(() => inkAt(page, SECOND)).toBe(0);
	expect(await inkAt(page, FIRST)).toBeGreaterThan(0);

	await page.keyboard.press('Control+z');
	await expect.poll(() => inkAt(page, FIRST)).toBe(0);

	// Nothing left to undo: the button says so, and pressing again is
	// harmless.
	await expect(page.getByRole('button', { name: 'Undo' })).toBeDisabled();
	await page.keyboard.press('Control+z');
	expect(await inkAt(page, FIRST)).toBe(0);

	// And back out again, oldest first.
	await page.keyboard.press('Control+Shift+z');
	await expect.poll(() => inkAt(page, FIRST)).toBeGreaterThan(0);

	await page.getByRole('button', { name: 'Redo' }).click();
	await expect.poll(() => inkAt(page, SECOND)).toBeGreaterThan(0);
	await expect(page.getByRole('button', { name: 'Redo' })).toBeDisabled();
});

test('undo puts back a stroke you erased', async ({ page }) => {
	await createRoomWithScene(page, 'Undo Erase');

	await drawLine(page, FIRST.y);
	await expect.poll(() => inkAt(page, FIRST)).toBeGreaterThan(0);

	await selectTool(page, 'Erase');
	const origin = await canvasOrigin(page);
	await page.mouse.click(origin.x + FIRST.x, origin.y + FIRST.y);
	await expect.poll(() => inkAt(page, FIRST)).toBe(0);

	await page.keyboard.press('Control+z');
	await expect.poll(() => inkAt(page, FIRST)).toBeGreaterThan(0);

	// It survives a reload, so it was restored on the server and not
	// just redrawn locally.
	await page.reload();
	await expect(page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => inkAt(page, FIRST)).toBeGreaterThan(0);
});

test('typing in the chat box keeps its own undo', async ({ page }) => {
	await createRoomWithScene(page, 'Undo Chat');

	await drawLine(page, FIRST.y);
	await expect.poll(() => inkAt(page, FIRST)).toBeGreaterThan(0);

	const chat = page.getByPlaceholder('Say something, or /roll 2d6+3');
	await chat.fill('a message I am rethinking');
	await chat.press('Control+z');

	// The stroke is untouched — Ctrl+Z belonged to the text field.
	await page.waitForTimeout(300);
	expect(await inkAt(page, FIRST)).toBeGreaterThan(0);
});
