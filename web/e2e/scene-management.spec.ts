import { expect, test, type Page, type Browser } from '@playwright/test';
import { fixture } from './fixtures/images';
import {
	createRoom,
	joinAsNewPlayer,
	openAssetsPage,
	openNewSceneDialog,
	openScenesDialog
} from './fixtures/room';

// Reaching a scene other than the one you just made. Before this there
// was no switcher at all, which is why scene.create used to activate
// every new scene — so the thing most worth proving here is that the
// second scene *doesn't* take the room over, and that a GM can still get
// to it.
//
// Since per-client viewing, "get to it" and "take the room over" are two
// separate actions, and most of what these specs are watching for is one
// of them doing the other's job.

// Layer order, by index into document.querySelectorAll('canvas'):
// 0 map, 1 grid, 2 fog, 3 drawings, 4 tokens, 5 pings, 6 measurements,
// 7 preview, 8 selection. Inserting a layer renumbers these.
const MAP_LAYER = 0;
const TOKEN_LAYER = 4;

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

async function createRoomAsGM(browser: Browser, roomName: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	const slug = await createRoom(page, roomName);

	return { context, page, slug };
}

async function joinAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, 'Bob');
	await expect(page.getByText('player', { exact: true })).toBeVisible();

	return { context, page };
}

async function createScene(page: Page, name: string) {
	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create scene' }).click();
	// The dialog closing is the signal the command went out; asserting on
	// the canvas instead would be wrong for a scene that doesn't activate.
	await expect(page.getByLabel('Name')).toBeHidden();
}

// Scenes moved into the room menu with the full-bleed layout, so
// reaching it is two clicks rather than one.
const openScenes = (page: Page) => openScenesDialog(page);

test('a new scene takes the GM to it and leaves the table where it was', async ({ browser }) => {
	const gm = await createRoomAsGM(browser, 'Scene Prep');
	const player = await joinAsPlayer(browser, gm.slug);

	await createScene(gm.page, 'Tavern');
	// The first scene still moves the room — there was nothing to switch
	// away from, and a room with no map after making one reads as a
	// failure.
	await expect(gm.page.locator('canvas').first()).toBeVisible();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	await expect(gm.page.getByText('The table is on')).toBeHidden();

	await createScene(gm.page, 'Dungeon');

	// The GM is standing on the new scene, and says so — the rail is the
	// only thing on screen that can, since one empty map looks like
	// another.
	await expect(gm.page.getByText('The table is on')).toBeVisible();
	// Scoped to the rail: the Scenes dialog names every scene too.
	await expect(gm.page.getByLabel('Session info').getByText('Tavern')).toBeVisible();

	await openScenes(gm.page);
	await expect(gm.page.getByRole('button', { name: 'View Tavern' })).toBeVisible();
	// Nothing offers to take you where you already are.
	await expect(gm.page.getByRole('button', { name: 'View Dungeon' })).toBeHidden();
	await expect(gm.page.getByRole('button', { name: 'Move everyone to Dungeon' })).toBeVisible();

	// The player heard a scene was made and nothing else.
	await expect(player.page.getByText('The table is on')).toBeHidden();

	await gm.page.getByRole('button', { name: 'Move everyone to Dungeon' }).click();

	// Now the room is there too, so neither side is away from the table
	// and the reveal button has nowhere left to send anyone.
	await expect(gm.page.getByRole('button', { name: 'Move everyone to Dungeon' })).toBeHidden();
	await expect(gm.page.getByRole('button', { name: 'Move everyone to Tavern' })).toBeVisible();
	await expect(gm.page.getByText('The table is on')).toBeHidden();
	await expect(player.page.locator('canvas').first()).toBeVisible();

	await gm.context.close();
	await player.context.close();
});

// The half a Player has: the list, and a look at any scene on it. What
// they must not have is any way to move anyone else, which is the same
// line the server draws.
test('a player can look at another scene without moving the table', async ({ browser }) => {
	const gm = await createRoomAsGM(browser, 'Scene Look');
	const player = await joinAsPlayer(browser, gm.slug);

	await createScene(gm.page, 'Tavern');
	await createScene(gm.page, 'Dungeon');
	// Back with everyone else, so the GM is a witness to not moving.
	await openScenes(gm.page);
	await gm.page.getByRole('button', { name: 'View Tavern' }).click();
	await gm.page.keyboard.press('Escape');
	await expect(gm.page.getByText('The table is on')).toBeHidden();

	await openScenes(player.page);
	await expect(player.page.getByRole('button', { name: 'New scene' })).toBeHidden();
	await expect(player.page.getByRole('button', { name: 'Delete Dungeon' })).toBeHidden();
	await expect(player.page.getByRole('button', { name: 'Move everyone to Dungeon' })).toBeHidden();

	await player.page.getByRole('button', { name: 'View Dungeon' }).click();

	await expect(player.page.getByText('The table is on')).toBeVisible();
	// The GM never moved, which is the whole point.
	await expect(gm.page.getByText('The table is on')).toBeHidden();

	// And there's a way back that doesn't need the menu.
	await player.page.keyboard.press('Escape');
	await player.page.getByRole('button', { name: 'Go there' }).click();
	await expect(player.page.getByText('The table is on')).toBeHidden();

	await gm.context.close();
	await player.context.close();
});

test("the table's scene cannot be deleted, but another one can", async ({ browser }) => {
	const gm = await createRoomAsGM(browser, 'Scene Delete');

	await createScene(gm.page, 'Tavern');
	await createScene(gm.page, 'Dungeon');

	await openScenes(gm.page);

	// Tavern is the table's: deleting it would leave the room pointing at
	// a scene that no longer exists, so the button is refused up front.
	await expect(gm.page.getByRole('button', { name: 'Delete Tavern' })).toBeDisabled();

	// Deleting takes two clicks — a scene takes its tokens, fog and
	// drawings with it, which is too much to lose to a stray click.
	await gm.page.getByRole('button', { name: 'Delete Dungeon' }).click();
	await expect(gm.page.getByRole('button', { name: 'Confirm deleting Dungeon' })).toBeVisible();
	await gm.page.getByRole('button', { name: 'Confirm deleting Dungeon' }).click();

	await expect(gm.page.getByRole('button', { name: 'Delete Dungeon' })).toBeHidden();
	await expect(gm.page.getByRole('button', { name: 'Delete Tavern' })).toBeVisible();

	// The GM was standing on Dungeon when it went — making a scene takes
	// you to it — so deleting it has to put them back with the table
	// rather than leave a map on screen the server can't answer for.
	await expect(gm.page.getByText('The table is on')).toBeHidden();
	await expect(gm.page.getByRole('button', { name: 'View Tavern' })).toBeHidden();

	// It's really gone, not just gone from this list.
	await gm.page.reload();
	await openScenes(gm.page);
	await expect(gm.page.getByRole('button', { name: 'Delete Tavern' })).toBeVisible();
	await expect(gm.page.getByRole('button', { name: 'Delete Dungeon' })).toBeHidden();

	await gm.context.close();
});

test('replacing a map keeps the tokens already standing on the scene', async ({ browser }) => {
	const gm = await createRoomAsGM(browser, 'Scene Remap');
	const player = await joinAsPlayer(browser, gm.slug);

	await createScene(gm.page, 'Tavern');
	await expect(gm.page.locator('canvas').first()).toBeVisible();

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill('Goblin');
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);
	const tokenInk = await layerInk(player.page, TOKEN_LAYER);
	const mapInk = await layerInk(player.page, MAP_LAYER);

	// Art is added on the assets page and only picked here — the replace
	// dialog has no upload of its own any more.
	await openAssetsPage(gm.page);
	// The tab is chosen before the file is, and it decides what the upload
	// is filed as — which has to be a map, since that's the half of the
	// library the replace-map picker opens on.
	await gm.page.getByRole('tab', { name: /^Maps/ }).click();
	await gm.page.getByLabel('Choose images to add').setInputFiles(fixture('swamp.png'));
	await gm.page.getByRole('button', { name: 'Add to library' }).click();
	await expect(gm.page.getByText('swamp', { exact: true })).toBeVisible();
	await gm.page.getByRole('link', { name: 'Back to the table' }).click();

	await openScenes(gm.page);
	await gm.page.getByRole('button', { name: 'Replace the map for Tavern' }).click();
	await gm.page.getByRole('button', { name: 'swamp' }).click();
	await gm.page.getByRole('button', { name: 'Replace map', exact: true }).click();

	// The map changed for everyone: the scene was a bare 1400x1000 grey
	// rect and is now an 8x8 image, so the map layer's ink collapses.
	// Asserted on the player's canvas, which only knows through the
	// broadcast.
	await expect.poll(() => layerInk(player.page, MAP_LAYER)).toBeLessThan(mapInk);

	// ...and the token standing on it is untouched, which is the whole
	// reason to replace a map rather than build a new scene.
	expect(await layerInk(player.page, TOKEN_LAYER)).toBe(tokenInk);

	await gm.context.close();
	await player.context.close();
});
