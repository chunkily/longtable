import { expect, test, type Browser, type Page } from '@playwright/test';
import { joinAsNewPlayer } from './fixtures/room';

// Who is at the table right now exists only in the server's memory —
// there is no row to check and no way to see it from one browser. So
// this drives two, and closing one context is the only honest way to
// test a disconnect.

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
	return { context, page, slug };
}

async function joinRoomAsPlayer(browser: Browser, slug: string, name: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, name);
	await expect(page.getByRole('region', { name: "Who's connected" }).first()).toBeVisible();

	return { context, page };
}

function whoIsHere(page: Page) {
	return page.getByRole('region', { name: "Who's connected" }).first();
}

test('the room shows who is connected, live, and drops them when they go', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Presence');

	// Alone to start with, and the GM can see themselves.
	await expect(whoIsHere(gm.page)).toContainText('Alice (GM)');
	await expect(whoIsHere(gm.page)).not.toContainText('Bob');

	const player = await joinRoomAsPlayer(browser, gm.slug, 'Bob');

	// Each sees both, which needs the arrival broadcast in one direction
	// and the roster in state.sync in the other.
	await expect(whoIsHere(gm.page)).toContainText('Bob');
	await expect(whoIsHere(player.page)).toContainText('Alice (GM)');
	await expect(whoIsHere(player.page)).toContainText('Bob');

	// Closing the context drops the socket without a goodbye, which is
	// what a closed laptop looks like.
	await player.context.close();

	await expect(whoIsHere(gm.page)).not.toContainText('Bob');
	await expect(whoIsHere(gm.page)).toContainText('Alice (GM)');

	await gm.context.close();
});

// The roster is everyone who has ever joined; this list is only who is
// here now. Someone who played and left must not linger in it.
test('someone who joined earlier but is offline does not appear', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Presence History');

	const earlier = await joinRoomAsPlayer(browser, gm.slug, 'Carol');
	await expect(whoIsHere(gm.page)).toContainText('Carol');
	await earlier.context.close();
	await expect(whoIsHere(gm.page)).not.toContainText('Carol');

	// A fresh client's state.sync has to agree with the live view: Carol
	// is on the roster the server sent, and still must not be listed.
	const player = await joinRoomAsPlayer(browser, gm.slug, 'Bob');
	await expect(whoIsHere(player.page)).toContainText('Alice (GM)');
	await expect(whoIsHere(player.page)).toContainText('Bob');
	await expect(whoIsHere(player.page)).not.toContainText('Carol');

	await gm.context.close();
	await player.context.close();
});

// A second tab is a second connection but the same person, and the
// server only announces the first arrival and the last departure.
test('a second tab does not add or remove a second person', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Presence Tabs');
	const player = await joinRoomAsPlayer(browser, gm.slug, 'Bob');
	await expect(whoIsHere(gm.page)).toContainText('Bob');

	// Same context, so the same session token out of localStorage —
	// which is what makes this the same participant rather than a new one.
	const secondTab = await player.context.newPage();
	await secondTab.goto(`/r/${gm.slug}`);
	await expect(whoIsHere(secondTab).first()).toContainText('Bob');

	// Exactly one badge for Bob, on every page.
	await expect(gm.page.getByText('Bob', { exact: true })).toHaveCount(1);

	await secondTab.close();
	// Still here: the first tab is still open.
	await gm.page.waitForTimeout(500);
	await expect(whoIsHere(gm.page)).toContainText('Bob');

	await player.context.close();
	await expect(whoIsHere(gm.page)).not.toContainText('Bob');

	await gm.context.close();
});
