import { expect, test, type Browser, type Page } from '@playwright/test';

// Editing a token is the first command whose broadcast depends on what
// the token *was*, not only on what it is now: crossing the hidden line
// tells a Player something different in each direction, and neither
// direction is visible from the DOM. So this drives two browsers and
// reads the canvas.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview,
// selection.
const TOKEN_LAYER = 4;
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

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

// Where a freshly created token lands — the cell at the centre of the
// creator's view, on a scene still at the identity transform. See
// token-selection.spec.ts.
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

// Waits for the dialog to be gone as well as the token to arrive: raw
// mouse coordinates make no actionability checks, so a dialog still
// running its exit animation swallows the click meant for the canvas.
async function createToken(page: Page, name: string) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);
}

function detailsSection(page: Page) {
	return page.getByRole('region', { name: 'Selected token' }).first();
}

async function openEditor(page: Page) {
	await page.getByRole('button', { name: 'Edit token' }).first().click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeVisible();
}

async function save(page: Page) {
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeHidden();
}

test('a GM renames and resizes a token, and the whole room sees it', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Token Edit');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);
	const atOneSquare = await layerInk(player.page, TOKEN_LAYER);

	await gm.page.mouse.click(box.x + spawn.x, box.y + spawn.y);
	await expect(detailsSection(gm.page)).toContainText('Goblin');

	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Hobgoblin');
	await gm.page.getByRole('button', { name: 'Large (2×2 squares)' }).click();
	await save(gm.page);

	// The strip reads from room.tokens, so it re-renders from the
	// broadcast rather than from what was typed.
	await expect(detailsSection(gm.page)).toContainText('Hobgoblin');
	await expect(detailsSection(gm.page)).toContainText('2×2 squares');

	// A 2x2 token covers four times the ground, on the map of someone who
	// only knows through the socket.
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(atOneSquare * 2);

	// And it is really on the server, not just on two canvases.
	await player.page.reload();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(atOneSquare * 2);

	await gm.context.close();
	await player.context.close();
});

// The two halves of the hidden line, which are the only place this
// command needs to know what the token used to be.
test('hiding a token takes it off the players map, and revealing it puts it back', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Token Hide');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Ambusher');
	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);

	await gm.page.mouse.click(box.x + spawn.x, box.y + spawn.y);
	await expect(detailsSection(gm.page)).toContainText('Ambusher');

	await openEditor(gm.page);
	await gm.page.getByRole('button', { name: 'Hidden from players' }).click();
	await save(gm.page);

	// Gone for the player. The GM keeps it — dimmed, but still theirs to
	// see — which is what stops this passing for the wrong reason.
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBe(0);
	expect(await layerInk(gm.page, TOKEN_LAYER)).toBeGreaterThan(0);
	await expect(detailsSection(gm.page)).toContainText('hidden from players');

	// Back the other way. The player was never told the token existed, so
	// what arrives has to be the whole thing rather than a change to
	// something they are holding.
	await openEditor(gm.page);
	await gm.page.getByRole('button', { name: 'Visible', exact: true }).click();
	await save(gm.page);

	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);

	await gm.context.close();
	await player.context.close();
});

test('a player is offered no way to edit a token they can select', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Token Edit Player');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(player.page);
	const spawn = spawnCentre(box);
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);

	await player.page.mouse.click(box.x + spawn.x, box.y + spawn.y);

	// Selecting works for anyone — asserted first, so a missed click can't
	// make the absent button look like a permission check.
	await expect(detailsSection(player.page)).toContainText('Goblin');
	await expect(player.page.getByRole('button', { name: 'Edit token' })).toBeHidden();

	await gm.context.close();
	await player.context.close();
});
