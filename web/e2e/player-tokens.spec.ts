import { expect, test, type Browser, type Page } from '@playwright/test';
import { createRoom, joinAsNewPlayer, openNewSceneDialog } from './fixtures/room';

// A Player putting their own tokens on the map, and taking them off
// again. Two browsers throughout: half of what's being claimed is that
// the *other* side of the table sees it, and a token is Konva, so the
// canvas is the only witness for "there are three of them, on three
// squares".

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

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

// Where a freshly created token lands: the cell at the centre of the
// creator's own view. Read from the page being clicked, never shared
// between two of them.
function spawnCentre(box: { width: number; height: number }) {
	const cell = { x: Math.round(box.width / 2 / GRID), y: Math.round(box.height / 2 / GRID) };
	return { x: cell.x * GRID + GRID / 2, y: cell.y * GRID + GRID / 2 };
}

async function openRoomAsGM(browser: Browser, roomName: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	const slug = await createRoom(page, roomName);

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page, slug };
}

async function joinRoomAsPlayer(browser: Browser, slug: string, name = 'Bob') {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, name);
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

// Waits for the dialog to be gone as well as the ink to arrive — a
// dialog still running its exit animation swallows a click meant for
// the canvas.
async function createTokens(page: Page, name: string, count = 1) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	if (count > 1) await page.getByLabel('How many').fill(String(count));
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);
}

function detailsSection(page: Page) {
	return page.getByRole('region', { name: 'Selected token' }).first();
}

// Clicks until the details panel names the token. Konva fires `click`
// only when both halves land on the same node, and renderTokens rebuilds
// every group whenever room.tokens changes — a rebuild between them
// swallows the click. See token-trackers.spec.ts, which explains it at
// length.
async function selectToken(page: Page, at: { x: number; y: number }, name: string) {
	const box = await canvasBox(page);
	await expect
		.poll(async () => {
			await page.mouse.click(box.x + at.x, box.y + at.y);
			return detailsSection(page).textContent();
		})
		.toContain(name);
}

test('a player creates a token, owns it, and the GM sees it appear', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Player Tokens');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createTokens(player.page, 'Familiar');

	// The GM's map, not the creator's: a token nobody else can see would
	// be a private drawing rather than a token.
	await expect.poll(() => layerInk(gm.page, TOKEN_LAYER)).toBeGreaterThan(0);

	// Theirs without having chosen an owner — the roster is what turns the
	// stored id back into a name, so this is also proof the id that landed
	// is the player's own.
	const spawn = spawnCentre(await canvasBox(player.page));
	await selectToken(player.page, spawn, 'Familiar');
	await expect(detailsSection(player.page)).toContainText("Bob's token");

	await gm.context.close();
	await player.context.close();
});

// The two fields a Player's dialog doesn't have. Both are enforced by
// the server whatever the form sends; this is about not offering them.
test('the new-token dialog offers a player no owner and no visibility', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Player Dialog');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await player.page.getByRole('button', { name: 'New token' }).click();
	await expect(player.page.getByLabel('Name')).toBeVisible();
	await expect(player.page.getByLabel('Owner')).toHaveCount(0);
	await expect(player.page.getByRole('button', { name: 'Hidden from players' })).toHaveCount(0);
	// The count is not one of them: six goblins is the same problem as
	// eight monkeys, so a GM and a Player get the same stepper.
	await expect(player.page.getByLabel('How many')).toBeVisible();

	// The GM's dialog is the one that still has all of it.
	await gm.page.getByRole('button', { name: 'New token' }).click();
	await expect(gm.page.getByLabel('Owner')).toBeVisible();
	await expect(gm.page.getByRole('button', { name: 'Hidden from players' })).toBeVisible();
	await expect(gm.page.getByLabel('How many')).toBeVisible();

	await gm.context.close();
	await player.context.close();
});

// Clearing up after yourself is the other half of conjuring: without it
// the eight monkeys become the GM's cleanup, which is the busywork the
// whole feature removes.
test('a player deletes their own token but is offered no way to delete the GMs', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Player Deletes');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createTokens(player.page, 'Familiar');
	const spawn = spawnCentre(await canvasBox(player.page));
	await selectToken(player.page, spawn, 'Familiar');

	const deleteButton = player.page.getByRole('button', { name: 'Delete token' }).first();
	await expect(deleteButton).toBeVisible();
	await deleteButton.click();

	// Gone from the GM's map too, which is what makes it a deletion rather
	// than a local dismissal.
	await expect.poll(() => layerInk(gm.page, TOKEN_LAYER)).toBe(0);

	// A monster the GM put down is not theirs to remove. The GM's own view
	// is where it spawned, so that is the page whose box locates it.
	await createTokens(gm.page, 'Goblin');
	const gmSpawn = spawnCentre(await canvasBox(gm.page));
	await selectToken(player.page, gmSpawn, 'Goblin');
	await expect(player.page.getByRole('button', { name: 'Delete token' })).toHaveCount(0);

	await gm.context.close();
	await player.context.close();
});

// Eight monkeys in one trip through the dialog, on eight squares.
test('several tokens at once are numbered and land as a block, undone one at a time', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Conjure Animals');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createTokens(player.page, 'Monkey', 3);
	const three = await layerInk(player.page, TOKEN_LAYER);

	// The first of a batch keeps the square that was pointed at, and the
	// numbering starts at 1 rather than 0 — see spawnCells.
	const spawn = spawnCentre(await canvasBox(player.page));
	await selectToken(player.page, spawn, 'Monkey 1');

	// One undo per token, newest first: three presses take all three back,
	// deliberately not one press for the whole batch.
	for (let i = 0; i < 3; i++) await player.page.keyboard.press('Control+z');
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBe(0);

	// And the room agrees — undo goes through the same token.delete
	// everyone else hears about.
	await expect.poll(() => layerInk(gm.page, TOKEN_LAYER)).toBe(0);

	// The yardstick, measured last because a batch spreads around whatever
	// is already standing there and this one has to land on the same empty
	// map the monkeys did. Three tokens *stacked* would draw about as much
	// ink as one, which is what makes this the assertion that says "block,
	// not stack" — the count alone couldn't.
	await createTokens(player.page, 'Alone');
	const one = await layerInk(player.page, TOKEN_LAYER);
	expect(three).toBeGreaterThan(one * 2.5);

	await gm.context.close();
	await player.context.close();
});
