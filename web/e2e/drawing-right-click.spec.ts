import { expect, test, type Page } from '@playwright/test';

// Right-dragging used to drive every tool exactly as a left-drag does,
// because Konva reports all mouse buttons through the same
// mousedown/mouseup events. These read pixels rather than the DOM, since
// Konva draws to a canvas and there is no element to assert on.
//
// Each case pairs the right-button assertion with a left-button one on
// the same probe. Without that, a test that silently stopped measuring
// the right thing would still pass.

// Layer order, by index into document.querySelectorAll('canvas'):
// 0 map, 1 grid, 2 fog, 3 drawings, 4 tokens, 5 pings, 6 measurements,
// 7 preview, 8 selection. Inserting a layer renumbers these.
const FOG_LAYER = 2;
const DRAWING_LAYER = 3;
const PING_LAYER = 5;
const MEASURE_LAYER = 6;
const PREVIEW_LAYER = 7;

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

// Total alpha across a layer, rather than a count of pixels that have
// any. Fog needs this: a GM's cover is drawn at 0.35 opacity and revealed
// cells are punched out at 0.35 as well, so revealing lowers a pixel's
// alpha without ever taking it to zero — layerInk above counts exactly
// the same number of pixels before and after a reveal and reports no
// change at all.
async function layerAlpha(page: Page, layer: number): Promise<number> {
	return page.evaluate((index) => {
		const canvas = document.querySelectorAll('canvas')[index] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;
		let total = 0;
		for (let i = 3; i < data.length; i += 4) total += data[i];
		return total;
	}, layer);
}

async function canvasOrigin(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return { x: box.x, y: box.y };
}

async function selectTool(page: Page, name: string) {
	const button = page.getByRole('button', { name, exact: true });
	const alreadyActive = await button.evaluate((el) => el.className.includes('bg-primary'));
	if (!alreadyActive) await button.click();
	await expect(button).toHaveClass(/bg-primary/);
}

// The fog button relabels itself rather than just restyling, so the
// shared selectTool can't wait on it — its locator stops matching the
// moment the tool becomes active. Waiting on the new label is what proves
// the switch happened, and the wait matters: handlers are rebound in an
// effect, so a drag in the same tick still lands on the previous tool.
async function selectFogTool(page: Page) {
	await page.getByRole('button', { name: 'Reveal fog', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Painting fog…', exact: true })).toBeVisible();
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

const FROM = { x: 120, y: 120 };
const TO = { x: 320, y: 240 };

async function dragWith(page: Page, button: 'left' | 'right', origin: { x: number; y: number }) {
	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down({ button });
	await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
	await page.mouse.up({ button });
}

// Every tool that draws, since the freehand and rubber-band paths bind
// separate handlers — covering one proves nothing about the other.
for (const tool of ['Freehand', 'Line', 'Rectangle', 'Ellipse']) {
	test(`right-dragging with the ${tool} tool neither draws nor previews`, async ({ page }) => {
		await createRoomWithScene(page, `RightClick ${tool}`);
		await selectTool(page, tool);
		const origin = await canvasOrigin(page);

		// The same pointer path with no button held. Freehand paints a
		// cursor ring on the preview layer showing how wide the line will
		// be, and that ring follows the pointer whatever the buttons are
		// doing — so "no preview" means "no more ink than the ring alone",
		// not "no ink". Ending at the same point keeps the ring identical.
		await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
		await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
		const ringOnly = await layerInk(page, PREVIEW_LAYER);

		await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
		await page.mouse.down({ button: 'right' });
		await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
		// Asserted mid-drag: the preview shape is destroyed on mouseup, so
		// checking only afterwards would pass even if it had been drawn.
		expect(await layerInk(page, PREVIEW_LAYER)).toBe(ringOnly);
		await page.mouse.up({ button: 'right' });

		// No event to poll for, so the absence needs a real wait.
		await page.waitForTimeout(500);
		expect(await layerInk(page, DRAWING_LAYER)).toBe(0);

		// The same drag on the left button does land, which is what proves
		// the probe above was looking at a layer that can show a drawing.
		await dragWith(page, 'left', origin);
		await expect.poll(() => layerInk(page, DRAWING_LAYER)).toBeGreaterThan(0);
	});
}

test('releasing the right button mid-stroke does not commit the left-button drawing', async ({
	page
}) => {
	await createRoomWithScene(page, 'RightClick Mid');
	await selectTool(page, 'Line');
	const origin = await canvasOrigin(page);

	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });

	// A stray right-click part-way through. mouseup fires for it too, and
	// unguarded it would finish the line here, at whatever length it had
	// reached.
	await page.mouse.down({ button: 'right' });
	await page.mouse.up({ button: 'right' });
	await page.waitForTimeout(300);
	expect(await layerInk(page, DRAWING_LAYER)).toBe(0);

	// Still live: the left button finishes it, and only now does it commit.
	await page.mouse.up();
	await expect.poll(() => layerInk(page, DRAWING_LAYER)).toBeGreaterThan(0);
});

test('right-dragging the eraser leaves the stroke alone', async ({ page }) => {
	await createRoomWithScene(page, 'RightClick Erase');
	const origin = await canvasOrigin(page);

	await selectTool(page, 'Line');
	await dragWith(page, 'left', origin);
	await expect.poll(() => layerInk(page, DRAWING_LAYER)).toBeGreaterThan(0);
	const drawn = await layerInk(page, DRAWING_LAYER);

	await selectTool(page, 'Erase');
	await dragWith(page, 'right', origin);
	await page.waitForTimeout(500);
	expect(await layerInk(page, DRAWING_LAYER)).toBe(drawn);

	// Left-dragging the same path does erase it, so the stroke really was
	// within the eraser's reach the whole time.
	await dragWith(page, 'left', origin);
	await expect.poll(() => layerInk(page, DRAWING_LAYER)).toBe(0);
});

test('right-dragging the fog tool reveals nothing', async ({ page }) => {
	await createRoomWithScene(page, 'RightClick Fog');
	const origin = await canvasOrigin(page);

	await selectFogTool(page);
	// Fog starts as a cover over the scene, and revealing takes alpha out
	// of it, so a reveal shows up as the layer losing alpha rather than
	// gaining anything.
	const covered = await layerAlpha(page, FOG_LAYER);
	expect(covered).toBeGreaterThan(0);

	await dragWith(page, 'right', origin);
	await page.waitForTimeout(500);
	expect(await layerAlpha(page, FOG_LAYER)).toBe(covered);

	await dragWith(page, 'left', origin);
	await expect.poll(() => layerAlpha(page, FOG_LAYER)).toBeLessThan(covered);
});

test('right-clicking the ping tool sends no ping', async ({ page }) => {
	await createRoomWithScene(page, 'RightClick Ping');
	await selectTool(page, 'Ping');
	const origin = await canvasOrigin(page);

	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down({ button: 'right' });
	await page.mouse.up({ button: 'right' });
	await page.waitForTimeout(500);
	expect(await layerInk(page, PING_LAYER)).toBe(0);

	await page.mouse.down();
	await page.mouse.up();
	await expect.poll(() => layerInk(page, PING_LAYER)).toBeGreaterThan(0);
});

test('right-dragging the measure tool measures nothing', async ({ page }) => {
	await createRoomWithScene(page, 'RightClick Measure');
	await selectTool(page, 'Measure');
	const origin = await canvasOrigin(page);

	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down({ button: 'right' });
	await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
	// Measurements are ephemeral and vanish on release, so this has to be
	// asserted while the drag is still held.
	expect(await layerInk(page, MEASURE_LAYER)).toBe(0);
	await page.mouse.up({ button: 'right' });

	await page.mouse.move(origin.x + FROM.x, origin.y + FROM.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + TO.x, origin.y + TO.y, { steps: 8 });
	await expect.poll(() => layerInk(page, MEASURE_LAYER)).toBeGreaterThan(0);
	await page.mouse.up();
});
