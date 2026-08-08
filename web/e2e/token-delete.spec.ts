import { expect, test, type Browser, type Page } from '@playwright/test';
import { joinAsNewPlayer, openNewSceneDialog } from './room';

// Deleting a token has to reach the whole room and be recoverable, and
// neither half shows up in the DOM: the token is Konva, and "it came
// back in the same square" is a claim about pixels. So this drives two
// browsers and reads the canvas.

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
// token *here*", which is what makes "restored in the same square"
// checkable rather than just "restored somewhere".
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
// creator's view, and a fresh scene sits at the identity transform, so
// canvas pixels are world coordinates. See token-selection.spec.ts.
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
	await joinAsNewPlayer(page, 'Bob');
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

// Waits for the dialog to be gone as well as the token to arrive —
// page.mouse.click() makes no actionability checks, so a dialog still
// running its exit animation swallows the click meant for the canvas.
async function createToken(page: Page, name: string) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);
}

// Both layouts render the same snippet, so the sidebar and the mobile
// sheet each have one; only the sidebar is visible at this viewport.
function detailsSection(page: Page) {
	return page.getByRole('region', { name: 'Selected token' }).first();
}

function deleteButton(page: Page) {
	return page.getByRole('button', { name: 'Delete token' }).first();
}

test('a GM deletes the selected token for the whole room, and undo puts it back', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Delete Token');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	// Two cells up and left of where it spawned, so what's asserted below
	// is that the token came back *where it was*, not merely that a token
	// with that name exists again.
	const moved = { x: spawn.x - 2 * GRID, y: spawn.y - 2 * GRID };

	// Selected *before* the drag rather than after it, and not for
	// convenience: renderTokens destroys and rebuilds every token group
	// when the token.moved echo arrives, and Konva only fires `click` if
	// mousedown and mouseup landed on the same node. A click sent straight
	// after a drag races that rebuild and is silently swallowed — it fails
	// about three runs in four. The selection survives a drag either way
	// (token-selection.spec.ts proves that), so this covers the same
	// ground without the race.
	await gm.page.mouse.click(box.x + spawn.x, box.y + spawn.y);
	await expect(detailsSection(gm.page)).toContainText('Goblin');

	await gm.page.mouse.move(box.x + spawn.x, box.y + spawn.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + moved.x, box.y + moved.y, { steps: 8 });
	await gm.page.mouse.up();
	await expect.poll(() => tokenInkAt(player.page, moved)).toBeGreaterThan(0);

	await deleteButton(gm.page).click();

	// Gone for the player, who only knows through the broadcast — and the
	// GM's strip falls back to its empty state because the token behind
	// the selection is no longer there.
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBe(0);
	await expect.poll(() => layerInk(gm.page, TOKEN_LAYER)).toBe(0);
	await expect(detailsSection(gm.page)).toContainText('No token selected');

	await gm.page.getByRole('button', { name: 'Undo', exact: true }).click();

	// Back in the square it was standing in, for everyone.
	await expect.poll(() => tokenInkAt(player.page, moved)).toBeGreaterThan(0);
	expect(await tokenInkAt(player.page, spawn)).toBe(0);
	// And selected again on the GM's screen: the id was never cleared, so
	// a token returning under it returns selected. Deliberate — see the
	// backlog note — and the reason nothing has to clear it by hand.
	await expect(detailsSection(gm.page)).toContainText('Goblin');

	// It's really on the server, not just back on two canvases.
	await player.page.reload();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => tokenInkAt(player.page, moved)).toBeGreaterThan(0);

	await gm.context.close();
	await player.context.close();
});

test('a player can select a token but is offered no way to delete it', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Delete Token Player');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(player.page);
	const spawn = spawnCentre(box);
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	await player.page.mouse.click(box.x + spawn.x, box.y + spawn.y);

	// Selecting works for anyone; the delete button is the GM's alone. The
	// selection succeeding is what stops this from passing for the wrong
	// reason — a missed click would show no button either.
	await expect(detailsSection(player.page)).toContainText('Goblin');
	await expect(deleteButton(player.page)).toBeHidden();
	await expect(deleteButton(gm.page)).toBeHidden(); // nothing selected there yet

	await gm.context.close();
	await player.context.close();
});
