import { expect, test, type Browser, type Page } from '@playwright/test';
import {
	createRoom,
	joinAsNewPlayer,
	mapGestureOrigin,
	openNewSceneDialog,
	selectTool,
	selectToolFamily
} from './fixtures/room';

// Fog beyond revealing: re-hiding a square, wiping a scene back to fully
// covered, and uncovering it all at once. Every assertion here is made
// against a *Player's* canvas rather than the GM's, because a GM's fog is
// deliberately translucent (0.5 by default, so they can still see the map
// under it) while a Player's is opaque — testing the Player side means
// "covered" and "revealed" are a clean full-vs-0 alpha delta rather than a
// fraction of one, regardless of what opacity the GM has chosen.

// Layer order, by index into document.querySelectorAll('canvas'):
// 0 map, 1 grid, 2 fog, 3 drawings, 4 tokens, 5 pings, 6 measurements,
// 7 preview, 8 selection. Inserting a layer renumbers these.
const FOG_LAYER = 2;

// Total alpha across the fog layer. The sum rather than a count of
// non-transparent pixels, because it says how *much* is covered — which
// is what tells a partial reveal apart from a total one, and what lets a
// GM's translucent fog register at all.
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

// Both fog tools used to relabel themselves while active ("Reveal fog"
// became "Painting fog…"), which the shared selectTool helper couldn't
// wait on — the locator it clicked stopped matching the moment the tool
// went live. Since the full-bleed layout they're icons on the fog
// family's contextual strip with stable names, so the shared helper
// handles them like any other variant. It still does the waiting that
// matters: tool handlers are rebound in an effect, so a drag issued in
// the same tick would otherwise run under the previous tool.
const selectRevealTool = (page: Page) => selectTool(page, 'Reveal fog');
const selectHideTool = (page: Page) => selectTool(page, 'Hide fog');

// Drags a rectangle over a few grid squares. Both fog tools commit every
// cell inside its bounding box on release, so the same corner-to-corner
// path reveals and re-hides exactly the same set.
async function dragFogRect(page: Page, origin: { x: number; y: number }) {
	await page.mouse.move(origin.x + 100, origin.y + 100);
	await page.mouse.down();
	await page.mouse.move(origin.x + 300, origin.y + 200, { steps: 8 });
	await page.mouse.up();
}

async function createRoomWithScene(browser: Browser, name: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	const slug = await createRoom(page, name);

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	// A scene starts fully revealed — fog stores what's hidden, and a new
	// scene has none. These tests are about the hide/reset/reveal-all
	// mechanics rather than that default, so they cover the scene
	// themselves first instead of assuming a baseline. Given time to land
	// rather than returned straight after the click: the reset is a
	// command round trip plus a Konva redraw, and a caller reading before
	// either settles would catch a still-transitioning layer instead of
	// the fully covered one.
	await selectToolFamily(page, 'Fog');
	await page.getByRole('button', { name: 'Hide all', exact: true }).click();
	await page.waitForTimeout(300);

	return { context, page, slug };
}

// A separate browser context, so the player gets their own localStorage
// and is treated as a different person rather than the GM again.
async function joinAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, 'Bob');
	await expect(page.getByText('player', { exact: true })).toBeVisible();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

test('hiding fog puts revealed squares back under cover for players', async ({ browser }) => {
	const gm = await createRoomWithScene(browser, 'Fog Hide');
	const player = await joinAsPlayer(browser, gm.slug);

	const covered = await fogAlpha(player.page);
	expect(covered).toBeGreaterThan(0); // createRoomWithScene reset it first

	const origin = await mapGestureOrigin(gm.page);
	await selectRevealTool(gm.page);
	await dragFogRect(gm.page, origin);

	// The reveal has to reach the player, not just the GM who painted it.
	await expect.poll(() => fogAlpha(player.page)).toBeLessThan(covered);

	await selectHideTool(gm.page);
	await dragFogRect(gm.page, origin);

	// Back to exactly the starting cover, pixel for pixel: the same
	// rectangle hidden again sets exactly the bits the reveal cleared.
	// Anything short of equality would mean a square stayed uncovered.
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

	const origin = await mapGestureOrigin(gm.page);
	await selectRevealTool(gm.page);
	await dragFogRect(gm.page, origin);
	await expect.poll(() => fogAlpha(player.page)).toBeLessThan(covered);

	await gm.page.getByRole('button', { name: 'Hide all', exact: true }).click();
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

	// Reveal all lives on the fog family's strip, so the family has to be
	// open before the button exists — unlike the tests above, this one
	// never picks a fog *tool*, so nothing else would have opened it.
	await selectToolFamily(gm.page, 'Fog');
	await gm.page.getByRole('button', { name: 'Reveal all', exact: true }).click();

	// Reveal-all drops every chunk the scene had, so the fog layer draws
	// nothing at all — not merely less of it.
	await expect.poll(() => fogAlpha(player.page)).toBe(0);

	await gm.context.close();
	await player.context.close();
});
