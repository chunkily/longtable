import { expect, test, type Page } from '@playwright/test';
import { mapGestureOrigin, openNewSceneDialog, selectTool } from './fixtures/room';

// Everything a Player does on a LAN address, faked on localhost.
//
// `crypto.randomUUID` is defined only in a secure context, and Longtable's
// deployment story is a GM running the binary while everyone else opens
// `http://192.168.x.x:8080` — which is not one. Drawing and pings both
// mint ids client-side, so both threw for every Player, and neither the
// e2e suite nor a developer's browser could see it: Playwright drives
// localhost, which is always a secure context.
//
// So the insecure context is simulated rather than reached. An init
// script takes `randomUUID` away before any page script runs, which is
// exactly the shape a LAN client sees — `getRandomValues` stays, because
// it isn't gated on a secure context and is what the fallback uses.
//
// The vitest beside `random-id.ts` already checks the fallback's spelling.
// What only a browser can check is the half that matters more: that the
// server *accepts* the id it produces. `isCanonicalUUID` rejects anything
// but the lowercase hyphenated form, so a fallback of the wrong shape
// would draw a stroke that vanished on reload — passing on localhost,
// where it never runs, and failing on the LAN, where it always does.

const DRAWING_LAYER = 3;
const PING_LAYER = 5;

async function stripRandomUUID(page: Page) {
	await page.addInitScript(() => {
		Object.defineProperty(crypto, 'randomUUID', { value: undefined, configurable: true });
	});
}

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

async function createRoomWithScene(page: Page, name: string) {
	await page.goto('/');
	await page.getByLabel('Room name').fill(name);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();
}

test('a stroke drawn without crypto.randomUUID is accepted and kept by the server', async ({
	page
}) => {
	await stripRandomUUID(page);
	await createRoomWithScene(page, 'Insecure Draw');

	// A positive control on the simulation itself. Without this the test
	// would still pass if addInitScript quietly stopped working, and would
	// then be checking nothing at all.
	expect(await page.evaluate(() => typeof crypto.randomUUID)).toBe('undefined');

	await selectTool(page, 'Line');
	const origin = await mapGestureOrigin(page);
	await page.mouse.move(origin.x + 100, origin.y + 150);
	await page.mouse.down();
	await page.mouse.move(origin.x + 400, origin.y + 150, { steps: 8 });
	await page.mouse.up();

	await expect.poll(() => layerInk(page, DRAWING_LAYER)).toBeGreaterThan(0);

	// The stroke renders optimistically, so ink on the canvas only proves
	// the client didn't throw. Reloading is what proves the server took the
	// id: a spelling `isCanonicalUUID` refused would come back empty here.
	await page.reload();
	await expect(page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => layerInk(page, DRAWING_LAYER)).toBeGreaterThan(0);
});

test('a ping is folded in without crypto.randomUUID', async ({ page }) => {
	await stripRandomUUID(page);
	await createRoomWithScene(page, 'Insecure Ping');

	await selectTool(page, 'Ping');
	const origin = await mapGestureOrigin(page);
	await page.mouse.click(origin.x + 250, origin.y + 150);

	// This one threw on *receipt* rather than on send, so it broke for the
	// GM too the moment any Player was in the room — the id is minted while
	// folding the event in, not while sending it.
	await expect.poll(() => layerInk(page, PING_LAYER)).toBeGreaterThan(0);
});
