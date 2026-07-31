import { expect, test, type Page, type Browser } from '@playwright/test';

// Reaching a scene other than the one you just made. Before this there
// was no switcher at all, which is why scene.create used to activate
// every new scene — so the thing most worth proving here is that the
// second scene *doesn't* take over, and that a GM can still get to it.

// A real 8x8 PNG of a flat colour nothing else in the suite uploads.
// Content-addressed storage plus a scratch database that is never reset
// between runs means reusing another spec's pixels would resolve to that
// spec's asset row under its original filename.
//
// Genuinely encoded, not hand-edited from another spec's base64: an
// upload has to survive imageproc's re-encode, which sniffs the content
// and answers 400 for anything that isn't really an image. A corrupted
// literal fails as an empty asset library with no server-side log,
// because the request never gets past the handler's decode.
const SWAMP_PNG = Buffer.from(
	'iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAGUlEQVR4nGLRz7ZiwAaYsIoOWglAAAAA//+tTQDnQ54igQAAAABJRU5ErkJggg==',
	'base64'
);

// Layer order, by index into document.querySelectorAll('canvas'):
// 0 map, 1 grid, 2 fog, 3 drawings, 4 tokens, 5 pings, 6 measurements,
// 7 preview. Inserting a layer renumbers these.
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

	await page.goto('/');
	await page.getByLabel('Room name').fill(roomName);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();
	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);

	return { context, page, slug: new URL(page.url()).pathname.split('/').pop()! };
}

async function joinAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join' }).click();
	await expect(page.getByText('player', { exact: true })).toBeVisible();

	return { context, page };
}

async function createScene(page: Page, name: string) {
	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create scene' }).click();
	// The dialog closing is the signal the command went out; asserting on
	// the canvas instead would be wrong for a scene that doesn't activate.
	await expect(page.getByLabel('Name')).toBeHidden();
}

const openScenes = (page: Page) =>
	page.getByRole('button', { name: 'Scenes', exact: true }).click();

test('a second scene waits to be switched to, and switching moves the whole room', async ({
	browser
}) => {
	const gm = await createRoomAsGM(browser, 'Scene Switch');
	const player = await joinAsPlayer(browser, gm.slug);

	await createScene(gm.page, 'Tavern');
	// The first scene still activates — there was nothing to switch away
	// from, and a room with no map after making one reads as a failure.
	await expect(gm.page.locator('canvas').first()).toBeVisible();
	await expect(player.page.locator('canvas').first()).toBeVisible();

	await createScene(gm.page, 'Dungeon');

	await openScenes(gm.page);
	await expect(gm.page.getByRole('button', { name: 'Switch to Dungeon' })).toBeVisible();
	// Tavern is still the active one, so it offers no switch button at all.
	await expect(gm.page.getByRole('button', { name: 'Switch to Tavern' })).toBeHidden();

	await gm.page.getByRole('button', { name: 'Switch to Dungeon' }).click();

	// Both sides move, and the row that was switchable becomes the active one.
	await expect(gm.page.getByRole('button', { name: 'Switch to Tavern' })).toBeVisible();
	await expect(gm.page.getByRole('button', { name: 'Switch to Dungeon' })).toBeHidden();
	await expect(player.page.locator('canvas').first()).toBeVisible();

	await gm.context.close();
	await player.context.close();
});

test('the active scene cannot be deleted, but another one can', async ({ browser }) => {
	const gm = await createRoomAsGM(browser, 'Scene Delete');

	await createScene(gm.page, 'Tavern');
	await createScene(gm.page, 'Dungeon');

	await openScenes(gm.page);

	// Tavern is active: deleting it would leave the room pointing at a
	// scene that no longer exists, so the button is refused up front.
	await expect(gm.page.getByRole('button', { name: 'Delete Tavern' })).toBeDisabled();

	// Deleting takes two clicks — a scene takes its tokens, fog and
	// drawings with it, which is too much to lose to a stray click.
	await gm.page.getByRole('button', { name: 'Delete Dungeon' }).click();
	await expect(gm.page.getByRole('button', { name: 'Confirm deleting Dungeon' })).toBeVisible();
	await gm.page.getByRole('button', { name: 'Confirm deleting Dungeon' }).click();

	await expect(gm.page.getByRole('button', { name: 'Delete Dungeon' })).toBeHidden();
	await expect(gm.page.getByRole('button', { name: 'Delete Tavern' })).toBeVisible();

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

	await openScenes(gm.page);
	await gm.page.getByRole('button', { name: 'Replace the map for Tavern' }).click();
	await gm.page
		.getByLabel('Upload an image')
		.setInputFiles({ name: 'swamp.png', mimeType: 'image/png', buffer: SWAMP_PNG });
	// The upload re-encodes to WebP on the way past, so the library entry
	// appearing under the new name is what proves the round trip landed.
	await expect(gm.page.getByRole('button', { name: 'swamp.webp' })).toBeVisible();
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
