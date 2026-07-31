import { expect, test, type Page, type Browser } from '@playwright/test';

// Fog beyond revealing: re-hiding a square, wiping a scene back to fully
// covered, and uncovering it all at once. Every assertion here is made
// against a *Player's* canvas rather than the GM's, because the GM's view
// is deliberately see-through — their cover is drawn at 0.35 opacity and
// revealed cells are punched out at 0.35 too, so a GM's fog barely moves
// the numbers. A Player gets an opaque cover and a full punch-out, which
// is what makes "covered" and "revealed" tell each other apart at all.

// Layer order, by index into document.querySelectorAll('canvas'):
// 0 map, 1 grid, 2 fog, 3 drawings, 4 tokens, 5 pings, 6 measurements,
// 7 preview. Inserting a layer renumbers these.
const FOG_LAYER = 2;

// Total alpha across the fog layer. A count of non-transparent pixels
// can't see a GM's fog at all (see the note above), and even for a
// Player the sum is the more useful number: it says how *much* is
// covered, so a partial reveal is distinguishable from a total one.
async function fogAlpha(page: Page): Promise<number> {
	return page.evaluate((index) => {
		const canvas = document.querySelectorAll('canvas')[index] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;
		let total = 0;
		for (let i = 3; i < data.length; i += 4) total += data[i];
		return total;
	}, FOG_LAYER);
}

async function canvasOrigin(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return { x: box.x, y: box.y };
}

// Both fog tools relabel themselves while active rather than only
// restyling, so the shared selectTool helper in the other specs can't
// wait on them — the locator it clicked stops matching the moment the
// tool goes live. Waiting on the new label is what proves the switch
// landed, and the wait matters: tool handlers are rebound in an effect,
// so a drag issued in the same tick still runs under the previous tool.
async function selectFogTool(page: Page, idle: string, active: string) {
	await page.getByRole('button', { name: idle, exact: true }).click();
	await expect(page.getByRole('button', { name: active, exact: true })).toBeVisible();
}

const selectRevealTool = (page: Page) => selectFogTool(page, 'Reveal fog', 'Painting fog…');
const selectHideTool = (page: Page) => selectFogTool(page, 'Hide fog', 'Hiding fog…');

// A short drag across a few grid squares. Both fog tools paint per cell
// crossed, so the same path reveals and re-hides exactly the same set.
async function paintAcrossCells(page: Page, origin: { x: number; y: number }) {
	await page.mouse.move(origin.x + 100, origin.y + 100);
	await page.mouse.down();
	await page.mouse.move(origin.x + 300, origin.y + 200, { steps: 8 });
	await page.mouse.up();
}

async function createRoomWithScene(browser: Browser, name: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto('/');
	await page.getByLabel('Room name').fill(name);
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

// A separate browser context, so the player gets their own localStorage
// and is treated as a different person rather than the GM again.
async function joinAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join' }).click();
	await expect(page.getByText('player', { exact: true })).toBeVisible();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

test('hiding fog puts revealed squares back under cover for players', async ({ browser }) => {
	const gm = await createRoomWithScene(browser, 'Fog Hide');
	const player = await joinAsPlayer(browser, gm.slug);

	const covered = await fogAlpha(player.page);
	expect(covered).toBeGreaterThan(0); // a scene starts fully hidden

	const origin = await canvasOrigin(gm.page);
	await selectRevealTool(gm.page);
	await paintAcrossCells(gm.page, origin);

	// The reveal has to reach the player, not just the GM who painted it.
	await expect.poll(() => fogAlpha(player.page)).toBeLessThan(covered);

	await selectHideTool(gm.page);
	await paintAcrossCells(gm.page, origin);

	// Back to exactly the starting cover: with no cells revealed the fog
	// layer is the bare cover rect again, pixel for pixel. Anything short
	// of equality would mean a square stayed punched out.
	await expect.poll(() => fogAlpha(player.page)).toBe(covered);

	await gm.context.close();
	await player.context.close();
});

test('resetting fog re-hides the whole scene, and it stays reset after a reload', async ({
	browser
}) => {
	const gm = await createRoomWithScene(browser, 'Fog Reset');
	const player = await joinAsPlayer(browser, gm.slug);

	const covered = await fogAlpha(player.page);

	const origin = await canvasOrigin(gm.page);
	await selectRevealTool(gm.page);
	await paintAcrossCells(gm.page, origin);
	await expect.poll(() => fogAlpha(player.page)).toBeLessThan(covered);

	await gm.page.getByRole('button', { name: 'Reset fog', exact: true }).click();
	await expect.poll(() => fogAlpha(player.page)).toBe(covered);

	// A reload re-reads the scene from the server, so this is what proves
	// the reset was persisted rather than only applied in each client.
	await player.page.reload();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => fogAlpha(player.page)).toBe(covered);

	await gm.context.close();
	await player.context.close();
});

test('revealing all uncovers the entire scene for players', async ({ browser }) => {
	const gm = await createRoomWithScene(browser, 'Fog Reveal All');
	const player = await joinAsPlayer(browser, gm.slug);

	expect(await fogAlpha(player.page)).toBeGreaterThan(0);

	await gm.page.getByRole('button', { name: 'Reveal all', exact: true }).click();

	// Every cell inside the scene's bounds is revealed, and a player's
	// punch-out is total, so nothing of the cover survives anywhere on
	// the layer — not merely less of it.
	await expect.poll(() => fogAlpha(player.page)).toBe(0);

	await gm.context.close();
	await player.context.close();
});
